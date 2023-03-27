package main

import (
	"net"
	"os"
	"strings"
	"time"

	pb "github.com/freeconf/gnmi/pb/gnmi"
	"github.com/freeconf/gnmi/server"
	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
	"google.golang.org/grpc"
)

// write your code to capture your domain however you want
type Car struct {
	Speed int
	Miles float64
}

func (c *Car) Start() {
	for {
		<-time.After(time.Second)
		c.Miles += float64(c.Speed)
	}
}

// write mangement api to bridge from YANG to code
func manage(car *Car) node.Node {
	return &nodeutil.Extend{

		// use reflect when possible, here we're using to get/set speed AND
		// to read miles metrics.
		Base: nodeutil.ReflectChild(car),

		// handle action request
		OnAction: func(parent node.Node, req node.ActionRequest) (node.Node, error) {
			switch req.Meta.Ident() {
			case "reset":
				car.Miles = 0
			}
			return nil, nil
		},
	}
}

// Connect everything together into a server to start up
func main() {

	// Your app
	car := &Car{}

	// Device can hold multiple modules, here we are only adding one
	d := device.New(source.Path(os.Getenv("YANGPATH")))
	if err := d.Add("car", manage(car)); err != nil {
		panic(err)
	}

	// Select wire-protocol RESTCONF to serve the device.
	restconf.NewServer(d)

	gsrv := grpc.NewServer()
	drv := &server.Driver{Device: d}
	pb.RegisterGNMIServer(gsrv, drv)

	lis, err := net.Listen("tcp", "127.0.0.1:8090")
	if err != nil {
		panic(err)
	}
	gsrv.Serve(lis)
	//srv.UnhandledRequestHandler = gsrv.ServeHTTP

	// apply start-up config normally stored in a config file on disk
	config := `{
		"fc-restconf":{
			"web":{
				"port":":8080",
				"_tls": {
					"cert": {
						"certFile": "server.crt",
						"keyFile": "server.key"
					}
				}
			}
		},
        "car":{"speed":10}
	}`
	if err := d.ApplyStartupConfig(strings.NewReader(config)); err != nil {
		panic(err)
	}

	// start your app
	car.Start()
}
