package bridge

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/v8/mods/bridge/connector"
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
		br = connector.NewSqliteBridge(def.Name, def.Path)
	case model.BRIDGE_POSTGRES:
		br = connector.NewPostgresBridge(def.Name, def.Path)
	case model.BRIDGE_MYSQL:
		br = connector.NewMySQLBridge(def.Name, def.Path)
	case model.BRIDGE_MSSQL:
		br = connector.NewMSSQLBridge(def.Name, def.Path)
	case model.BRIDGE_MQTT:
		br = NewMqttBridge(def.Name, def.Path)
	case model.BRIDGE_NATS:
		br = NewNatsBridge(def.Name, def.Path)
	default:
		return fmt.Errorf("undefined bridge type %s, unable to register", def.Type)
	}

	if err = br.BeforeRegister(); err != nil {
		return err
	}
	registry[def.Name] = br
	if sqlBridge, ok := br.(SqlBridge); ok {
		connector.SetDatabase(def.Name, sqlBridge.DB(), sqlBridge.Type(), def.Path)
	}
	return nil
}

func Unregister(name string) {
	registryLock.Lock()
	defer registryLock.Unlock()

	if c, ok := registry[name]; ok {
		delete(registry, name)
		if _, ok := c.(SqlBridge); ok {
			connector.UnsetDatabase(name)
		}
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
