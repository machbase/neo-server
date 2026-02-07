package native

import (
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/http"
	"github.com/machbase/neo-server/v8/jsh/native/mqtt"
	"github.com/machbase/neo-server/v8/jsh/native/net"
	"github.com/machbase/neo-server/v8/jsh/native/os"
	"github.com/machbase/neo-server/v8/jsh/native/parser"
	"github.com/machbase/neo-server/v8/jsh/native/pretty"
	"github.com/machbase/neo-server/v8/jsh/native/readline"
	"github.com/machbase/neo-server/v8/jsh/native/shell"
	"github.com/machbase/neo-server/v8/jsh/native/stream"
	"github.com/machbase/neo-server/v8/jsh/native/ws"
	"github.com/machbase/neo-server/v8/jsh/native/zlib"
)

func Enable(n *engine.JSRuntime) {
	n.RegisterNativeModule("@jsh/process", n.Process)
	n.RegisterNativeModule("@jsh/fs", n.Filesystem)
	n.RegisterNativeModule("@jsh/os", os.Module)
	n.RegisterNativeModule("@jsh/shell", shell.Module)
	n.RegisterNativeModule("@jsh/readline", readline.Module)
	n.RegisterNativeModule("@jsh/http", http.Module)
	n.RegisterNativeModule("@jsh/ws", ws.Module)
	n.RegisterNativeModule("@jsh/mqtt", mqtt.Module)
	n.RegisterNativeModule("@jsh/stream", stream.Module)
	n.RegisterNativeModule("@jsh/zlib", zlib.Module)
	n.RegisterNativeModule("@jsh/net", net.Module)
	n.RegisterNativeModule("@jsh/parser", parser.Module)
	n.RegisterNativeModule("@jsh/pretty", pretty.Module)
}
