package script

import (
	"os"
	"path/filepath"

	"github.com/machbase/neo-server/v8/mods/script/internal/bridge_tengo"
)

type Script interface {
	Run() error
	SetFunc(name string, value func(...any) (any, error))
	SetVar(name string, value any) error
	GetVar(name string, value any) error
}

type Loader interface {
	Load(name string) (Script, error)
	Parse(rawScript []byte) (Script, error)
}

type Option func(sl *loader)

func OptionPath(paths ...string) Option {
	return func(sl *loader) {
		sl.paths = append(sl.paths, paths...)
	}
}

func NewLoader(opts ...Option) Loader {
	sl := &loader{}
	for _, ot := range opts {
		ot(sl)
	}
	return sl
}

type loader struct {
	paths []string
}

func (ld *loader) Load(name string) (Script, error) {
	var content []byte
	for _, p := range ld.paths {
		file := filepath.Join(p, name+".tengo")
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		content, err = os.ReadFile(file)
		if err != nil {
			continue
		}
	}
	if len(content) == 0 {
		return nil, os.ErrNotExist
	}

	return ld.Parse(content)
}

func (ld *loader) Parse(rawScript []byte) (Script, error) {
	var sc Script
	var err error
	sc, err = bridge_tengo.New(rawScript)
	if err != nil {
		return nil, err
	}
	return sc, nil
}
