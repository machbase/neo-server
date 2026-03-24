package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/stretchr/testify/require"
)

func TestServiceString(t *testing.T) {
	svc := &Service{
		Config: Config{
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
	base := Config{
		Name:        "svc-a",
		Enable:      true,
		AutoStart:   true,
		WorkingDir:  "/work",
		Environment: map[string]string{"A": "1", "B": "2"},
		Executable:  "node",
		Args:        []string{"app.js", "--port", "8080"},
	}

	same := Config{
		Name:        "svc-a",
		Enable:      true,
		AutoStart:   true,
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

func TestServiceCtlStartStopNotFound(t *testing.T) {
	ctl := &Controller{services: map[string]*Service{}}

	sc := &Config{Name: "missing"}
	ctl.startService(sc)
	if sc.StartError == nil || !strings.Contains(sc.StartError.Error(), "not found") {
		t.Fatalf("startService() expected not found error, got %v", sc.StartError)
	}

	ctl.stopService(sc)
	if sc.StopError == nil || !strings.Contains(sc.StopError.Error(), "not found") {
		t.Fatalf("stopService() expected not found error, got %v", sc.StopError)
	}
}

func TestServiceCtlStopCallsStop(t *testing.T) {
	ctl := &Controller{
		services: map[string]*Service{
			"svc-a": {
				Config: Config{Name: "svc-a"},
				Status: ServiceStatusRunning,
			},
		},
	}

	ctl.Stop(nil)

	if ctl.services["svc-a"].Config.StopError != nil {
		t.Fatalf("Stop() unexpected error: %v", ctl.services["svc-a"].Config.StopError)
	}
	if ctl.services["svc-a"].Status != ServiceStatusStopped {
		t.Fatalf("Stop() status=%s, want %s", ctl.services["svc-a"].Status, ServiceStatusStopped)
	}
}

func TestServiceCtlUpdate(t *testing.T) {
	ctl := &Controller{
		services: map[string]*Service{
			"old": {
				Config: Config{Name: "old"},
				Status: ServiceStatusRunning,
			},
			"upd": {
				Config: Config{Name: "upd"},
				Status: ServiceStatusRunning,
			},
		},
	}

	confErr := errors.New("invalid json")
	ctl.reread = &ServiceList{
		Removed: []*Config{{Name: "old"}},
		Updated: []*Config{{Name: "upd", Enable: true}},
		Added:   []*Config{{Name: "add", Enable: true}},
		Errored: []*Config{{Name: "broken", ReadError: confErr}},
	}

	actions := make([]string, 0)
	errs := make(map[string]error)
	ctl.Update(func(sc *Config, action string, err error) {
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

	if _, exists := ctl.services["old"]; exists {
		t.Fatal("Update() expected removed service to be deleted from map")
	}
	if _, exists := ctl.services["upd"]; !exists {
		t.Fatal("Update() expected updated service to remain in map")
	}
	if !ctl.services["upd"].Config.Enable {
		t.Fatal("Update() expected updated service config to be applied")
	}
	if ctl.services["upd"].Status != ServiceStatusRunning {
		t.Fatalf("updated service status=%s, want %s", ctl.services["upd"].Status, ServiceStatusRunning)
	}
	addSvc, exists := ctl.services["add"]
	if !exists {
		t.Fatal("Update() expected added service to exist in map")
	}
	if addSvc.Status != ServiceStatusRunning {
		t.Fatalf("added service status=%s, want %s", addSvc.Status, ServiceStatusRunning)
	}
}

func TestServiceAppendOutputKeepsLastLines(t *testing.T) {
	svc := &Service{}
	for i := 0; i < serviceOutputMaxLines+5; i++ {
		svc.appendOutput(fmt.Sprintf("line-%d", i))
	}

	got := svc.outputSnapshot()
	if len(got) != serviceOutputMaxLines {
		t.Fatalf("output len=%d, want %d", len(got), serviceOutputMaxLines)
	}
	if got[0] != "line-5" {
		t.Fatalf("first output=%q, want %q", got[0], "line-5")
	}
	if got[len(got)-1] != fmt.Sprintf("line-%d", serviceOutputMaxLines+4) {
		t.Fatalf("last output=%q, want %q", got[len(got)-1], fmt.Sprintf("line-%d", serviceOutputMaxLines+4))
	}
}

func TestServiceOutputWriterFlushesTrailingLine(t *testing.T) {
	svc := &Service{}
	writer := newServiceOutputWriter(svc)

	if _, err := writer.Write([]byte("stdout-1\nstderr-1\r\npartial")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	writer.Flush()

	got := svc.outputSnapshot()
	want := []string{"stdout-1", "stderr-1", "partial"}
	if len(got) != len(want) {
		t.Fatalf("output len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("output[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestServices(t *testing.T) {
	// build jsh binary for testing
	var jshBinPath string
	tmpDir := os.TempDir()
	jshBinPath = filepath.Join(tmpDir, "jsh")
	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		jshBinPath = jshBinPath + ".exe"
	}
	args = append(args, jshBinPath)
	args = append(args, "..")
	cmd := exec.Command("go", args...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build jsh binary for tests: %v", err)
	}

	// create ServiceCtl with test config
	ctl, err := NewController(&ControllerConfig{
		Launcher: []string{
			jshBinPath,
			"-v", "/work=./test",
		},
		ConfigDir: "/work/services",
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS("./test")},
		},
	})
	if err != nil {
		t.Fatalf("NewServiceCtl() error: %v", err)
	}

	// start services
	if err = ctl.Read(); err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// update services with callback to log actions and errors
	ctl.Update(func(sc *Config, s string, err error) {
		if err != nil {
			t.Logf("Service %s %s error: %v", sc.Name, s, err)
		}
	})

	// wait for service to stop
	for {
		s := ctl.StatusOf("hello")
		if s.Status == ServiceStatusStopped {
			lines := s.outputSnapshot()
			require.Equal(t, []string{
				fmt.Sprintf("Hello 0: %s 1: hello.js 2: World", jshBinPath),
				"Environment variables:",
				"ENV_VAR1=value1",
				"ENV_VAR2=value2",
			}, lines)
			break
		} else {
			time.Sleep(1 * time.Millisecond)
		}
	}
}
