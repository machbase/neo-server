package tql

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Loader interface {
	Load(path string) (*Script, error)
}

type loader struct {
	dirs []string
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
		}
		break
	}
	if ret == nil {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	return ret, nil
}

type Script struct {
	path string
}

func (sc *Script) String() string {
	return fmt.Sprintf("path: %s", sc.path)
}
