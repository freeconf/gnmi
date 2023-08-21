package gnmi

import (
	"embed"

	"github.com/freeconf/restconf"
	"github.com/freeconf/yang/source"
)

//go:embed yang/*.yang
var internal embed.FS

// Access to fc-gnmi yang definitions.
var InternalYPath = source.Any(restconf.InternalYPath, source.EmbedDir(internal, "yang"))
