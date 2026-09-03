package timer

import "github.com/machbase/neo-server/v8/mods/trigger"

type State = trigger.State
type Entry = trigger.Entry

const (
	UNKNOWN  = trigger.UNKNOWN
	FAILED   = trigger.FAILED
	STOP     = trigger.STOP
	STOPPING = trigger.STOPPING
	STARTING = trigger.STARTING
	RUNNING  = trigger.RUNNING
)

// BaseEntry preserves the existing timer implementation while delegating all
// state synchronization to the shared trigger runtime.
type BaseEntry struct {
	trigger.BaseEntry
	name string
}

func NewBaseEntry(name string, state State, autoStart bool) BaseEntry {
	return BaseEntry{BaseEntry: trigger.NewBaseEntry(name, state, autoStart), name: name}
}

func (entry *BaseEntry) setState(state State) { entry.SetState(state) }
func (entry *BaseEntry) setError(err error)   { entry.SetError(err) }
func (entry *BaseEntry) setStateError(state State, err error) {
	entry.SetStateError(state, err)
}
func (entry *BaseEntry) statusError() (State, error) { return entry.StatusError() }
