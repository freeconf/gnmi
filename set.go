package gnmi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

func set(d device.Device, ctx context.Context, req *pb_gnmi.SetRequest) (*pb_gnmi.SetResponse, error) {
	var updates []*pb_gnmi.UpdateResult
	// order according to gNMI spec should be delete, replace then update
	for _, del := range req.Delete {
		sel, err := selectFullPath(d, req.Prefix, del)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("del request %s", sel.Path)
		err = sel.Delete()
		if err != nil {
			return nil, err
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Op:   pb_gnmi.UpdateResult_DELETE,
			Path: del,
		})
	}
	for _, u := range req.Replace {
		sel, err := selectFullPath(d, req.Prefix, u.Path)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("replace request %s", sel.Path)
		if sel == nil {
			return nil, fmt.Errorf("no selection found at %s", u.String())
		}
		err = setVal(sel, modeReplace, u.Val)
		if err != nil {
			return nil, err
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Op:   pb_gnmi.UpdateResult_REPLACE,
			Path: u.Path,
		})
	}
	for _, u := range req.Update {
		sel, err := selectFullPath(d, req.Prefix, u.Path)
		if err != nil {
			return nil, err
		}
		fc.Debug.Printf("update request %s", sel.Path)
		err = setVal(sel, modePatch, u.Val)
		if err != nil {
			return nil, err
		}
		if sel == nil {
			return nil, fmt.Errorf("no selection found at %s", u.String())
		}
		updates = append(updates, &pb_gnmi.UpdateResult{
			Op:   pb_gnmi.UpdateResult_UPDATE,
			Path: u.Path,
		})
	}

	return &pb_gnmi.SetResponse{
		Timestamp: time.Now().UnixNano(),
		Response:  updates,
	}, nil
}

const (
	modePatch = iota
	modeReplace
)

var errTypeNotSupported = errors.New("gnmi encoding type not supported")

func setVal(sel *node.Selection, mode int, v *pb_gnmi.TypedValue) error {
	if v == nil {
		return fmt.Errorf("empty value for %s", sel.Path)
	}
	if meta.IsLeaf(sel.Path.Meta) {
		var vstr string
		switch x := v.Value.(type) {
		case *pb_gnmi.TypedValue_JsonIetfVal:
			vstr = string(x.JsonIetfVal)
		case *pb_gnmi.TypedValue_JsonVal:
			vstr = string(x.JsonVal)
		default:
			return errTypeNotSupported
		}
		return sel.SetValue(vstr)
	}

	var data string
	switch x := v.Value.(type) {
	case *pb_gnmi.TypedValue_JsonIetfVal:
		data = string(x.JsonIetfVal)
	case *pb_gnmi.TypedValue_JsonVal:
		data = string(x.JsonVal)
	default:
		return errTypeNotSupported
	}
	n, err := nodeutil.ReadJSON(data)
	if err != nil {
		return err
	}
	switch mode {
	case modePatch:
		if err := sel.UpsertFrom(n); err != nil {
			return err
		}
	case modeReplace:
		if err := sel.ReplaceFrom(n); err != nil {
			return err
		}
	}
	return nil
}
