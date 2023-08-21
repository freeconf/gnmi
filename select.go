package gnmi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/meta"
	"github.com/freeconf/yang/node"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

var errModelOrOrigin = errors.New("you must use models or use origin as model")

// TBH, i don't understand the use case for multiple models
var errOnlyOneModel = errors.New("only one model is currently supported")

var errNoSelection = errors.New("no prefix or path found")

var errKeysWhenNoList = errors.New("found keys when model is not a list")

func selectPath(device device.Device, models []*pb_gnmi.ModelData, path *pb_gnmi.Path) (*node.Selection, error) {
	var model string
	if len(models) > 0 {
		if len(models) > 1 {
			return nil, errOnlyOneModel
		}
		model = models[0].Name
	} else if path == nil {
		return nil, nil
	} else if path.Origin == "" && len(path.Elem) > 0 {
		return nil, errModelOrOrigin
	} else {
		model = path.Origin
	}
	b, err := device.Browser(model)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("no module with name '%s' found", models[0].Name)
	}
	s := b.Root()
	ptr := s
	if path != nil && len(path.Elem) > 0 {
		s, err = advanceSelection(device, ptr, path)
		if err != nil {
			return nil, err
		}
		ptr = s
	}
	return ptr, nil
}

func advanceSelection(device device.Device, prefix *node.Selection, path *pb_gnmi.Path) (*node.Selection, error) {
	if prefix == nil && path == nil {
		return nil, errNoSelection
	}
	ptr := prefix
	if prefix == nil {
		if path == nil {
			return nil, errNoSelection
		}
		if path.Origin == "" {
			return nil, errModelOrOrigin
		}
		var err error
		if ptr, err = selectPath(device, nil, path); err != nil {
			return nil, err
		}
		return ptr, nil
	}

	if ptr == nil {
		return nil, errNoSelection
	}

	if path != nil {
		for _, seg := range path.Elem {
			if seg == nil || seg.Name == "" {
				continue
			}
			ident := seg.Name
			if len(seg.Key) > 0 {
				lmeta, valid := ptr.Meta().(*meta.List)
				if !valid {
					return nil, errKeysWhenNoList
				}
				ident = ident + "=" + encodeKey(lmeta, seg.Key)
			}
			s, err := ptr.Find(ident) // should find take keys to avoid encode/decoding step?
			if err != nil || s == nil {
				return nil, err
			}
			ptr = s
		}
	}

	return ptr, nil
}

// Posted on 4/3/23 asking question on openconfig google group about how
// set is only method that doesn't have a use_model
func selectFullPath(device device.Device, prefix *pb_gnmi.Path, path *pb_gnmi.Path) (*node.Selection, error) {
	var ptr *node.Selection
	var err error
	if prefix != nil {
		if ptr, err = selectPath(device, nil, prefix); err != nil {
			return nil, err
		}
	}
	if path != nil {
		if ptr == nil {
			if ptr, err = selectPath(device, nil, path); err != nil {
				return nil, err
			}
		} else {
			s, err := advanceSelection(device, ptr, path)
			if err != nil {
				return nil, err
			}
			ptr = s
		}
	}
	if ptr == nil {
		return nil, errNoSelection
	}
	return ptr, nil
}

func encodeKey(m *meta.List, keys map[string]string) string {
	vals := make([]string, len(m.KeyMeta()))
	for i, k := range m.KeyMeta() {
		vals[i] = keys[k.Ident()]
	}
	return strings.Join(vals, ",")
}
