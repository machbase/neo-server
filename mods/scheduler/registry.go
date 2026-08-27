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

// RegisterTimer registers (or re-registers) a timer entry in the runtime
// registry, restarting it if it was previously running.
func RegisterTimer(s *Service, def *model.TimerDefinition) error {
	registryLock.Lock()
	defer registryLock.Unlock()

	name := strings.ToUpper(def.Name)
	var initRegister, stateRunning bool
	if old, ok := registry[name]; ok {
		if old.Status() == RUNNING {
			if err := old.Stop(); err != nil {
				return err
			}
			stateRunning = true
		}
	} else {
		initRegister = true
	}

	ent, err := NewTimerEntry(s, def)
	if err != nil {
		return err
	}
	registry[name] = ent

	prevState := ent.Status()
	if _, err := s.tqlLoader.Load(def.Task); err != nil {
		ent.setState(FAILED)
		return err
	}
	ent.setState(prevState)

	return finishRegister(s, ent, initRegister, stateRunning)
}

// RegisterSubscriber registers (or re-registers) a subscriber entry in the
// runtime registry.
func RegisterSubscriber(s *Service, def *model.SubscriberDefinition) error {
	registryLock.Lock()
	defer registryLock.Unlock()

	name := strings.ToUpper(def.Name)
	var initRegister bool
	if _, ok := registry[name]; !ok {
		initRegister = true
	}

	ent, err := NewSubscriberEntry(s, def)
	if err != nil {
		return err
	}
	registry[name] = ent

	return finishRegister(s, ent, initRegister, false)
}

func finishRegister(s *Service, ent Entry, initRegister bool, stateRunning bool) error {
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
