package bridge

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/mods/bridge/internal/mqtt"
	"github.com/machbase/neo-server/mods/bridge/internal/mysql"
	"github.com/machbase/neo-server/mods/bridge/internal/postgres"
	"github.com/machbase/neo-server/mods/bridge/internal/sqlite3"
)

var registry = map[string]Bridge{}
var registryLock sync.RWMutex

func Register(def *Define) (err error) {
	registryLock.Lock()
	defer registryLock.Unlock()

	var br Bridge
	switch def.Type {
	case SQLITE:
		var b SqlBridge = sqlite3.New(def.Name, def.Path)
		br = b
	case POSTGRES:
		var b SqlBridge = postgres.New(def.Name, def.Path)
		br = b
	case MYSQL:
		var b SqlBridge = mysql.New(def.Name, def.Path)
		br = b
	case MQTT:
		var b MqttBridge = mqtt.New(def.Name, def.Path)
		br = b
	default:
		return fmt.Errorf("undefined bridge type %s, unable to register", def.Type)
	}

	if br != nil {
		if err = br.BeforeRegister(); err != nil {
			return err
		}
		registry[def.Name] = br
		return nil
	} else {
		// never happen, for now.
		return nil
	}
}

func Unregister(name string) {
	registryLock.Lock()
	defer registryLock.Unlock()

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
	registryLock.RLock()
	defer registryLock.RUnlock()

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
