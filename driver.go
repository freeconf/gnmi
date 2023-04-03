package gnmi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

/*
driver bridges between gnmi and FreeCONF. mapping GNMI commands to node operations
*/
type driver struct {
	device *device.Local
	subMgr subscriptionManager
	pb_gnmi.UnimplementedGNMIServer
}

func (d *driver) Capabilities(ctx context.Context, req *pb_gnmi.CapabilityRequest) (*pb_gnmi.CapabilityResponse, error) {
	resp := &pb_gnmi.CapabilityResponse{
		SupportedModels: nil,
		SupportedEncodings: []pb_gnmi.Encoding{
			pb_gnmi.Encoding_JSON,
			pb_gnmi.Encoding_JSON_IETF,
		},
		GNMIVersion: Version,
	}
	for moduleName, module := range d.device.Modules() {
		md := &pb_gnmi.ModelData{
			Name:         moduleName,
			Organization: module.Organization(),
			Version:      module.Revision().Ident(),
		}
		resp.SupportedModels = append(resp.SupportedModels, md)
	}

	return resp, nil
}

var errNoModule = errors.New("no module specified")

func (d *driver) Get(ctx context.Context, req *pb_gnmi.GetRequest) (*pb_gnmi.GetResponse, error) {
	now := time.Now().UnixNano()
	resp := &pb_gnmi.Notification{
		Timestamp: now,
	}

	prefix, err := findPrefix(d.device, req.UseModels, req.Prefix)
	if err != nil {
		return nil, err
	}

	for _, p := range req.Path {
		sel, err := find(d.device, prefix, p)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("get request %s", sel.Path)
		val, err := get(sel)
		if err != nil {
			return nil, err
		}
		resp.Update = append(resp.Update, &pb_gnmi.Update{
			Path: p,
			Val:  val,
		})
	}

	return &pb_gnmi.GetResponse{
		Notification: []*pb_gnmi.Notification{resp},
	}, nil
}

func findPrefix(device *device.Local, prefix *pb_gnmi.Path) (*node.Selection, error) {
	// if len(models) == 0 {
	// 	return nil, fmt.Errorf("must specify exactly 1 model")
	// }
	if prefix == nil || len(prefix.Elem) == 0 {
		return nil, nil
	}

	s, err := find(device, nil, prefix)
	if err != nil || s.IsNil() {
		return nil, err
	}
	return nil, nil
}

func find(device *device.Local, prefix *node.Selection, path *pb_gnmi.Path) (node.Selection, error) {
	var empty node.Selection
	next := prefix

	for _, p := range path.Elem {
		if next == nil {
			module := p.Name
			b, err := device.Browser(module)
			if err != nil || b == nil {
				return empty, err
			}
			root := b.Root()
			next = &root
		} else {
			s := (*next).Find(p.Name)
			if s.IsNil() || s.LastErr != nil {
				return empty, s.LastErr
			}
			next = &s
		}
	}
	if next == nil {
		return empty, nil
	}
	return (*next), nil
}

func (d *driver) Set(ctx context.Context, req *pb_gnmi.SetRequest) (*pb_gnmi.SetResponse, error) {
	var updates []*pb_gnmi.UpdateResult

	prefix, err := findPrefix(d.device, req.Prefix)
	if err != nil {
		return nil, err
	}

	// order according to gNMI spec should be delete, replace then update
	for _, del := range req.Delete {
		sel, err := find(d.device, prefix, del)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("del request %s", sel.Path)
		err = sel.Delete()
		if err != nil {
			return nil, err
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Path: del,
		})
	}
	for _, u := range req.Replace {
		sel, err := find(d.device, prefix, u.Path)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("replace request %s", sel.Path)
		err = set(sel, modeReplace, u.Val)
		if err != nil {
			return nil, err
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Path: u.Path,
		})
	}
	for _, u := range req.Update {
		sel, err := find(d.device, prefix, u.Path)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("update request %s", sel.Path)
		err = set(sel, modePatch, u.Val)
		if err != nil {
			return nil, err
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Path: u.Path,
		})
	}

	return &pb_gnmi.SetResponse{
		Response: updates,
	}, nil
}

func get(sel node.Selection) (*pb_gnmi.TypedValue, error) {
	msg, err := nodeutil.WriteJSON(sel)
	if err != nil {
		return nil, err
	}
	v := &pb_gnmi.TypedValue{
		Value: &pb_gnmi.TypedValue_JsonVal{
			JsonVal: []byte(msg),
		},
	}
	return v, nil
}

const (
	modePatch = iota
	modeReplace
)

func set(sel node.Selection, mode int, v *pb_gnmi.TypedValue) error {
	if v == nil {
		return fmt.Errorf("empty value for %s", sel.Path)
	}
	var n node.Node
	switch x := v.Value.(type) {
	case *pb_gnmi.TypedValue_JsonIetfVal:
		n = nodeutil.ReadJSON(string(x.JsonIetfVal))
	case *pb_gnmi.TypedValue_JsonVal:
		n = nodeutil.ReadJSON(string(x.JsonVal))
	}
	switch mode {
	case modePatch:
		if err := sel.UpsertFrom(n).LastErr; err != nil {
			return err
		}
	case modeReplace:
		if err := sel.ReplaceFrom(n); err != nil {
			return err
		}
	}
	return nil
}

// func find(dev device.Device, prefix string, p *pb_gnmi.Path) (node.Selection, error) {
// 	var empty node.Selection
// 	// target is device id
// 	module := p.Origin
// 	if module == "" {
// 		return empty, errNoModule
// 	}
// 	b, err := dev.Browser(module)
// 	if err != nil {
// 		return empty, err
// 	}

// 	s := b.Root()
// 	for _, elem := range p.Elem {
// 		s = s.Find(elem.Name)
// 		if s.IsNil() || s.LastErr != nil {
// 			return empty, s.LastErr
// 		}
// 	}
// 	return s, nil
// }

func (d *driver) Subscribe(server pb_gnmi.GNMI_SubscribeServer) error {
	for {
		req, err := server.Recv()
		if err != nil && req != nil {
			return err
		}
		if req != nil {
			if err = d.handleSubscribeList(server.Context(), req, server.Send); err != nil {
				return err
			}
		}
	}
}

// according to gNMI spec, this is for config or metrics only, not YANG notifications!
func (d *driver) handleSubscribeList(ctx context.Context, req *pb_gnmi.SubscribeRequest, sink subscriptionSink) error {
	list := req.GetSubscribe()

	prefix, err := findPrefix(d.device, list.Prefix)
	if err != nil {
		return err
	}

	for _, subReq := range list.Subscription {
		fc.Debug.Printf("new sub mode = %d", list.Mode)

		sub := newSubscription(d.device, prefix, subReq, sink)

		// execute once sychronously avoids kicking off threads and runs thru
		// sub to validate paths
		if err := sub.execute(); err != nil {
			return err
		}
		if list.Mode != pb_gnmi.SubscriptionList_ONCE {
			if err := d.subMgr.add(ctx, sub); err != nil {
				return err
			}
		}
	}
	return nil
}
