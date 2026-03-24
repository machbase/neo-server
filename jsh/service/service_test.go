package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

func TestControllerStopServiceReturnsUpdatedStatus(t *testing.T) {
	ctl := &Controller{
		services: map[string]*Service{
			"svc-a": {
				Config: Config{Name: "svc-a", Enable: true, Executable: "echo"},
				Status: ServiceStatusRunning,
			},
		},
	}

	svc, err := ctl.StopService("svc-a")
	if err != nil {
		t.Fatalf("StopService() error: %v", err)
	}
	if svc == nil {
		t.Fatal("StopService() returned nil service")
	}
	if svc.Status != ServiceStatusStopped {
		t.Fatalf("StopService() status=%s, want %s", svc.Status, ServiceStatusStopped)
	}
	if ctl.services["svc-a"].Status != ServiceStatusStopped {
		t.Fatalf("stored service status=%s, want %s", ctl.services["svc-a"].Status, ServiceStatusStopped)
	}
	if svc.Config.StopError != nil {
		t.Fatalf("StopService() unexpected StopError: %v", svc.Config.StopError)
	}
}

func TestControllerStartServiceReturnsUpdatedStatus(t *testing.T) {
	ctl := &Controller{
		services: map[string]*Service{
			"svc-a": {
				Config: Config{Name: "svc-a", Enable: true, Executable: "echo"},
				Status: ServiceStatusStopped,
			},
		},
	}

	svc, err := ctl.StartService("svc-a")
	if err != nil {
		t.Fatalf("StartService() error: %v", err)
	}
	if svc == nil {
		t.Fatal("StartService() returned nil service")
	}
	if svc.Status != ServiceStatusRunning {
		t.Fatalf("StartService() status=%s, want %s", svc.Status, ServiceStatusRunning)
	}
	if ctl.services["svc-a"].Status != ServiceStatusRunning {
		t.Fatalf("stored service status=%s, want %s", ctl.services["svc-a"].Status, ServiceStatusRunning)
	}
	if svc.Config.StartError != nil {
		t.Fatalf("StartService() unexpected StartError: %v", svc.Config.StartError)
	}
	if svc.ExitCode != 0 {
		t.Fatalf("StartService() exitCode=%d, want 0", svc.ExitCode)
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

func TestControllerJSONRPC(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeConfig := func(sc Config) {
		data, err := json.MarshalIndent(sc, "", "  ")
		if err != nil {
			t.Fatalf("MarshalIndent() error: %v", err)
		}
		path := filepath.Join(servicesDir, sc.Name+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", path, err)
		}
	}

	writeConfig(Config{Name: "alpha", Enable: false, Executable: "echo"})

	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(tmpDir)},
		},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	if err := ctl.Start(nil); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	port := ctl.Port()
	if port == 0 {
		t.Fatal("Port() = 0, want assigned random port")
	}

	var list []ServiceSnapshot
	callControllerRPC(t, port, 1, "service.list", nil, &list)
	if len(list) != 1 {
		t.Fatalf("service.list len=%d, want 1", len(list))
	}
	if list[0].Config.Name != "alpha" {
		t.Fatalf("service.list name=%q, want %q", list[0].Config.Name, "alpha")
	}
	if list[0].Status != ServiceStatusStopped {
		t.Fatalf("service.list status=%s, want %s", list[0].Status, ServiceStatusStopped)
	}

	var beta ServiceSnapshot
	callControllerRPC(t, port, 2, "service.install", Config{Name: "beta", Enable: true, Executable: "echo"}, &beta)
	if beta.Config.Name != "beta" {
		t.Fatalf("service.install name=%q, want %q", beta.Config.Name, "beta")
	}
	if beta.Status != ServiceStatusStopped {
		t.Fatalf("service.install status=%s, want %s", beta.Status, ServiceStatusStopped)
	}

	callControllerRPC(t, port, 3, "service.start", map[string]any{"name": "beta"}, &beta)
	if beta.Status != ServiceStatusRunning {
		t.Fatalf("service.start status=%s, want %s", beta.Status, ServiceStatusRunning)
	}

	writeConfig(Config{Name: "beta", Enable: false, Executable: "echo", Args: []string{"v2"}})

	var reread ServiceListSnapshot
	callControllerRPC(t, port, 4, "service.read", nil, &reread)
	if len(reread.Updated) != 1 {
		t.Fatalf("service.read updated len=%d, want 1", len(reread.Updated))
	}
	if reread.Updated[0].Name != "beta" {
		t.Fatalf("service.read updated name=%q, want %q", reread.Updated[0].Name, "beta")
	}

	var updateResult ControllerUpdateResult
	callControllerRPC(t, port, 5, "service.update", nil, &updateResult)
	if len(updateResult.Actions) != 1 {
		t.Fatalf("service.update actions len=%d, want 1", len(updateResult.Actions))
	}
	if updateResult.Actions[0].Action != "UPDATE stop" {
		t.Fatalf("service.update first action=%q, want %q", updateResult.Actions[0].Action, "UPDATE stop")
	}

	var betaAfter ServiceSnapshot
	callControllerRPC(t, port, 6, "service.get", map[string]any{"name": "beta"}, &betaAfter)
	if betaAfter.Config.Enable {
		t.Fatal("service.get enable=true, want false")
	}
	if betaAfter.Status != ServiceStatusStopped {
		t.Fatalf("service.get status=%s, want %s", betaAfter.Status, ServiceStatusStopped)
	}
	if len(betaAfter.Config.Args) != 1 || betaAfter.Config.Args[0] != "v2" {
		t.Fatalf("service.get args=%v, want [v2]", betaAfter.Config.Args)
	}

	var removed bool
	callControllerRPC(t, port, 7, "service.uninstall", map[string]any{"name": "beta"}, &removed)
	if !removed {
		t.Fatal("service.uninstall result=false, want true")
	}

	oldPort := port
	ctl.Stop(nil)
	if ctl.Port() != 0 {
		t.Fatalf("Port() after Stop() = %d, want 0", ctl.Port())
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", oldPort), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("DialTimeout() succeeded after Stop(), want listener closed")
	}
}

func callControllerRPC(t *testing.T, port int, id int, method string, params any, out any) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("DialTimeout() error: %v", err)
	}
	defer conn.Close()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var resp struct {
		Version string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("Decode() error: %v", err)
	}
	if resp.Version != "2.0" {
		t.Fatalf("jsonrpc version=%q, want %q", resp.Version, "2.0")
	}
	if resp.Error != nil {
		t.Fatalf("rpc error=%d %s", resp.Error.Code, resp.Error.Message)
	}
	if out == nil {
		return
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		t.Fatalf("Unmarshal(result) error: %v (payload=%s)", err, string(resp.Result))
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
