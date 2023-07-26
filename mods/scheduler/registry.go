package scheduler

import "sync"

type Define struct {
	Id        string `json:"-"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	AutoStart bool   `json:"autoStart"`
	Task      string `json:"task"`

	// periodic task
	Schedule string `json:"schedule,omitempty"`
	// listener task
	Bridge string `json:"bridge,omitempty"`
	Topic  string `json:"topic,omitempty"`
	QoS    int    `json:"qos,omitempty"`
}

type Entry interface {
	Start(*svr) error
	Stop(*svr) error
	Status() State
	AutoStart() bool
}

type State int

const (
	UNKNOWN State = iota
	STOP
	STOPPING
	STARTING
	RUNNING
)

func (st State) String() string {
	switch st {
	default:
		return "UNKNOWN"
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

type BaseEntry struct {
	state State
}

func (e *BaseEntry) Start(*svr) error {
	return nil
}

func (e *BaseEntry) Stop(*svr) error {
	return nil
}

func (e *BaseEntry) Status() State {
	return e.state
}

func (e *BaseEntry) AutoStart() bool {
	return false
}

var registry = map[string]Entry{}
var registryLock sync.RWMutex

func Register(def *Define) (err error) {
	registryLock.Lock()
	defer registryLock.Unlock()
	return
}

func Unregister(id string) {
	registryLock.Lock()
	defer registryLock.Unlock()
}

func UnregisterAll() {
	for id := range registry {
		Unregister(id)
	}
}
