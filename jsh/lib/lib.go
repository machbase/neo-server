package lib

import (
	_ "embed"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib/archive/tar"
	"github.com/machbase/neo-server/v8/jsh/lib/archive/zip"
	"github.com/machbase/neo-server/v8/jsh/lib/db"
	"github.com/machbase/neo-server/v8/jsh/lib/git"
	"github.com/machbase/neo-server/v8/jsh/lib/http"
	"github.com/machbase/neo-server/v8/jsh/lib/machcli"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/filter"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/interp"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/mat"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/simplex"
	"github.com/machbase/neo-server/v8/jsh/lib/mathx/spatial"
	"github.com/machbase/neo-server/v8/jsh/lib/mqtt"
	"github.com/machbase/neo-server/v8/jsh/lib/nats"
	"github.com/machbase/neo-server/v8/jsh/lib/net"
	"github.com/machbase/neo-server/v8/jsh/lib/opcua"
	"github.com/machbase/neo-server/v8/jsh/lib/os"
	"github.com/machbase/neo-server/v8/jsh/lib/parser"
	"github.com/machbase/neo-server/v8/jsh/lib/pretty"
	"github.com/machbase/neo-server/v8/jsh/lib/publisher"
	"github.com/machbase/neo-server/v8/jsh/lib/readline"
	"github.com/machbase/neo-server/v8/jsh/lib/semver"
	"github.com/machbase/neo-server/v8/jsh/lib/shell"
	"github.com/machbase/neo-server/v8/jsh/lib/stream"
	"github.com/machbase/neo-server/v8/jsh/lib/system"
	"github.com/machbase/neo-server/v8/jsh/lib/util"
	"github.com/machbase/neo-server/v8/jsh/lib/util/tail"
	"github.com/machbase/neo-server/v8/jsh/lib/uuid"
	"github.com/machbase/neo-server/v8/jsh/lib/vizspec"
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

//go:embed events.js
var events_js []byte

//go:embed fs.js
var fs_js []byte

//go:embed path.js
var path_js []byte

//go:embed process.js
var process_js []byte

//go:embed service.js
var service_js []byte

func libFiles() map[string][]byte {
	return map[string][]byte{
		"events.js":  events_js,
		"fs.js":      fs_js,
		"path.js":    path_js,
		"process.js": process_js,
		"service.js": service_js,
	}
}

// Enable registers all native modules to the given JSRuntime
// If you want to link only specific modules, register them individually.
func Enable(n *engine.JSRuntime) {
	// engine modules
	n.RegisterNativeModule("@jsh/process", n.Process)
	n.RegisterNativeModule("@jsh/fs", n.Filesystem)
	// lib files
	addFiles(libFiles())

	// native modules
	n.RegisterNativeModule("@jsh/archive/tar", tar.Module)
	addFiles(tar.Files())
	n.RegisterNativeModule("@jsh/archive/zip", zip.Module)
	addFiles(zip.Files())
	n.RegisterNativeModule("@jsh/db", db.Module)
	n.RegisterNativeModule("@jsh/git", git.ModuleWithFS(n.MountedFS()))
	addFiles(git.Files())
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
	n.RegisterNativeModule("@jsh/nats", nats.Module)
	addFiles(nats.Files())
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
	n.RegisterNativeModule("@jsh/semver", semver.Module)
	addFiles(semver.Files())
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	addFiles(shell.Files())
	n.RegisterNativeModule("@jsh/stream", stream.Module)
	addFiles(stream.Files())
	n.RegisterNativeModule("@jsh/system", system.Module)
	addFiles(system.Files())
	n.RegisterNativeModule("@jsh/util", util.Module)
	addFiles(util.Files())
	n.RegisterNativeModule("@jsh/util/tail", tail.ModuleWithFS(n.MountedFS()))
	addFiles(tail.Files())
	n.RegisterNativeModule("@jsh/vizspec", vizspec.Module)
	addFiles(vizspec.Files())
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	addFiles(ws.Files())
	n.RegisterNativeModule("@jsh/uuid", uuid.Module)
	addFiles(uuid.Files())
	n.RegisterNativeModule("@jsh/zlib", zlib.Module)
	addFiles(zlib.Files())
}
