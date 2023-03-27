package server

import (
	"context"

	pb "github.com/freeconf/gnmi/pb/gnmi"
	"github.com/freeconf/restconf/device"
)

var Version = "0.0.0"

type Driver struct {
	Device *device.Local
	pb.UnimplementedGNMIServer
}

func (d *Driver) Capabilities(ctx context.Context, req *pb.CapabilityRequest) (*pb.CapabilityResponse, error) {
	resp := &pb.CapabilityResponse{
		SupportedModels: nil,
		SupportedEncodings: []pb.Encoding{
			pb.Encoding_JSON,
			pb.Encoding_JSON_IETF,
		},
		GNMIVersion: Version,
	}
	for moduleName, module := range d.Device.Modules() {
		md := &pb.ModelData{
			Name:         moduleName,
			Organization: module.Organization(),
			Version:      module.Revision().Ident(),
		}
		resp.SupportedModels = append(resp.SupportedModels, md)
	}

	return resp, nil
}

func (d *Driver) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	return nil, nil
}

func (d *Driver) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	return nil, nil
}

func (d *Driver) Subscribe(pb.GNMI_SubscribeServer) error {
	return nil
}
