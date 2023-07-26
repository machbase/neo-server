package bridge

import (
	"fmt"

	"github.com/machbase/neo-server/mods/bridge/internal/postgres"
	"github.com/machbase/neo-server/mods/bridge/internal/sqlite3"
)

var registry = map[string]Bridge{}

func Register(def *Define) (err error) {
	var sqlBr SqlBridge
	switch def.Type {
	case SQLITE:
		sqlBr = sqlite3.New(def.Name, def.Path)
	case POSTGRES:
		sqlBr = postgres.New(def.Name, def.Path)
	default:
		return fmt.Errorf("undefined bridge type %s, unable to register", def.Type)
	}
	if sqlBr != nil {
		if err = sqlBr.BeforeRegister(); err != nil {
			return err
		}
		registry[def.Name] = sqlBr
		return nil
	} else {
		// never happen, for now.
		return nil
	}
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

func GetSqlBridge(name string) (SqlBridge, error) {
	br, err := GetBridge(name)
	if err != nil {
		return nil, err
	}

	if sqlBr, ok := br.(SqlBridge); ok {
		return sqlBr, nil
	} else {
		return nil, fmt.Errorf("'%s' is not a SqlBridge", name)
	}
}
