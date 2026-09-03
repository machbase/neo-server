package subscriber

import (
	"context"
	"sync"

	"github.com/machbase/neo-server/v8/mods/model"
)

var registry = struct {
	sync.RWMutex
	entries map[int64]*SubscriberEntry
}{entries: map[int64]*SubscriberEntry{}}

func GetEntry(id int64) *SubscriberEntry {
	registry.RLock()
	defer registry.RUnlock()
	return registry.entries[id]
}
func (service *Service) Register(definition *model.SubscriberDefinition) error {
	if definition.Disabled {
		Unregister(definition.Id)
		return nil
	}
	entry, err := NewSubscriberEntry(service, definition)
	if err != nil {
		service.recordRuntimeError(definition.Id, err)
		return err
	}
	old := service.replace(definition.Id, entry)
	if old != nil && old.Status() == RUNNING {
		_ = old.Stop()
		if err := entry.Start(); err != nil {
			service.recordRuntimeError(definition.Id, err)
			return err
		}
	} else if old == nil && entry.AutoStart() {
		if err := entry.Start(); err != nil {
			service.recordRuntimeError(definition.Id, err)
			return err
		}
	}
	service.recordRuntimeError(definition.Id, nil)
	return nil
}
func (service *Service) replace(id int64, entry *SubscriberEntry) *SubscriberEntry {
	registry.Lock()
	defer registry.Unlock()
	old := registry.entries[id]
	registry.entries[id] = entry
	return old
}
func Unregister(id int64) {
	registry.Lock()
	entry := registry.entries[id]
	delete(registry.entries, id)
	registry.Unlock()
	if entry != nil {
		_ = entry.Stop()
	}
}
func unregisterAll() {
	registry.Lock()
	entries := registry.entries
	registry.entries = map[int64]*SubscriberEntry{}
	registry.Unlock()
	for _, entry := range entries {
		_ = entry.Stop()
	}
}
func (service *Service) recordRuntimeError(id int64, err error) {
	if service.models == nil {
		return
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	_ = service.models.SetSubscriberRuntimeError(context.Background(), id, message)
}
