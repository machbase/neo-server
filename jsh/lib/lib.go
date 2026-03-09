package lib

import (
	_ "embed"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib/db"
	"github.com/machbase/neo-server/v8/jsh/lib/http"
	"github.com/machbase/neo-server/v8/jsh/lib/machcli"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/filter"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/interp"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/mat"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/simplex"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/spatial"
	"github.com/machbase/neo-server/v8/jsh/lib/mqtt"
	"github.com/machbase/neo-server/v8/jsh/lib/net"
	"github.com/machbase/neo-server/v8/jsh/lib/opcua"
	"github.com/machbase/neo-server/v8/jsh/lib/os"
	"github.com/machbase/neo-server/v8/jsh/lib/parser"
	"github.com/machbase/neo-server/v8/jsh/lib/pretty"
	"github.com/machbase/neo-server/v8/jsh/lib/publisher"
	"github.com/machbase/neo-server/v8/jsh/lib/readline"
	"github.com/machbase/neo-server/v8/jsh/lib/shell"
	"github.com/machbase/neo-server/v8/jsh/lib/stream"
	"github.com/machbase/neo-server/v8/jsh/lib/system"
	"github.com/machbase/neo-server/v8/jsh/lib/util"
	"github.com/machbase/neo-server/v8/jsh/lib/uuid"
	"github.com/machbase/neo-server/v8/jsh/lib/ws"
	"github.com/machbase/neo-server/v8/jsh/lib/zlib"
)

var libFS = engine.NewVirtualFS()

func LibFSTab() engine.FSTab {
	return engine.FSTab{MountPoint: "/lib", FS: libFS}
}

func LibFS() *engine.VirtualFS {
	return libFS
}

func addFiles(files map[string][]byte) {
	for name, content := range files {
		libFS.AddFile(name, engine.VirtualFileContent(content), engine.VirtualFileProperty{Mode: 0444})
	}
}

//go:embed path.js
var path_js []byte

func libFiles() map[string][]byte {
	return map[string][]byte{
		"path.js": path_js,
	}
}

// Enable registers all native modules to the given JSRuntime
// If you want to link only specific modules, register them individually.
func Enable(n *engine.JSRuntime) {
	// engine modules
	n.RegisterNativeModule("@jsh/process", n.Process)
	addFiles(n.ProcessFiles())
	n.RegisterNativeModule("@jsh/fs", n.Filesystem)
	// lib files
	addFiles(libFiles())

	// native modules
	n.RegisterNativeModule("@jsh/db", db.Module)
	n.RegisterNativeModule("@jsh/http", http.Module)
	addFiles(http.Files())
	n.RegisterNativeModule("@jsh/machcli", machcli.Module)
	addFiles(machcli.Files())
	n.RegisterNativeModule("@jsh/mathx", mathx.Module)
	addFiles(mathx.Files())
	n.RegisterNativeModule("@jsh/mathx/filter", filter.Module)
	addFiles(filter.Files())
	n.RegisterNativeModule("@jsh/mathx/interp", interp.Module)
	addFiles(interp.Files())
	n.RegisterNativeModule("@jsh/mathx/mat", mat.Module)
	addFiles(mat.Files())
	n.RegisterNativeModule("@jsh/mathx/simplex", simplex.Module)
	addFiles(simplex.Files())
	n.RegisterNativeModule("@jsh/mathx/spatial", spatial.Module)
	addFiles(spatial.Files())
	n.RegisterNativeModule("@jsh/mqtt", mqtt.Module)
	addFiles(mqtt.Files())
	n.RegisterNativeModule("@jsh/net", net.Module)
	addFiles(net.Files())
	n.RegisterNativeModule("@jsh/opcua", opcua.Module)
	addFiles(opcua.Files())
	n.RegisterNativeModule("@jsh/os", os.Module)
	addFiles(os.Files())
	n.RegisterNativeModule("@jsh/parser", parser.Module)
	addFiles(parser.Files())
	n.RegisterNativeModule("@jsh/pretty", pretty.Module)
	addFiles(pretty.Files())
	n.RegisterNativeModule("@jsh/publisher", publisher.Module)
	n.RegisterNativeModule("@jsh/readline", readline.Module)
	addFiles(readline.Files())
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	n.RegisterNativeModule("@jsh/stream", stream.Module)
	addFiles(stream.Files())
	n.RegisterNativeModule("@jsh/system", system.Module)
	n.RegisterNativeModule("@jsh/util", util.Module)
	addFiles(util.Files())
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	addFiles(ws.Files())
	n.RegisterNativeModule("@jsh/uuid", uuid.Module)
	addFiles(uuid.Files())
	n.RegisterNativeModule("@jsh/zlib", zlib.Module)
	addFiles(zlib.Files())
}
