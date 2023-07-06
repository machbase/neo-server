package connector

import (
	"fmt"
	"sync"
)

var registry = map[string]*Registry{}

var sharedConnectors = map[string]Connector{}
var sharedConnectorsLock sync.Mutex

func Register(def *Define) error {
	reg := &Registry{
		define: def,
	}
	switch reg.define.Type {
	case SQLITE3:
		reg.factoryFn = sqlite3Connector
	default:
		return fmt.Errorf("undefined connector type %s", reg.define.Type)
	}
	registry[def.Name] = reg
	return nil
}

func NewConnector(name string) (Connector, error) {
	reg, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("undefined connector '%s'", name)
	}
	return reg.factoryFn(reg.define)
}

func SharedConnector(name string) (Connector, error) {
	if c, ok := sharedConnectors[name]; ok {
		return c, nil
	}

	sharedConnectorsLock.Lock()
	defer sharedConnectorsLock.Unlock()

	c, err := NewConnector(name)
	if err != nil {
		return nil, err
	}
	sharedConnectors[name] = c
	return c, nil
}
