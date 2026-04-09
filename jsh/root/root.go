package root

import (
	"embed"
	"io/fs"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/sbin"
	"github.com/machbase/neo-server/v8/jsh/usr"
)

//go:embed embed/*
var rootFS embed.FS

func RootFSTab() engine.FSTab {
	ret := engine.NewFS()
	dir, _ := fs.Sub(rootFS, "embed")
	ret.Mount("/", dir)
	ret.Mount("/sbin", sbin.Files)
	ret.Mount("/usr", usr.Files)
	return engine.FSTab{MountPoint: "/", FS: ret}
}
