package gnmi

import (
	"github.com/freeconf/yang/node"
	"github.com/freeconf/yang/nodeutil"
)

func Manage(s *Server) node.Node {
	return &nodeutil.Basic{
		OnChild: func(r node.ChildRequest) (child node.Node, err error) {
			switch r.Meta.Ident() {
			case "web":
				return options(s), nil
			}
			return nil, nil
		},
	}
}

func options(s *Server) node.Node {
	opts := s.Options()
	return &nodeutil.Extend{
		Base: nodeutil.ReflectChild(&opts),
		OnEndEdit: func(parent node.Node, r node.NodeRequest) error {
			if err := parent.EndEdit(r); err != nil {
				return err
			}
			return s.Apply(opts)
		},
	}
}
