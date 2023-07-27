package scheduler

import (
	"errors"
	"strings"
	"sync"
)

type Define struct {
	Name      string `json:"-"`
	Type      Type   `json:"type"`
	AutoStart bool   `json:"autoStart"`
	Task      string `json:"task"`

	// periodic task
	Schedule string `json:"schedule,omitempty"`
	// listener task
	Bridge string `json:"bridge,omitempty"`
	Topic  string `json:"topic,omitempty"`
	QoS    int    `json:"qos,omitempty"`
}

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

type Type string

const (
	UNDEFINED Type = ""
	TIMER     Type = "timer"
	LISTENER  Type = "listener"
)

func (typ Type) String() string {
	switch typ {
	default:
		return "UNDEFINED"
	case TIMER:
		return "TIMER"
	case LISTENER:
		return "LISTENER"
	}
}

func ParseType(typ string) Type {
	switch strings.ToUpper(typ) {
	default:
		return UNDEFINED
	case "TIMER":
		return TIMER
	case "LISTENER":
		return LISTENER
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

func Register(s *svr, def *Define) error {
	registryLock.Lock()
	defer registryLock.Unlock()

	var ent Entry
	var err error
	switch def.Type {
	case TIMER:
		ent, err = NewTimerEntry(s, def)
	case LISTENER:
		ent, err = NewListenerEntry(s, def)
	default:
		err = errors.New("undefined schedule type")
	}
	if err != nil {
		return err
	}

	name := strings.ToUpper(ent.Name())
	registry[name] = ent

	if ent.AutoStart() {
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
