package bridge

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/v8/mods/bridge/internal/mssql"
	"github.com/machbase/neo-server/v8/mods/bridge/internal/mysql"
	"github.com/machbase/neo-server/v8/mods/bridge/internal/postgres"
	"github.com/machbase/neo-server/v8/mods/bridge/internal/sqlite3"
	"github.com/machbase/neo-server/v8/mods/model"
)

var registry = map[string]Bridge{}
var registryLock sync.RWMutex

func Register(def *model.BridgeDefinition) (err error) {
	registryLock.Lock()
	defer registryLock.Unlock()

	var br Bridge
	switch def.Type {
	case model.BRIDGE_SQLITE:
		var b SqlBridge = sqlite3.New(def.Name, def.Path)
		br = b
	case model.BRIDGE_POSTGRES:
		var b SqlBridge = postgres.New(def.Name, def.Path)
		br = b
	case model.BRIDGE_MYSQL:
		var b SqlBridge = mysql.New(def.Name, def.Path)
		br = b
	case model.BRIDGE_MSSQL:
		var b SqlBridge = mssql.New(def.Name, def.Path)
		br = b
	case model.BRIDGE_MQTT:
		var b *MqttBridge = NewMqttBridge(def.Name, def.Path)
		br = b
	case model.BRIDGE_NATS:
		var b *NatsBridge = NewNatsBridge(def.Name, def.Path)
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
		return fmt.Errorf("unsupported bridge type %s", def.Type)
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

func GetMqttBridge(name string) (*MqttBridge, error) {
	br, err := GetBridge(name)
	if err != nil {
		return nil, err
	}

	if mqttBr, ok := br.(*MqttBridge); ok {
		return mqttBr, nil
	} else {
		return nil, fmt.Errorf("'%s' is not a MqttBridge", name)
	}
}
