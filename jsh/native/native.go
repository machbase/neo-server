package native

import (
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/db"
	"github.com/machbase/neo-server/v8/jsh/native/filter"
	"github.com/machbase/neo-server/v8/jsh/native/generator"
	"github.com/machbase/neo-server/v8/jsh/native/http"
	"github.com/machbase/neo-server/v8/jsh/native/machcli"
	"github.com/machbase/neo-server/v8/jsh/native/matrix"
	"github.com/machbase/neo-server/v8/jsh/native/mqtt"
	"github.com/machbase/neo-server/v8/jsh/native/net"
	"github.com/machbase/neo-server/v8/jsh/native/opcua"
	"github.com/machbase/neo-server/v8/jsh/native/os"
	"github.com/machbase/neo-server/v8/jsh/native/parser"
	"github.com/machbase/neo-server/v8/jsh/native/pretty"
	"github.com/machbase/neo-server/v8/jsh/native/publisher"
	"github.com/machbase/neo-server/v8/jsh/native/readline"
	"github.com/machbase/neo-server/v8/jsh/native/shell"
	"github.com/machbase/neo-server/v8/jsh/native/spatial"
	"github.com/machbase/neo-server/v8/jsh/native/stats"
	"github.com/machbase/neo-server/v8/jsh/native/stream"
	"github.com/machbase/neo-server/v8/jsh/native/system"
	"github.com/machbase/neo-server/v8/jsh/native/ws"
	"github.com/machbase/neo-server/v8/jsh/native/zlib"
)

// Enable registers all native modules to the given JSRuntime
// If you want to link only specific modules, register them individually.
func Enable(n *engine.JSRuntime) {
	// engine modules
	n.RegisterNativeModule("@jsh/process", n.Process)
	n.RegisterNativeModule("@jsh/fs", n.Filesystem)

	// native modules
	n.RegisterNativeModule("@jsh/db", db.Module)
	n.RegisterNativeModule("@jsh/filter", filter.Module)
	n.RegisterNativeModule("@jsh/generator", generator.Module)
	n.RegisterNativeModule("@jsh/http", http.Module)
	n.RegisterNativeModule("@jsh/machcli", machcli.Module)
	n.RegisterNativeModule("@jsh/matrix", matrix.Module)
	n.RegisterNativeModule("@jsh/mqtt", mqtt.Module)
	n.RegisterNativeModule("@jsh/net", net.Module)
	n.RegisterNativeModule("@jsh/opcua", opcua.Module)
	n.RegisterNativeModule("@jsh/os", os.Module)
	n.RegisterNativeModule("@jsh/parser", parser.Module)
	n.RegisterNativeModule("@jsh/pretty", pretty.Module)
	n.RegisterNativeModule("@jsh/publisher", publisher.Module)
	n.RegisterNativeModule("@jsh/readline", readline.Module)
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	n.RegisterNativeModule("@jsh/spatial", spatial.Module)
	n.RegisterNativeModule("@jsh/stats", stats.Module)
	n.RegisterNativeModule("@jsh/stream", stream.Module)
	n.RegisterNativeModule("@jsh/system", system.Module)
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	n.RegisterNativeModule("@jsh/zlib", zlib.Module)
}
