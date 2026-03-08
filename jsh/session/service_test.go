package session

import (
	"errors"
	"strings"
	"testing"
)

func resetServicesForTest(t *testing.T, services map[string]*Service) {
	t.Helper()
	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()
	jshServices = services
}

func TestServiceString(t *testing.T) {
	svc := &Service{
		Config: ServiceConfig{
			Name:       "svc-a",
			Enable:     true,
			Executable: "node",
			Args:       []string{"app.js", "--port", "8080"},
		},
		Status: ServiceStatusRunning,
	}

	out := svc.String()

	checks := []string{
		"[svc-a] ENABLED",
		"status: running",
		"start: node [ app.js, --port, 8080 ]",
	}
	for _, chk := range checks {
		if !strings.Contains(out, chk) {
			t.Fatalf("String() output missing %q, got %q", chk, out)
		}
	}
}

func TestServiceConfigEqual(t *testing.T) {
	base := ServiceConfig{
		Name:        "svc-a",
		Enable:      true,
		AutoStart:   true,
		AutoRestart: true,
		WorkingDir:  "/work",
		Environment: map[string]string{"A": "1", "B": "2"},
		Executable:  "node",
		Args:        []string{"app.js", "--port", "8080"},
	}

	same := ServiceConfig{
		Name:        "svc-a",
		Enable:      true,
		AutoStart:   true,
		AutoRestart: true,
		WorkingDir:  "/work",
		Environment: map[string]string{"B": "2", "A": "1"},
		Executable:  "node",
		Args:        []string{"app.js", "--port", "8080"},
	}

	if !base.Equal(same) {
		t.Fatal("Equal() expected true for identical configs")
	}

	modified := same
	modified.Args = []string{"app.js", "--port", "9090"}
	if base.Equal(modified) {
		t.Fatal("Equal() expected false for different args")
	}
}

func TestServiceConfigStartStopNotFound(t *testing.T) {
	resetServicesForTest(t, map[string]*Service{})

	sc := &ServiceConfig{Name: "missing"}
	sc.Start()
	if sc.StartError == nil || !strings.Contains(sc.StartError.Error(), "not found") {
		t.Fatalf("Start() expected not found error, got %v", sc.StartError)
	}

	sc.Stop()
	if sc.StopError == nil || !strings.Contains(sc.StopError.Error(), "not found") {
		t.Fatalf("Stop() expected not found error, got %v", sc.StopError)
	}
}

func TestStopServicesCallsStop(t *testing.T) {
	resetServicesForTest(t, map[string]*Service{
		"svc-a": {
			Config: ServiceConfig{Name: "other-name"},
		},
	})

	StopServices(nil)

	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()
	if jshServices["svc-a"].Config.StopError == nil {
		t.Fatal("StopServices() expected StopError to be set by Config.Stop()")
	}
}

func TestServiceListUpdate(t *testing.T) {
	resetServicesForTest(t, map[string]*Service{
		"old": {
			Config: ServiceConfig{Name: "old"},
			Status: ServiceStatusRunning,
		},
		"upd": {
			Config: ServiceConfig{Name: "upd"},
			Status: ServiceStatusRunning,
		},
	})

	confErr := errors.New("invalid json")
	result := ServiceList{
		Removed: []*ServiceConfig{{Name: "old"}},
		Updated: []*ServiceConfig{{Name: "upd", Enable: true}},
		Added:   []*ServiceConfig{{Name: "add", Enable: true}},
		Errored: []*ServiceConfig{{Name: "broken", ReadError: confErr}},
	}

	actions := make([]string, 0)
	errs := make(map[string]error)
	result.Update(func(sc *ServiceConfig, action string, err error) {
		actions = append(actions, sc.Name+":"+action)
		errs[sc.Name+":"+action] = err
	})

	wantActions := []string{
		"old:REMOVE stop",
		"upd:UPDATE stop",
		"upd:UPDATE start",
		"add:ADD start",
		"broken:CONF",
	}
	if len(actions) != len(wantActions) {
		t.Fatalf("Update() callback count=%d, want %d (%v)", len(actions), len(wantActions), actions)
	}
	for i, want := range wantActions {
		if actions[i] != want {
			t.Fatalf("Update() action[%d]=%q, want %q", i, actions[i], want)
		}
	}

	if errs["old:REMOVE stop"] != nil {
		t.Fatalf("REMOVE stop error=%v, want nil", errs["old:REMOVE stop"])
	}
	if errs["upd:UPDATE stop"] != nil {
		t.Fatalf("UPDATE stop error=%v, want nil", errs["upd:UPDATE stop"])
	}
	if errs["upd:UPDATE start"] != nil {
		t.Fatalf("UPDATE start error=%v, want nil", errs["upd:UPDATE start"])
	}
	if errs["add:ADD start"] != nil {
		t.Fatalf("ADD start error=%v, want nil", errs["add:ADD start"])
	}
	if !errors.Is(errs["broken:CONF"], confErr) {
		t.Fatalf("CONF error=%v, want %v", errs["broken:CONF"], confErr)
	}

	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()
	if _, exists := jshServices["old"]; exists {
		t.Fatal("Update() expected removed service to be deleted from map")
	}
	if _, exists := jshServices["upd"]; !exists {
		t.Fatal("Update() expected updated service to remain in map")
	}
	addSvc, exists := jshServices["add"]
	if !exists {
		t.Fatal("Update() expected added service to exist in map")
	}
	if addSvc.Status != ServiceStatusStopped {
		t.Fatalf("added service status=%s, want %s", addSvc.Status, ServiceStatusStopped)
	}
}
