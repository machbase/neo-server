package booter

import (
	"fmt"
	"sync"
)

type Boot interface {
	Start() error
	Stop()
}

type BootFactory struct {
	Id          string
	NewConfig   func() any
	NewInstance func(config any) (Boot, error)
}

var factoryRegistry = make(map[string]*BootFactory)
var factoryRegistryLock sync.Mutex

func RegisterBootFactory(def *BootFactory) {
	factoryRegistryLock.Lock()
	if _, exists := factoryRegistry[def.Id]; !exists {
		factoryRegistry[def.Id] = def
	}
	factoryRegistryLock.Unlock()
}

func UnregisterBootFactory(moduleId string) {
	delete(factoryRegistry, moduleId)
}

func getFactory(moduleId string) *BootFactory {
	if obj, ok := factoryRegistry[moduleId]; ok {
		return obj
	}
	return nil
}

func Register[T any](moduleId string, configFactory func() T, factory func(conf T) (Boot, error)) {
	RegisterBootFactory(&BootFactory{
		Id: moduleId,
		NewConfig: func() any {
			return configFactory()
		},
		NewInstance: func(conf any) (Boot, error) {
			if c, ok := conf.(T); ok {
				return factory(c)
			} else {
				return nil, fmt.Errorf("invalid config type: %T", conf)
			}
		},
	})
}
