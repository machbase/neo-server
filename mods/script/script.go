package script

import "github.com/machbase/neo-server/mods/script/internal/bridge_tengo"

type Script interface {
	Run() error
	SetFunc(name string, value func(...any) (any, error))
	SetVar(name string, value any) error
	GetVar(name string, value any) error
}

func NewScript(rawScript []byte) (Script, error) {
	var sc Script
	var err error
	sc, err = bridge_tengo.New(rawScript)
	if err != nil {
		return nil, err
	}
	return sc, nil
}
