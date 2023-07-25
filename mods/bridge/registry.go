package bridge

import (
	"fmt"
)

var registry = map[string]Bridge{}

func Register(def *Define) (err error) {
	var c Bridge
	switch def.Type {
	case SQLITE:
		c = NewSqlite3Bridge(def)
		if err = c.BeforeRegister(); err != nil {
			return err
		}
	case POSTGRES:
		c = NewPostgresBridge(def)
		if err = c.BeforeRegister(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("undefined bridge type %s, unable to register", def.Type)
	}
	registry[def.Name] = c
	return nil
}

func Unregister(name string) {
	if c, ok := registry[name]; ok {
		delete(registry, name)
		c.AfterUnregister()
	}
}

func UnregisterAll() {
	for name := range registry {
		Unregister(name)
	}
}

func GetBridge(name string) (Bridge, error) {
	if c, ok := registry[name]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("undefined bridge name '%s'", name)
}
