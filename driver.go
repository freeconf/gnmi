package gnmi

import (
	"context"

	"github.com/freeconf/restconf/device"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

/*
driver bridges between gnmi and FreeCONF. mapping GNMI commands to node operations
*/
type driver struct {
	device device.Device
	subs   *subService
	pb_gnmi.UnimplementedGNMIServer
}

func newDriver(d device.Device) *driver {
	return &driver{
		device: d,
		subs:   &subService{},
	}
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

func (d *driver) Set(ctx context.Context, req *pb_gnmi.SetRequest) (*pb_gnmi.SetResponse, error) {
	return set(d.device, ctx, req)
}

func (d *driver) Get(ctx context.Context, req *pb_gnmi.GetRequest) (*pb_gnmi.GetResponse, error) {
	return get(d.device, ctx, req)
}

func (d *driver) Subscribe(server pb_gnmi.GNMI_SubscribeServer) error {
	return d.subs.subscribe(d.device, server)
}
