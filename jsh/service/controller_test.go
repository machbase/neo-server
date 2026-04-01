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

type stubAddr struct {
	network string
	addr    string
}

func (a stubAddr) Network() string { return a.network }

func (a stubAddr) String() string { return a.addr }

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
		WorkingDir:  "/work",
		Environment: map[string]string{"A": "1", "B": "2"},
		Executable:  "node",
		Args:        []string{"app.js", "--port", "8080"},
	}

	same := Config{
		Name:        "svc-a",
		Enable:      true,
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

func TestServiceCtlReload(t *testing.T) {
	ctl := &Controller{
		services: map[string]*Service{
			"keep": {
				Config: Config{Name: "keep", Enable: true, Executable: "echo"},
				Status: ServiceStatusRunning,
			},
			"disable": {
				Config: Config{Name: "disable", Enable: false, Executable: "echo"},
				Status: ServiceStatusRunning,
			},
			"update": {
				Config: Config{Name: "update", Enable: true, Executable: "echo", Args: []string{"v1"}},
				Status: ServiceStatusRunning,
			},
			"remove": {
				Config: Config{Name: "remove", Enable: true, Executable: "echo"},
				Status: ServiceStatusRunning,
			},
		},
	}

	confErr := errors.New("invalid json")
	ctl.reread = &ServiceList{
		Unchanged: []*Config{{Name: "keep", Enable: true, Executable: "echo"}},
		Updated:   []*Config{{Name: "update", Enable: true, Executable: "echo", Args: []string{"v2"}}},
		Added:     []*Config{{Name: "add", Enable: true, Executable: "node"}},
		Removed:   []*Config{{Name: "remove", Enable: true, Executable: "echo"}},
		Errored:   []*Config{{Name: "broken", ReadError: confErr}},
	}

	actions := make([]string, 0)
	errMap := make(map[string]error)
	ctl.Reload(func(sc *Config, action string, err error) {
		actions = append(actions, sc.Name+":"+action)
		errMap[sc.Name+":"+action] = err
	})

	wantActions := []string{
		"disable:RELOAD stop",
		"keep:RELOAD stop",
		"remove:RELOAD stop",
		"update:RELOAD stop",
		"broken:CONF",
		"add:RELOAD start",
		"keep:RELOAD start",
		"update:RELOAD start",
	}
	if len(actions) != len(wantActions) {
		t.Fatalf("Reload() callback count=%d, want %d (%v)", len(actions), len(wantActions), actions)
	}
	for i, want := range wantActions {
		if actions[i] != want {
			t.Fatalf("Reload() action[%d]=%q, want %q", i, actions[i], want)
		}
	}
	for _, action := range wantActions[:4] {
		if errMap[action] != nil {
			t.Fatalf("%s error=%v, want nil", action, errMap[action])
		}
	}
	if !errors.Is(errMap["broken:CONF"], confErr) {
		t.Fatalf("CONF error=%v, want %v", errMap["broken:CONF"], confErr)
	}
	for _, action := range wantActions[5:] {
		if errMap[action] != nil {
			t.Fatalf("%s error=%v, want nil", action, errMap[action])
		}
	}

	if _, exists := ctl.services["remove"]; exists {
		t.Fatal("Reload() expected removed service to be deleted from map")
	}
	if ctl.services["disable"].Status != ServiceStatusStopped {
		t.Fatalf("disabled service status=%s, want %s", ctl.services["disable"].Status, ServiceStatusStopped)
	}
	if ctl.services["keep"].Status != ServiceStatusRunning {
		t.Fatalf("unchanged enabled service status=%s, want %s", ctl.services["keep"].Status, ServiceStatusRunning)
	}
	if ctl.services["update"].Status != ServiceStatusRunning {
		t.Fatalf("updated enabled service status=%s, want %s", ctl.services["update"].Status, ServiceStatusRunning)
	}
	if len(ctl.services["update"].Config.Args) != 1 || ctl.services["update"].Config.Args[0] != "v2" {
		t.Fatalf("updated service args=%v, want [v2]", ctl.services["update"].Config.Args)
	}
	if ctl.services["add"].Status != ServiceStatusRunning {
		t.Fatalf("added enabled service status=%s, want %s", ctl.services["add"].Status, ServiceStatusRunning)
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
	tests := []struct {
		name    string
		address func(string) string
	}{
		{
			name: "tcp",
			address: func(string) string {
				return ""
			},
		},
		{
			name: "unix",
			address: func(tmpDir string) string {
				return "unix://" + filepath.Join(os.TempDir(), fmt.Sprintf("neo-%d.sock", time.Now().UnixNano()))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
				Address: tc.address(tmpDir),
			})
			if err != nil {
				t.Fatalf("NewController() error: %v", err)
			}
			if err := ctl.Start(nil); err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			address := ctl.Address()
			if address == "" {
				t.Fatal("Address() = empty, want assigned random address")
			}

			var list []ServiceSnapshot
			callControllerRPC(t, address, 1, "service.list", nil, &list)
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
			callControllerRPC(t, address, 2, "service.install", Config{Name: "beta", Enable: false, Executable: "echo"}, &beta)
			if beta.Config.Name != "beta" {
				t.Fatalf("service.install name=%q, want %q", beta.Config.Name, "beta")
			}
			if beta.Status != ServiceStatusStopped {
				t.Fatalf("service.install status=%s, want %s", beta.Status, ServiceStatusStopped)
			}

			callControllerRPC(t, address, 3, "service.start", map[string]any{"name": "beta"}, &beta)
			if beta.Status != ServiceStatusRunning {
				t.Fatalf("service.start status=%s, want %s", beta.Status, ServiceStatusRunning)
			}

			var betaRuntime ServiceRuntimeSnapshot
			callControllerRPC(t, address, 31, "service.runtime.get", map[string]any{"name": "beta"}, &betaRuntime)
			if len(betaRuntime.Output) != 0 {
				t.Fatalf("service.runtime.get output=%v, want empty", betaRuntime.Output)
			}
			if betaRuntime.Details != nil {
				t.Fatalf("service.runtime.get details=%v, want nil", betaRuntime.Details)
			}

			callControllerRPC(t, address, 32, "service.runtime.detail.add", map[string]any{"name": "beta", "key": "health", "value": "ok"}, &betaRuntime)
			if betaRuntime.Details["health"] != "ok" {
				t.Fatalf("service.runtime.detail.add details=%v, want health=ok", betaRuntime.Details)
			}

			callControllerRPC(t, address, 321, "service.runtime.detail.set", map[string]any{"name": "beta", "key": "health", "value": "good"}, &betaRuntime)
			if betaRuntime.Details["health"] != "good" {
				t.Fatalf("service.runtime.detail.set details=%v, want health=good", betaRuntime.Details)
			}

			callControllerRPC(t, address, 33, "service.runtime.detail.update", map[string]any{"name": "beta", "key": "health", "value": "warn"}, &betaRuntime)
			if betaRuntime.Details["health"] != "warn" {
				t.Fatalf("service.runtime.detail.update details=%v, want health=warn", betaRuntime.Details)
			}

			callControllerRPC(t, address, 34, "service.runtime.detail.delete", map[string]any{"name": "beta", "key": "health"}, &betaRuntime)
			if betaRuntime.Details != nil {
				t.Fatalf("service.runtime.detail.delete details=%v, want nil", betaRuntime.Details)
			}

			writeConfig(Config{Name: "beta", Enable: false, Executable: "echo", Args: []string{"v2"}})

			var reread ServiceListSnapshot
			callControllerRPC(t, address, 4, "service.read", nil, &reread)
			if len(reread.Updated) != 1 {
				t.Fatalf("service.read updated len=%d, want 1", len(reread.Updated))
			}
			if reread.Updated[0].Name != "beta" {
				t.Fatalf("service.read updated name=%q, want %q", reread.Updated[0].Name, "beta")
			}

			var updateResult ControllerUpdateResult
			callControllerRPC(t, address, 5, "service.update", nil, &updateResult)
			if len(updateResult.Actions) != 1 {
				t.Fatalf("service.update actions len=%d, want 1", len(updateResult.Actions))
			}
			if updateResult.Actions[0].Action != "UPDATE stop" {
				t.Fatalf("service.update first action=%q, want %q", updateResult.Actions[0].Action, "UPDATE stop")
			}

			var betaAfter ServiceSnapshot
			callControllerRPC(t, address, 6, "service.get", map[string]any{"name": "beta"}, &betaAfter)
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
			callControllerRPC(t, address, 7, "service.uninstall", map[string]any{"name": "beta"}, &removed)
			if !removed {
				t.Fatal("service.uninstall result=false, want true")
			}

			oldAddress := address
			ctl.Stop(nil)
			if ctl.Address() != "" {
				t.Fatalf("Address() after Stop() = %s, want empty", ctl.Address())
			}
			conn, err := dialControllerRPC(oldAddress, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Fatal("DialTimeout() succeeded after Stop(), want listener closed")
			}
		})
	}
}

func callControllerRPC(t *testing.T, address string, id int, method string, params any, out any) {
	t.Helper()

	conn, err := dialControllerRPC(address, time.Second)
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

func dialControllerRPC(address string, timeout time.Duration) (net.Conn, error) {
	network, target, err := parseRPCAddress(address)
	if err != nil {
		return nil, err
	}
	return net.DialTimeout(network, target, timeout)
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

func TestServiceStringIncludesPidAndOutput(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 4242}}
	svc := &Service{
		Config: Config{
			Name:       "svc-b",
			Enable:     false,
			Executable: "runner",
			Args:       []string{"--flag"},
		},
		Status: ServiceStatusRunning,
		cmd:    cmd,
		Runtime: ServiceRuntime{
			output: []string{"line-1", "line-2"},
		},
	}

	out := svc.String()
	checks := []string{
		"[svc-b] disabled",
		"pid: 4242",
		"output:\n    line-1\n    line-2\n",
	}
	for _, chk := range checks {
		if !strings.Contains(out, chk) {
			t.Fatalf("String() output missing %q, got %q", chk, out)
		}
	}
}

func TestConfigEqualRejectsEnvironmentDifferences(t *testing.T) {
	base := Config{
		Name:        "svc-a",
		Enable:      true,
		WorkingDir:  "/work",
		Environment: map[string]string{"A": "1", "B": "2"},
		Executable:  "node",
		Args:        []string{"app.js"},
	}

	tests := []struct {
		name  string
		right Config
	}{
		{
			name: "different name",
			right: Config{
				Name:        "svc-b",
				Enable:      true,
				WorkingDir:  "/work",
				Environment: map[string]string{"A": "1", "B": "2"},
				Executable:  "node",
				Args:        []string{"app.js"},
			},
		},
		{
			name: "different environment value",
			right: Config{
				Name:        "svc-a",
				Enable:      true,
				WorkingDir:  "/work",
				Environment: map[string]string{"A": "9", "B": "2"},
				Executable:  "node",
				Args:        []string{"app.js"},
			},
		},
		{
			name: "missing environment key",
			right: Config{
				Name:        "svc-a",
				Enable:      true,
				WorkingDir:  "/work",
				Environment: map[string]string{"A": "1"},
				Executable:  "node",
				Args:        []string{"app.js"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if base.Equal(tc.right) {
				t.Fatalf("Equal() = true, want false for %s", tc.name)
			}
		})
	}
}

func TestServiceOutputWriterFlushIgnoresEmptyPending(t *testing.T) {
	svc := &Service{}
	writer := newServiceOutputWriter(svc)
	writer.Flush()
	if got := svc.outputSnapshot(); len(got) != 0 {
		t.Fatalf("output len=%d, want 0", len(got))
	}
}

func TestSnapshotServiceCopiesRuntimeFields(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 3001}}
	svc := &Service{
		Config: Config{
			Name:        "svc-a",
			Enable:      true,
			WorkingDir:  "/tmp/work",
			Environment: map[string]string{"A": "1"},
			Executable:  "node",
			Args:        []string{"app.js"},
			ReadError:   errors.New("read failed"),
			StartError:  errors.New("start failed"),
			StopError:   errors.New("stop failed"),
		},
		Status:   ServiceStatusRunning,
		ExitCode: 23,
		Error:    errors.New("runtime failed"),
		cmd:      cmd,
		Runtime: ServiceRuntime{
			output:  []string{"stdout"},
			details: map[string]any{"mode": "test"},
		},
	}

	snap := snapshotService(svc)
	if snap.Config.Name != svc.Config.Name || snap.Config.WorkingDir != svc.Config.WorkingDir {
		t.Fatalf("snapshot config mismatch: %+v", snap.Config)
	}
	if snap.Config.Environment["A"] != "1" {
		t.Fatalf("snapshot environment=%v, want map with A=1", snap.Config.Environment)
	}
	if len(snap.Config.Args) != 1 || snap.Config.Args[0] != "app.js" {
		t.Fatalf("snapshot args=%v, want [app.js]", snap.Config.Args)
	}
	if snap.Config.ReadError != "read failed" || snap.Config.StartError != "start failed" || snap.Config.StopError != "stop failed" {
		t.Fatalf("snapshot errors=%+v", snap.Config)
	}
	if snap.Error != "runtime failed" {
		t.Fatalf("snapshot error=%q, want runtime failed", snap.Error)
	}
	if snap.PID != 3001 {
		t.Fatalf("snapshot pid=%d, want 3001", snap.PID)
	}
	if len(snap.Output) != 1 || snap.Output[0] != "stdout" {
		t.Fatalf("snapshot output=%v, want [stdout]", snap.Output)
	}
	if runtimeSnap := snapshotServiceRuntime(svc); runtimeSnap.Details["mode"] != "test" {
		t.Fatalf("runtime snapshot details=%v, want mode=test", runtimeSnap.Details)
	}

	snap.Config.Environment["A"] = "9"
	snap.Config.Args[0] = "mutated"
	snap.Output[0] = "mutated"
	runtimeSnap := snapshotServiceRuntime(svc)
	runtimeSnap.Details["mode"] = "mutated"
	if svc.Config.Environment["A"] != "1" {
		t.Fatal("snapshot mutated original environment")
	}
	if svc.Config.Args[0] != "app.js" {
		t.Fatal("snapshot mutated original args")
	}
	if svc.Runtime.output[0] != "stdout" {
		t.Fatal("snapshot mutated original output")
	}
	if svc.Runtime.details["mode"] != "test" {
		t.Fatal("snapshot mutated original details")
	}
}

func TestSnapshotServiceOmitsPidWhenStopped(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 77}}
	svc := &Service{Status: ServiceStatusStopped, cmd: cmd}
	if snap := snapshotService(svc); snap.PID != 0 {
		t.Fatalf("snapshot pid=%d, want 0", snap.PID)
	}
}

func TestControllerStartReturnsReadAndRPCErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		ctl, err := NewController(&ControllerConfig{ConfigDir: "/work/missing", Mounts: []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(t.TempDir())}}})
		if err != nil {
			t.Fatalf("NewController() error: %v", err)
		}
		if err := ctl.Start(nil); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("Start() error=%v, want missing dir error", err)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		tmpDir := t.TempDir()
		servicesDir := filepath.Join(tmpDir, "services")
		if err := os.MkdirAll(servicesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		ctl, err := NewController(&ControllerConfig{
			ConfigDir: "/work/services",
			Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
			Address:   "bad://127.0.0.1:0",
		})
		if err != nil {
			t.Fatalf("NewController() error: %v", err)
		}
		if err := ctl.Start(nil); err == nil || !strings.Contains(err.Error(), "unsupported rpc address scheme") {
			t.Fatalf("Start() error=%v, want unsupported scheme error", err)
		}
	})
}

func TestControllerInstallAndUninstallErrors(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	if err := ctl.Install(&Config{Name: "bad/name"}); err == nil || !strings.Contains(err.Error(), "cannot contain '/'") {
		t.Fatalf("Install() error=%v, want invalid name error", err)
	}

	sc := &Config{Name: "svc-a", Enable: false, Executable: "echo"}
	if err := ctl.Install(sc); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if err := ctl.Install(sc); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Install() duplicate error=%v, want already exists", err)
	}
	if err := ctl.Uninstall("missing"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Uninstall() error=%v, want does not exist", err)
	}
}

func TestControllerReadClassifiesConfigsAndErrors(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	if err := os.MkdirAll(filepath.Join(servicesDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeFile := func(name string, body string) {
		path := filepath.Join(servicesDir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", path, err)
		}
	}
	marshalConfig := func(sc Config) string {
		data, err := json.Marshal(sc)
		if err != nil {
			t.Fatalf("Marshal() error: %v", err)
		}
		return string(data)
	}

	writeFile("keep.json", marshalConfig(Config{Name: "keep", Enable: true, Executable: "echo"}))
	writeFile("update.json", marshalConfig(Config{Name: "update", Enable: true, Executable: "echo", Args: []string{"v2"}}))
	writeFile("add.json", marshalConfig(Config{Name: "add", Enable: false, Executable: "echo"}))
	writeFile("broken.json", "{")
	writeFile("note.txt", "ignored")

	ctl, err := NewController(&ControllerConfig{
		ConfigDir: "/work/services",
		Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	ctl.services = map[string]*Service{
		"keep":   {Config: Config{Name: "keep", Enable: true, Executable: "echo"}},
		"update": {Config: Config{Name: "update", Enable: true, Executable: "echo", Args: []string{"v1"}}},
		"gone":   {Config: Config{Name: "gone", Enable: false, Executable: "echo"}},
	}

	if err := ctl.Read(); err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if ctl.reread == nil {
		t.Fatal("Read() left reread=nil")
	}
	if len(ctl.reread.Unchanged) != 1 || ctl.reread.Unchanged[0].Name != "keep" {
		t.Fatalf("Unchanged=%v, want [keep]", snapshotConfigs(ctl.reread.Unchanged))
	}
	if len(ctl.reread.Updated) != 1 || ctl.reread.Updated[0].Name != "update" {
		t.Fatalf("Updated=%v, want [update]", snapshotConfigs(ctl.reread.Updated))
	}
	if len(ctl.reread.Added) != 1 || ctl.reread.Added[0].Name != "add" {
		t.Fatalf("Added=%v, want [add]", snapshotConfigs(ctl.reread.Added))
	}
	if len(ctl.reread.Removed) != 1 || ctl.reread.Removed[0].Name != "gone" {
		t.Fatalf("Removed=%v, want [gone]", snapshotConfigs(ctl.reread.Removed))
	}
	if len(ctl.reread.Errored) != 1 || ctl.reread.Errored[0].Name != "broken" || ctl.reread.Errored[0].ReadError == nil {
		t.Fatalf("Errored=%v, want [broken with error]", snapshotConfigs(ctl.reread.Errored))
	}
}

func TestControllerStartAndStopServiceEdgeStates(t *testing.T) {
	t.Run("start failure marks service failed", func(t *testing.T) {
		ctl := &Controller{
			launcher: []string{"/path/that/does/not/exist"},
			services: map[string]*Service{
				"svc-a": {Config: Config{Name: "svc-a", Executable: "echo"}, Status: ServiceStatusStopped},
			},
		}

		svc, err := ctl.StartService("svc-a")
		if err == nil {
			t.Fatal("StartService() error=nil, want failure")
		}
		if svc.Status != ServiceStatusFailed {
			t.Fatalf("status=%s, want %s", svc.Status, ServiceStatusFailed)
		}
		if svc.Config.StartError == nil {
			t.Fatal("StartError=nil, want failure")
		}
	})

	t.Run("non running nil command becomes stopped", func(t *testing.T) {
		svc := &Service{Status: ServiceStatusFailed}
		sc := &Config{Name: "svc-a"}
		(&Controller{}).stopServiceInstance(svc, sc)
		if svc.Status != ServiceStatusStopped {
			t.Fatalf("status=%s, want %s", svc.Status, ServiceStatusStopped)
		}
		if sc.StopError != nil {
			t.Fatalf("StopError=%v, want nil", sc.StopError)
		}
	})

	t.Run("running nil process becomes stopped", func(t *testing.T) {
		svc := &Service{Status: ServiceStatusRunning, cmd: &exec.Cmd{}}
		sc := &Config{Name: "svc-a"}
		(&Controller{}).stopServiceInstance(svc, sc)
		if svc.Status != ServiceStatusStopped {
			t.Fatalf("status=%s, want %s", svc.Status, ServiceStatusStopped)
		}
		if svc.Error != nil || sc.StopError != nil {
			t.Fatalf("errors svc=%v config=%v, want nil", svc.Error, sc.StopError)
		}
	})
}

func TestRPCUtilityFunctionsAndDispatchErrors(t *testing.T) {
	t.Run("parse and format address", func(t *testing.T) {
		network, address, err := parseRPCAddress("unix:///tmp/neo.sock")
		if err != nil || network != "unix" || address != "/tmp/neo.sock" {
			t.Fatalf("parseRPCAddress() = %q %q %v", network, address, err)
		}
		if _, _, err := parseRPCAddress(""); err == nil {
			t.Fatal("parseRPCAddress(empty) error=nil, want error")
		}
		if _, _, err := parseRPCAddress("udp://127.0.0.1:1"); err == nil {
			t.Fatal("parseRPCAddress(udp) error=nil, want error")
		}
		if got := formatRPCAddress("unix", &net.UnixAddr{Name: "/tmp/neo.sock", Net: "unix"}); got != "unix:///tmp/neo.sock" {
			t.Fatalf("formatRPCAddress(unix)=%q", got)
		}
		if got := formatRPCAddress("tcp", nil); got != "" {
			t.Fatalf("formatRPCAddress(nil)=%q, want empty", got)
		}
		if got := formatRPCAddress("pipe", stubAddr{network: "pipe", addr: "controller"}); got != "pipe://controller" {
			t.Fatalf("formatRPCAddress(stub)=%q, want pipe://controller", got)
		}
	})

	t.Run("cleanup and decode params", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "neo.sock")
		if err := os.WriteFile(sock, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		cleanupRPCAddress("unix://" + sock)
		if _, err := os.Stat(sock); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cleanupRPCAddress() stat err=%v, want not exist", err)
		}
		var req serviceNameRequest
		if err := decodeRPCParams(nil, &req); err != nil {
			t.Fatalf("decodeRPCParams(nil) error: %v", err)
		}
		if err := decodeRPCParams(json.RawMessage("null"), &req); err != nil {
			t.Fatalf("decodeRPCParams(null) error: %v", err)
		}
		if err := decodeRPCParams(json.RawMessage("{"), &req); err == nil {
			t.Fatal("decodeRPCParams(invalid) error=nil, want error")
		}
	})

	t.Run("request and response helpers", func(t *testing.T) {
		ctl := &Controller{}
		empty := ctl.rereadSnapshot()
		if len(empty.Added) != 0 || len(empty.Updated) != 0 || len(empty.Removed) != 0 || len(empty.Unchanged) != 0 || len(empty.Errored) != 0 {
			t.Fatalf("rereadSnapshot()=%+v, want empty slices", empty)
		}

		req := controllerRPCRequest{}
		if req.hasResponse() {
			t.Fatal("hasResponse()=true, want false")
		}
		if string(req.responseID()) != "null" {
			t.Fatalf("responseID()=%s, want null", string(req.responseID()))
		}
		req.ID = json.RawMessage("1")
		if !req.hasResponse() {
			t.Fatal("hasResponse()=false, want true")
		}
		if invalidParamsError(errors.New("bad")).Code != jsonRPCInvalidParam {
			t.Fatal("invalidParamsError() returned wrong code")
		}
		if internalRPCError(errors.New("bad")).Code != jsonRPCInternal {
			t.Fatal("internalRPCError() returned wrong code")
		}
	})

	t.Run("handle rpc validation and dispatch errors", func(t *testing.T) {
		ctl := &Controller{services: map[string]*Service{}}
		resp := ctl.handleRPC(controllerRPCRequest{Version: "1.0", ID: json.RawMessage("1"), Method: "service.list"})
		if resp.Error == nil || resp.Error.Code != jsonRPCInvalidReq {
			t.Fatalf("invalid version response=%+v", resp)
		}
		resp = ctl.handleRPC(controllerRPCRequest{Version: jsonRPCVersion, ID: json.RawMessage("1")})
		if resp.Error == nil || resp.Error.Message != "method is required" {
			t.Fatalf("missing method response=%+v", resp)
		}
		if _, rpcErr := ctl.dispatchRPC("service.get", json.RawMessage("{")); rpcErr == nil || rpcErr.Code != jsonRPCInvalidParam {
			t.Fatalf("dispatchRPC invalid params=%+v", rpcErr)
		}
		if _, rpcErr := ctl.dispatchRPC("service.get", json.RawMessage(`{"name":"missing"}`)); rpcErr == nil || rpcErr.Code != jsonRPCNotFound {
			t.Fatalf("dispatchRPC not found=%+v", rpcErr)
		}
		if _, rpcErr := ctl.dispatchRPC("service.runtime.detail.add", json.RawMessage(`{"name":"missing","key":"k","value":1}`)); rpcErr == nil || rpcErr.Code != jsonRPCNotFound {
			t.Fatalf("dispatchRPC runtime add not found=%+v", rpcErr)
		}
		if _, rpcErr := ctl.dispatchRPC("service.runtime.detail.set", json.RawMessage(`{"name":"missing","key":"k","value":1}`)); rpcErr == nil || rpcErr.Code != jsonRPCNotFound {
			t.Fatalf("dispatchRPC runtime set not found=%+v", rpcErr)
		}
		if _, rpcErr := ctl.dispatchRPC("missing.method", nil); rpcErr == nil || rpcErr.Code != jsonRPCMethodMiss {
			t.Fatalf("dispatchRPC missing method=%+v", rpcErr)
		}
	})

	t.Run("dispatch rpc lifecycle success", func(t *testing.T) {
		tmpDir := t.TempDir()
		servicesDir := filepath.Join(tmpDir, "services")
		if err := os.MkdirAll(servicesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		ctl, err := NewController(&ControllerConfig{
			ConfigDir: "/work/services",
			Mounts:    []engine.FSTab{{MountPoint: "/work", FS: os.DirFS(tmpDir)}},
		})
		if err != nil {
			t.Fatalf("NewController() error: %v", err)
		}

		installParams, _ := json.Marshal(Config{Name: "svc-a", Enable: false, Executable: "echo"})
		result, rpcErr := ctl.dispatchRPC("service.install", installParams)
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(install) error=%+v", rpcErr)
		}
		if result.(ServiceSnapshot).Config.Name != "svc-a" {
			t.Fatalf("install result=%+v, want svc-a", result)
		}

		nameParams := json.RawMessage(`{"name":"svc-a"}`)
		result, rpcErr = ctl.dispatchRPC("service.start", nameParams)
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(start) error=%+v", rpcErr)
		}
		if result.(ServiceSnapshot).Status != ServiceStatusRunning {
			t.Fatalf("start result=%+v, want running", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.runtime.detail.add", json.RawMessage(`{"name":"svc-a","key":"health","value":"ok"}`))
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(runtime.detail.add) error=%+v", rpcErr)
		}
		if result.(ServiceRuntimeSnapshot).Details["health"] != "ok" {
			t.Fatalf("runtime.detail.add result=%+v, want health=ok", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.runtime.detail.set", json.RawMessage(`{"name":"svc-a","key":"health","value":"good"}`))
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(runtime.detail.set) error=%+v", rpcErr)
		}
		if result.(ServiceRuntimeSnapshot).Details["health"] != "good" {
			t.Fatalf("runtime.detail.set result=%+v, want health=good", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.runtime.detail.update", json.RawMessage(`{"name":"svc-a","key":"health","value":"warn"}`))
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(runtime.detail.update) error=%+v", rpcErr)
		}
		if result.(ServiceRuntimeSnapshot).Details["health"] != "warn" {
			t.Fatalf("runtime.detail.update result=%+v, want health=warn", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.runtime.detail.delete", json.RawMessage(`{"name":"svc-a","key":"health"}`))
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(runtime.detail.delete) error=%+v", rpcErr)
		}
		if result.(ServiceRuntimeSnapshot).Details != nil {
			t.Fatalf("runtime.detail.delete result=%+v, want nil details", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.stop", nameParams)
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(stop) error=%+v", rpcErr)
		}
		if result.(ServiceSnapshot).Status != ServiceStatusStopped {
			t.Fatalf("stop result=%+v, want stopped", result)
		}

		result, rpcErr = ctl.dispatchRPC("service.uninstall", nameParams)
		if rpcErr != nil {
			t.Fatalf("dispatchRPC(uninstall) error=%+v", rpcErr)
		}
		if removed, ok := result.(bool); !ok || !removed {
			t.Fatalf("uninstall result=%v, want true", result)
		}
	})

	t.Run("reload snapshot returns actions and services", func(t *testing.T) {
		ctl := &Controller{services: map[string]*Service{
			"keep": {Config: Config{Name: "keep", Enable: true, Executable: "echo"}, Status: ServiceStatusRunning},
		}}
		ctl.reread = &ServiceList{
			Unchanged: []*Config{{Name: "keep", Enable: true, Executable: "echo"}},
			Added:     []*Config{{Name: "add", Enable: false, Executable: "echo"}},
			Errored:   []*Config{{Name: "broken", ReadError: errors.New("broken config")}},
		}

		result := ctl.reloadSnapshot()
		if len(result.Actions) != 3 {
			t.Fatalf("reloadSnapshot actions=%v, want 3 actions", result.Actions)
		}
		if len(result.Services) != 2 {
			t.Fatalf("reloadSnapshot services=%v, want 2 services", result.Services)
		}
	})
}
