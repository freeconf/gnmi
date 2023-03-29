package gnmi

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/freeconf/gnmi/testdata"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/fc"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/parser"
	"github.com/freeconf/yang/source"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

var updateFlag = flag.Bool("update", false, "update golden files instead of verifying against them")

func newTestDevice() (*testdata.Car, *device.Local) {
	ypath := source.Dir("testdata")
	d := device.New(ypath)
	c := testdata.New()
	n := testdata.Manage(c)
	d.Add("car", n)
	return c, d
}

func TestGet(t *testing.T) {
	_, dev := newTestDevice()
	drv := &driver{device: dev}
	ctx := context.TODO()
	req := &pb_gnmi.GetRequest{
		Path: []*pb_gnmi.Path{
			{Origin: "car"},
		},
	}
	resp, err := drv.Get(ctx, req)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, 1, len(resp.Notification))
	fc.AssertEqual(t, 1, len(resp.Notification[0].Update))
	fc.Gold(t, *updateFlag, resp.Notification[0].Update[0].Val.GetJsonVal(), "testdata/get-gold.json")
}

func TestSet(t *testing.T) {
	car, dev := newTestDevice()
	drv := &driver{device: dev}
	ctx := context.TODO()
	req := &pb_gnmi.SetRequest{
		Update: []*pb_gnmi.Update{
			{
				Path: &pb_gnmi.Path{
					Origin: "car",
				},
				Val: &pb_gnmi.TypedValue{
					Value: &pb_gnmi.TypedValue_JsonVal{
						JsonVal: []byte(`{"speed":100}`),
					},
				},
			},
		},
	}
	resp, err := drv.Set(ctx, req)
	fc.AssertEqual(t, nil, err)
	fc.AssertEqual(t, 1, len(resp.Response))
	fc.AssertEqual(t, true, resp.Response[0].Path != nil)
	fc.AssertEqual(t, 100, car.Speed)
}

var mstr = `module x {
	container me {
		uses user;
	}
	list users {
		key name;
		uses user;
	}
	grouping user {
		leaf name {
			type string;
		}
		leaf skill {
			type enumeration {
				enum mechanic;
				enum welder;
				enum manager;
			}
		}
		leaf address {
			type string;
		}
	}
}`

func newTest2Device(initial map[string]interface{}) *device.Local {
	m, err := parser.LoadModuleFromString(nil, mstr)
	if err != nil {
		panic(err)
	}
	d := device.New(nil)
	n := nodeutil.ReflectChild(initial)
	b := node.NewBrowser(m, n)
	d.AddBrowser(b)
	return d
}

type editTestOp struct {
	path *pb_gnmi.Path
	data string
}

func TestSet2(t *testing.T) {

	tests := []struct {
		name    string
		update  []editTestOp
		replace []editTestOp
		del     []*pb_gnmi.Path
	}{
		{
			name: "basic",
			update: []editTestOp{
				{
					path: &pb_gnmi.Path{Origin: "x"},
					data: `{"me":{"name":"bob"}}`,
				},
			},
		},
		{
			name: "del",
			del: []*pb_gnmi.Path{
				{
					Origin: "x", Elem: []*pb_gnmi.PathElem{
						{Name: "me"},
					},
				},
			},
		},
		{
			name: "replace",
			replace: []editTestOp{
				{
					path: &pb_gnmi.Path{
						Origin: "x", Elem: []*pb_gnmi.PathElem{
							{Name: "me"},
						},
					},
					data: `{"me":{"name":"barb", "skill":"welder"}}`,
				},
			},
		},
	}

	for _, test := range tests {
		data := map[string]interface{}{
			"me": map[string]interface{}{
				"name":    "joe",
				"skill":   "manager",
				"address": "123 mockingbird lane.",
			},
			"users": []map[string]interface{}{
				{"name": "mary", "skill": "welder"},
				{"name": "john", "skill": "mechanic"},
			},
		}
		dev := newTest2Device(data)
		drv := &driver{device: dev}
		ctx := context.TODO()
		req := &pb_gnmi.SetRequest{}
		for _, u := range test.update {
			req.Update = append(req.Update, &pb_gnmi.Update{
				Path: u.path,
				Val: &pb_gnmi.TypedValue{
					Value: &pb_gnmi.TypedValue_JsonVal{
						JsonVal: []byte(u.data),
					},
				},
			})
		}
		for _, u := range test.replace {
			req.Replace = append(req.Replace, &pb_gnmi.Update{
				Path: u.path,
				Val: &pb_gnmi.TypedValue{
					Value: &pb_gnmi.TypedValue_JsonVal{
						JsonVal: []byte(u.data),
					},
				},
			})
		}
		req.Delete = test.del

		resp, err := drv.Set(ctx, req)
		fc.AssertEqual(t, nil, err)
		count := len(test.update) + len(test.del) + len(test.replace)
		fc.AssertEqual(t, count, len(resp.Response))
		b, _ := dev.Browser("x")
		actual, err := nodeutil.WriteJSON(b.Root())
		fc.AssertEqual(t, nil, err)
		fc.Gold(t, *updateFlag, []byte(actual), fmt.Sprintf("testdata/set-%s-gold.json", test.name))
	}
}
