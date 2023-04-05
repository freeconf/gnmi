package gnmi

import (
	"fmt"
	"net"

	"github.com/freeconf/restconf/device"
	pb_gnmi "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
)

var Version = "0.0.0"

type ServerOpts struct {
	Port string
}

type Server struct {
	opts       ServerOpts
	grpcServer *grpc.Server
	listener   net.Listener
	driver     *driver
	device     *device.Local
}

func NewServer(d *device.Local) *Server {
	s := &Server{device: d}

	if err := d.Add("fc-gnmi", Manage(s)); err != nil {
		panic(err)
	}

	return s
}

func (s *Server) Options() ServerOpts {
	return s.opts
}

func (s *Server) Apply(opts ServerOpts) error {
	var err error
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
		s.grpcServer = nil
	}
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
	s.grpcServer = grpc.NewServer()
	s.driver = newDriver(s.device)
	pb_gnmi.RegisterGNMIServer(s.grpcServer, s.driver)
	s.listener, err = net.Listen("tcp", opts.Port)
	if err != nil {
		return err
	}
	s.opts = opts
	s.start()
	return nil
}

func (s *Server) start() {
	go func() {
		if err := s.grpcServer.Serve(s.listener); err != nil {
			panic(fmt.Sprintf("error starting or stopping gRPC server. %s", err))
		}
	}()
}
