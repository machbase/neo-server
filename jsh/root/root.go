package root

import (
	"embed"
	"io/fs"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

//go:embed embed/*
var rootFS embed.FS

func RootFSTab() engine.FSTab {
	fsDir, _ := fs.Sub(rootFS, "embed")
	return engine.FSTab{MountPoint: "/", FS: fsDir}
}
