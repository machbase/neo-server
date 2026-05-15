package scheduler

import (
	"errors"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/model"
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
	mu        sync.RWMutex
	name      string
	state     State
	autoStart bool
	err       error
}

func NewBaseEntry(name string, state State, autoStart bool) BaseEntry {
	return BaseEntry{name: name, state: state, autoStart: autoStart}
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
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

func (e *BaseEntry) AutoStart() bool {
	return e.autoStart
}

func (e *BaseEntry) Error() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.err
}

func (e *BaseEntry) setState(state State) {
	e.mu.Lock()
	e.state = state
	e.mu.Unlock()
}

func (e *BaseEntry) setError(err error) {
	e.mu.Lock()
	e.err = err
	e.mu.Unlock()
}

func (e *BaseEntry) setStateError(state State, err error) {
	e.mu.Lock()
	e.state = state
	e.err = err
	e.mu.Unlock()
}

func (e *BaseEntry) statusError() (State, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state, e.err
}

var registry = map[string]Entry{}
var registryLock sync.RWMutex

func Register(s *Service, def *model.ScheduleDefinition) error {
	registryLock.Lock()
	defer registryLock.Unlock()

	var initRegister bool = false
	var stateRunning bool = false
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
				stateRunning = true
			}
		} else {
			initRegister = true
		}
		ent, err = NewTimerEntry(s, def)
	case model.SCHEDULE_SUBSCRIBER:
		if _, ok := registry[strings.ToUpper(def.Name)]; !ok {
			initRegister = true
		}
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
			be.setState(FAILED)
			return err
		}
		be.setState(prevState)
	}

	if initRegister {
		if !ent.AutoStart() {
			return nil
		}
		if err := ent.Start(); err != nil {
			s.log.Warnf("schedule '%s' autostart failed, %s", ent.Name(), err.Error())
		}
		return nil
	}

	if stateRunning {
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
