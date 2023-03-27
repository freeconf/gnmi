package server

import (
	"context"
	"errors"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

var Version = "0.0.0"

type Driver struct {
	Device *device.Local
	pb_gnmi.UnimplementedGNMIServer
}

func (d *Driver) Capabilities(ctx context.Context, req *pb_gnmi.CapabilityRequest) (*pb_gnmi.CapabilityResponse, error) {
	resp := &pb_gnmi.CapabilityResponse{
		SupportedModels: nil,
		SupportedEncodings: []pb_gnmi.Encoding{
			pb_gnmi.Encoding_JSON,
			pb_gnmi.Encoding_JSON_IETF,
		},
		GNMIVersion: Version,
	}
	for moduleName, module := range d.Device.Modules() {
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

func (d *Driver) Get(ctx context.Context, req *pb_gnmi.GetRequest) (*pb_gnmi.GetResponse, error) {
	var vals []*pb_gnmi.TypedValue
	for _, p := range req.Path {
		// target is device id
		module := p.Origin
		if module == "" {
			return nil, errNoModule
		}
		b, err := d.Device.Browser(module)
		if err != nil {
			return nil, err
		}
		val, err := find(b, p.Elem)
		if err != nil {
			return nil, err
		}
		vals = append(vals, val)
	}

	now := time.Now().Unix()
	resp := pb_gnmi.GetResponse{
		Notification: []*pb_gnmi.Notification{
			{
				Timestamp: now,
			},
		},
	}

	resp.Notification[0].Update = make([]*pb_gnmi.Update, len(vals))
	for i, val := range vals {
		resp.Notification[0].Update[i] = &pb_gnmi.Update{
			Path: req.Path[i],
			Val:  val,
		}
	}
	return &resp, nil
}

func find(b *node.Browser, path []*pb_gnmi.PathElem) (*pb_gnmi.TypedValue, error) {
	s := b.Root()
	for _, p := range path {
		s = s.Find(p.Name)
		if s.IsNil() || s.LastErr != nil {
			return nil, s.LastErr
		}
	}
	msg, err := nodeutil.WriteJSON(s)
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

func (d *Driver) Set(ctx context.Context, req *pb_gnmi.SetRequest) (*pb_gnmi.SetResponse, error) {
	return nil, nil
}

func (d *Driver) Subscribe(pb_gnmi.GNMI_SubscribeServer) error {
	return nil
}
