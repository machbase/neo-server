package tql

import (
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/util/ssfs"
)

type VolatileAssetsProvider interface {
	VolatileFilePrefix() string
	VolatileFileWrite(name string, val []byte, deadline time.Time) fs.File
}

type Loader interface {
	Load(path string) (*Script, error)
	SetVolatileAssetsProvider(vap VolatileAssetsProvider)
}

type loader struct {
	vap VolatileAssetsProvider
}

func NewLoader() Loader {
	return &loader{}
}

func (ld *loader) Load(path string) (*Script, error) {
	var ret *Script
	fsmgr := ssfs.Default()
	ent, err := fsmgr.Get("/" + strings.TrimPrefix(path, "/"))
	if err != nil || ent.IsDir {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	ret = &Script{
		path0:   path,
		content: ent.Content,
		vap:     ld.vap,
	}
	return ret, nil
}

func (ld *loader) SetVolatileAssetsProvider(p VolatileAssetsProvider) {
	ld.vap = p
}

type Script struct {
	path0   string
	content []byte
	vap     VolatileAssetsProvider
}

func (sc *Script) String() string {
	return fmt.Sprintf("path: %s", sc.path0)
}
