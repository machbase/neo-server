package connector

import (
	"fmt"

	spi "github.com/machbase/neo-spi"
)

var registry = map[string]Connector{}

func Register(def *Define) (err error) {
	var c Connector
	switch def.Type {
	case SQLITE:
		c = NewSqlite3Connector(def)
		if err = c.BeforeRegister(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("undefined connector type %s", def.Type)
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

func GetConnector(name string) (Connector, error) {
	if c, ok := registry[name]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("undefined connector name '%s'", name)
}

func WrapDatabase(c SqlConnector) (spi.Database, error) {
	return &sqlWrap{SqlConnector: c}, nil
}

// Deprecated: use WrapDatabase() instead
func GetDatabaseConnector(name string) (spi.Database, error) {
	c, err := GetConnector(name)
	if err != nil {
		return nil, err
	}
	if sc, ok := c.(SqlConnector); ok {
		return WrapDatabase(sc)
	} else {
		return nil, fmt.Errorf("incompatible sql connector '%s'", name)
	}
}
