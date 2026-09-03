package trigger

import (
	"errors"
	"sync"
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

func (state State) String() string {
	switch state {
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
	default:
		return "UNKNOWN"
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

func (entry *BaseEntry) Name() string { return entry.name }
func (entry *BaseEntry) Start() error { return errors.New("Start() is not implemented") }
func (entry *BaseEntry) Stop() error  { return errors.New("Stop() is not implemented") }
func (entry *BaseEntry) Status() State {
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.state
}
func (entry *BaseEntry) AutoStart() bool { return entry.autoStart }
func (entry *BaseEntry) Error() error {
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.err
}
func (entry *BaseEntry) SetState(state State) {
	entry.mu.Lock()
	entry.state = state
	entry.mu.Unlock()
}
func (entry *BaseEntry) SetError(err error) {
	entry.mu.Lock()
	entry.err = err
	entry.mu.Unlock()
}
func (entry *BaseEntry) SetStateError(state State, err error) {
	entry.mu.Lock()
	entry.state, entry.err = state, err
	entry.mu.Unlock()
}
func (entry *BaseEntry) StatusError() (State, error) {
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.state, entry.err
}
