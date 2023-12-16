package tql

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	dirs []string
	vap  VolatileAssetsProvider
}

func NewLoader(dirs []string) Loader {
	abs := []string{}
	for _, d := range dirs {
		ap, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		abs = append(abs, ap)
	}
	return &loader{
		dirs: abs,
	}
}

func (ld *loader) Load(path string) (*Script, error) {
	var ret *Script
	for _, d := range ld.dirs {
		joined := filepath.Join(d, path)
		stat, err := os.Stat(joined)
		if err != nil || stat.IsDir() {
			continue
		}
		if !strings.HasPrefix(joined, d) {
			// check relative path leak
			continue
		}

		ret = &Script{
			path: joined,
			vap:  ld.vap,
		}
		break
	}
	if ret == nil {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	return ret, nil
}

func (ld *loader) SetVolatileAssetsProvider(p VolatileAssetsProvider) {
	ld.vap = p
}

type Script struct {
	path string
	vap  VolatileAssetsProvider
}

func (sc *Script) String() string {
	return fmt.Sprintf("path: %s", sc.path)
}
