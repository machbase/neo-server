package timer

import (
	"context"
	"sync"

	"github.com/machbase/neo-server/v8/mods/model"
)

var registry = struct {
	sync.RWMutex
	entries map[int64]*TimerEntry
}{entries: map[int64]*TimerEntry{}}

func GetEntry(id int64) *TimerEntry {
	registry.RLock()
	defer registry.RUnlock()
	return registry.entries[id]
}

func (service *Service) Register(definition *model.TimerDefinition) error {
	if definition.Disabled {
		Unregister(definition.Id)
		return nil
	}
	entry, err := NewTimerEntry(service, definition)
	if err != nil {
		service.recordRuntimeError(definition.Id, err)
		return err
	}
	if _, err := service.tqlLoader.Load(definition.Task); err != nil {
		entry.SetState(FAILED)
		if old := service.replace(definition.Id, entry); old != nil {
			if stopErr := old.Stop(); stopErr != nil {
				service.log.Warn("failed to stop replaced timer", definition.Id, stopErr)
			}
		}
		service.recordRuntimeError(definition.Id, err)
		return err
	}
	old := service.replace(definition.Id, entry)
	wasRunning := old != nil && old.Status() == RUNNING
	if wasRunning {
		if err := old.Stop(); err != nil {
			service.recordRuntimeError(definition.Id, err)
			return err
		}
	}
	if (old == nil && entry.AutoStart()) || wasRunning {
		if err := entry.Start(); err != nil {
			service.recordRuntimeError(definition.Id, err)
			return err
		}
	}
	return nil
}

func (service *Service) replace(id int64, entry *TimerEntry) *TimerEntry {
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
	registry.entries = map[int64]*TimerEntry{}
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
	if err := service.models.SetTimerRuntimeError(context.Background(), id, message); err != nil {
		service.log.Warn("failed to record timer error", id, err)
	}
}
