package scheduler

import (
	"errors"
	"strings"
	"sync"

	"github.com/machbase/neo-server/mods/model"
)

type State int

const (
	UNKNOWN State = iota
	FAILED
	STOP
	STOPPING
	STARTING
	RUNNING
)

func (st State) String() string {
	switch st {
	default:
		return "UNKNOWN"
	case FAILED:
		return "FAILED"
	case STOP:
		return "STOP"
	case STOPPING:
		return "STOPPING"
	case STARTING:
		return "STARTING"
	case RUNNING:
		return "RUNNING"
	}
}

type Entry interface {
	Name() string
	Start() error
	Stop() error
	Status() State
	AutoStart() bool
	Error() error
}

type BaseEntry struct {
	name      string
	state     State
	autoStart bool
	err       error
}

func (e *BaseEntry) Name() string {
	return e.name
}

func (e *BaseEntry) Start() error {
	return errors.New("Start() is not implemented")
}

func (e *BaseEntry) Stop() error {
	return errors.New("Stop() is not implemented")
}

func (e *BaseEntry) Status() State {
	return e.state
}

func (e *BaseEntry) AutoStart() bool {
	return e.autoStart
}

func (e *BaseEntry) Error() error {
	return e.err
}

var registry = map[string]Entry{}
var registryLock sync.RWMutex

func Register(s *svr, def *model.ScheduleDefinition) error {
	registryLock.Lock()
	defer registryLock.Unlock()

	var forceAutoStart bool = false
	var canAutoStart bool = true
	var ent Entry
	var err error
	switch def.Type {
	case model.SCHEDULE_TIMER:
		if ent, ok := registry[strings.ToUpper(def.Name)]; ok {
			status := ent.Status()
			if status == RUNNING {
				if err := ent.Stop(); err != nil {
					return err
				}
				forceAutoStart = true
			} else {
				canAutoStart = false
			}
		}
		ent, err = NewTimerEntry(s, def)
	case model.SCHEDULE_SUBSCRIBER:
		ent, err = NewSubscriberEntry(s, def)
	default:
		err = errors.New("undefined schedule type")
	}
	if err != nil {
		return err
	}
	name := strings.ToUpper(ent.Name())
	registry[name] = ent

	if be, ok := ent.(*TimerEntry); ok {
		prevState := ent.Status()
		if _, err := s.tqlLoader.Load(def.Task); err != nil {
			if be, ok := ent.(*TimerEntry); ok {
				be.state = FAILED
			}
			return err
		}
		be.state = prevState
	}

	if ent.AutoStart() && canAutoStart || forceAutoStart {
		if err := ent.Start(); err != nil {
			s.log.Warnf("schedule '%s' autostart failed, %s", ent.Name(), err.Error())
		}
	}

	return nil
}

func Unregister(name string) {
	registryLock.Lock()
	defer registryLock.Unlock()

	name = strings.ToUpper(name)
	if ent, ok := registry[name]; ok {
		ent.Stop()
		delete(registry, name)
	}
}

func UnregisterAll() {
	for name := range registry {
		Unregister(name)
	}
}

func GetEntry(name string) Entry {
	registryLock.RLock()
	defer registryLock.RUnlock()
	name = strings.ToUpper(name)
	if ent, ok := registry[name]; ok {
		return ent
	} else {
		return nil
	}
}
