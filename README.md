# ![FreeCONF](https://s3.amazonaws.com/freeconf-static/freeconf-no-wrench.svg)

For more information about this project, [see docs](https://freeconf.org/).

# gNMI server library

gNMI is a management standard to manage a system's config and metrics. More information can be found on [openconfig.net website](https://www.openconfig.net/docs/).


# Requirements

Requires Go version 1.20 or greater.

# Getting the source

```bash
go get -u github.com/freeconf/gnmi
```

# What can I do with this library?

Once you add this library to your application, your application can be managed by any gNMI compatible tools like [gNMIc for example](https://gnmic.kmrd.dev/).

# Example

### Step 1 - Create a Go project modeling a car
```bash
mkdir car
cd car
go mod init car
go get -u github.com/freeconf/gnmi
```

### Step 2 - Get root model files

There are some model files needed to start a web server and other basic things.

```bash
go run github.com/freeconf/yang/cmd/fc-yang get
```

you should now see bunch of *.yang files in the current directory.  They were actually extracted from the source, not downloaded.

### Step 3 - Write your own model file

Use [YANG](https://tools.ietf.org/html/rfc6020) to model your management API by creating the following file called `car.yang` with the following contents.

```YANG
module car {
	description "Car goes beep beep";

	revision 0;

	leaf speed {
		description "How fast the car goes";
	    type int32 {
		    range "0..120";
	    }
		units milesPerSecond;
	}

	leaf miles {
		description "How many miles has car moved";
	    type decimal64;
	    config false;
	}

	rpc reset {
		description "Reset the odometer";
	}
}
```

### Step 4 - Write a program, its management API and a main entry point

Create a go source file called `main.go` with the following contents.

```go
package main

import (
	"strings"
	"time"

	"github.com/freeconf/restconf"
	"github.com/freeconf/restconf/device"
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
	"github.com/freeconf/yang/source"
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
	d := device.New(source.Path("."))
	if err := d.Add("car", manage(car)); err != nil {
		panic(err)
	}

	// Select wire-protocol RESTCONF to serve the device.
	restconf.NewServer(d)

	// apply start-up config normally stored in a config file on disk
	config := `{
		"fc-restconf":{"web":{"port":":8080"}},
        "car":{"speed":10}
	}`
	if err := d.ApplyStartupConfig(strings.NewReader(config)); err != nil {
		panic(err)
	}

	// start your app
	car.Start()
}
```

### Step 5. Now run your program

Start your application

```bash
go run . &
```

You will see a warning about HTTP2, but you can ignore that.  Once you install a web certificate, that will go away.

#### Get Configuration using `gNMIc`

Here we will use the `gNMIc` client to interact with our car application.


`curl http://localhost:8080/restconf/data/car:`

```json
{"speed":10,"miles":450}
```

#### Change Configuration
`curl -XPUT http://localhost:8080/restconf/data/car: -d '{"speed":99}'`

#### Reset odometer
`curl -XPOST http://localhost:8080/restconf/data/car:reset`

## Compliance with RFC

Interop is important, include proper headers and all input and output will be in strict compliance w/RFC.  Major differences is namespaced JSON and slightly different base path for RPCs.  You can disallow non-compliance in API.

`curl -H 'Accept:application/yang-data+json' http://localhost:8080/restconf/data/car:`

```json
{"car:speed":99,"car:miles":3626}
```

`curl -H 'Accept:application/yang-data+json' http://localhost:8080/restconf/operations/car:reset`


## Resources
* [Docs](https://freeconf.org/docs/)
* [Discussions](https://github.com/freeconf/restconf/discussions)
* [Issues](https://github.com/freeconf/gnmi/issues)
