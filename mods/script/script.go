package script

import "github.com/machbase/neo-server/mods/script/internal/bridge_tengo"

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

type LoaderOption func(sl *loader)

func PathOption(paths ...string) LoaderOption {
	return func(sl *loader) {
		sl.paths = append(sl.paths, paths...)
	}
}

func NewLoader(opts ...LoaderOption) Loader {
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
	return nil, nil
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
