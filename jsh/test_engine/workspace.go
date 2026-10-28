package test_engine

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

// Workspace adapts engine.FS to workspace-relative readers in consumer tests.
// Server mount parsing and file API policies are tested in mods/server.
type Workspace struct {
	fs      *engine.FS
	rootDir string
}

func NewWorkspace(tabs ...engine.FSTab) (*Workspace, error) {
	ret := &Workspace{fs: engine.NewFS()}
	for _, tab := range tabs {
		dir, err := filepath.Abs(tab.Source)
		if err != nil {
			return nil, err
		}
		if err := ret.fs.MountStrict(tab.MountPoint, os.DirFS(dir)); err != nil {
			return nil, err
		}
		if tab.MountPoint == "/work" {
			ret.rootDir = dir
		}
	}
	return ret, nil
}

func (w *Workspace) OpenFile(name string) (io.ReadCloser, error) {
	return w.fs.Open("/work" + path.Clean("/"+filepath.ToSlash(name)))
}

func (w *Workspace) Stat(name string) (os.FileInfo, error) {
	return w.fs.Stat("/work" + path.Clean("/"+filepath.ToSlash(name)))
}

func (w *Workspace) RootDir() (string, bool) {
	return w.rootDir, w.rootDir != ""
}
