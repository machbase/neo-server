package trigger

import (
	"errors"
	"testing"
)

func TestStateAndBaseEntry(t *testing.T) {
	for state, want := range map[State]string{UNKNOWN: "UNKNOWN", FAILED: "FAILED", STOP: "STOP", STOPPING: "STOPPING", STARTING: "STARTING", RUNNING: "RUNNING", State(99): "UNKNOWN"} {
		if got := state.String(); got != want {
			t.Fatalf("String(%d) = %q, want %q", state, got, want)
		}
	}

	entry := NewBaseEntry("entry", STARTING, true)
	if entry.Name() != "entry" || !entry.AutoStart() || entry.Status() != STARTING {
		t.Fatal("constructor values were not preserved")
	}
	if entry.Start() == nil || entry.Stop() == nil {
		t.Fatal("base operations must report they are unimplemented")
	}
	err := errors.New("failed")
	entry.SetState(FAILED)
	entry.SetError(err)
	if entry.Status() != FAILED || !errors.Is(entry.Error(), err) {
		t.Fatal("individual setters did not update entry")
	}
	entry.SetStateError(RUNNING, nil)
	state, gotErr := entry.StatusError()
	if state != RUNNING || gotErr != nil {
		t.Fatalf("StatusError() = %s, %v", state, gotErr)
	}
}
