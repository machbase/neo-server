package connector

import (
	"fmt"
)

var registry = map[string]Connector{}

func Register(def *Define) (err error) {
	var c Connector
	switch def.Type {
	case SQLITE:
		c = NewSqlite3Connector(def)
		if err = c.BeforeRegister(); err != nil {
			return
		}
	default:
		return fmt.Errorf("undefined connector type %s", def.Type)
	}
	registry[def.Name] = c
	return nil
}

func Unregister(name string) {
	delete(registry, name)
}

func GetConnector(name string) (Connector, error) {
	if c, ok := registry[name]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("undefined connector name %s", name)
}
