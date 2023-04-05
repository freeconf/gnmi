package gnmi

import (
	"context"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

func get(d device.Device, ctx context.Context, req *pb_gnmi.GetRequest) (*pb_gnmi.GetResponse, error) {
	now := time.Now().UnixNano()
	resp := &pb_gnmi.Notification{
		Timestamp: now,
	}

	prefix, err := selectPath(d, req.UseModels, req.Prefix)
	if err != nil {
		return nil, err
	}

	for _, p := range req.Path {
		sel, err := advanceSelection(d, prefix, p)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("get request %s", sel.Path)
		val, err := getVal(sel)
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

func getVal(sel node.Selection) (*pb_gnmi.TypedValue, error) {
	var v *pb_gnmi.TypedValue
	if meta.IsLeaf(sel.Path.Meta) {
		val, err := sel.Get()
		if err != nil {
			return nil, err
		}
		v = &pb_gnmi.TypedValue{
			Value: &pb_gnmi.TypedValue_JsonVal{
				JsonVal: []byte(val.String()),
			},
		}
	} else {
		msg, err := nodeutil.WriteJSON(sel)
		if err != nil {
			return nil, err
		}
		v = &pb_gnmi.TypedValue{
			Value: &pb_gnmi.TypedValue_JsonVal{
				JsonVal: []byte(msg),
			},
		}
	}

	return v, nil
}
