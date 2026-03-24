package root_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	jshservice "github.com/machbase/neo-server/v8/jsh/service"
)

type serviceRPCRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func TestServiceCommandListFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.list" {
			t.Fatalf("method=%q, want %q", req.Method, "service.list")
		}
		return []map[string]any{
			{
				"config": map[string]any{"name": "alpha", "enable": true, "executable": "echo"},
				"status": "running",
				"pid":    101,
			},
			{
				"config": map[string]any{"name": "beta", "enable": false, "executable": "/bin/date"},
				"status": "stopped",
			},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "list")
	if err != nil {
		t.Fatalf("service list failed: %v\n%s", err, output)
	}

	lines := nonEmptyLines(output)
	if len(lines) < 4 {
		t.Fatalf("service list output too short: %q", output)
	}
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "EXECUTABLE") {
		t.Fatalf("header=%q, want columns", lines[0])
	}
	if !strings.Contains(lines[2], "alpha") || !strings.Contains(lines[2], "running") || !strings.Contains(lines[2], "101") {
		t.Fatalf("first row=%q, want alpha/running/101", lines[2])
	}
	if !strings.Contains(lines[3], "beta") || !strings.Contains(lines[3], "stopped") || !strings.Contains(lines[3], "/bin/date") {
		t.Fatalf("second row=%q, want beta/stopped/date", lines[3])
	}
}

func TestServiceCommandStatusFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.get" {
			t.Fatalf("method=%q, want %q", req.Method, "service.get")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "alpha" {
			t.Fatalf("status name=%v, want alpha", params["name"])
		}
		return map[string]any{
			"config": map[string]any{
				"name":        "alpha",
				"enable":      true,
				"auto_start":  true,
				"working_dir": "/work",
				"environment": map[string]any{"A": "1", "B": "2"},
				"executable":  "echo",
				"args":        []any{"hello", "world"},
			},
			"status":    "running",
			"exit_code": 0,
			"pid":       55,
			"output":    []any{"line-1", "line-2"},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "status", "alpha")
	if err != nil {
		t.Fatalf("service status failed: %v\n%s", err, output)
	}

	checks := []string{
		"[alpha] ENABLED",
		"status: running",
		"auto_start: yes",
		"exit_code: 0",
		"pid: 55",
		"cwd: /work",
		"A=1",
		"B=2",
		"line-1",
		"line-2",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandInstallFromFile(t *testing.T) {
	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "svc.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"name\": \"svc-file\",\n  \"enable\": true,\n  \"auto_start\": true,\n  \"working_dir\": \"/srv\",\n  \"environment\": {\"A\": \"1\"},\n  \"executable\": \"echo\",\n  \"args\": [\"hello\"]\n}\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.install" {
			t.Fatalf("method=%q, want %q", req.Method, "service.install")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "svc-file" {
			t.Fatalf("install name=%v, want svc-file", params["name"])
		}
		if params["working_dir"] != "/srv" {
			t.Fatalf("install working_dir=%v, want /srv", params["working_dir"])
		}
		return map[string]any{
			"config": params,
			"status": "running",
			"output": []any{},
		}
	})
	defer shutdown()

	output, err := runCommand(workDir, nil, "service", "--controller="+addr, "install", "svc.json")
	if err != nil {
		t.Fatalf("service install file failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "OPERATION", "install", "svc-file", "yes", "running", "SERVICE", "[svc-file] ENABLED"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("install output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandInstallInline(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.install" {
			t.Fatalf("method=%q, want %q", req.Method, "service.install")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "svc-inline" {
			t.Fatalf("inline name=%v, want svc-inline", params["name"])
		}
		if params["executable"] != "node" {
			t.Fatalf("inline executable=%v, want node", params["executable"])
		}
		if params["working_dir"] != "/work/app" {
			t.Fatalf("inline working_dir=%v, want /work/app", params["working_dir"])
		}
		if params["enable"] != true {
			t.Fatalf("inline enable=%v, want true", params["enable"])
		}
		if params["auto_start"] != true {
			t.Fatalf("inline auto_start=%v, want true", params["auto_start"])
		}
		args, ok := params["args"].([]any)
		if !ok || len(args) != 3 || args[0] != "app.js" || args[1] != "--port" || args[2] != "8080" {
			t.Fatalf("inline args=%#v, want [app.js --port 8080]", params["args"])
		}
		env, ok := params["environment"].(map[string]any)
		if !ok || env["A"] != "1" || env["B"] != "2" {
			t.Fatalf("inline environment=%#v, want A=1 B=2", params["environment"])
		}
		return map[string]any{
			"config": params,
			"status": "stopped",
			"output": []any{},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil,
		"service",
		"--controller="+addr,
		"install",
		"--name", "svc-inline",
		"--executable", "node",
		"--working-dir", "/work/app",
		"--enable",
		"--auto-start",
		"--arg", "app.js",
		"--arg", "--port",
		"--arg", "8080",
		"--env", "A=1",
		"--env", "B=2",
	)
	if err != nil {
		t.Fatalf("service install inline failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "install", "svc-inline", "yes", "stopped", "SERVICE", "[svc-inline] ENABLED"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("inline install output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandStartFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.start" {
			t.Fatalf("method=%q, want %q", req.Method, "service.start")
		}
		return map[string]any{
			"config": map[string]any{"name": "alpha", "enable": true, "executable": "echo", "start_error": ""},
			"status": "running",
			"pid":    88,
			"output": []any{},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "start", "alpha")
	if err != nil {
		t.Fatalf("service start failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "start", "alpha", "yes", "running", "88", "SERVICE", "[alpha] ENABLED"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("start output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandStopFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.stop" {
			t.Fatalf("method=%q, want %q", req.Method, "service.stop")
		}
		return map[string]any{
			"config":    map[string]any{"name": "alpha", "enable": true, "executable": "echo", "stop_error": ""},
			"status":    "stopped",
			"exit_code": 0,
			"output":    []any{},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "stop", "alpha")
	if err != nil {
		t.Fatalf("service stop failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "stop", "alpha", "yes", "stopped", "0", "SERVICE", "[alpha] ENABLED"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("stop output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandUninstallFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.uninstall" {
			t.Fatalf("method=%q, want %q", req.Method, "service.uninstall")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "alpha" {
			t.Fatalf("uninstall name=%v, want alpha", params["name"])
		}
		return true
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "uninstall", "alpha")
	if err != nil {
		t.Fatalf("service uninstall failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "uninstall", "alpha", "yes", "removed"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("uninstall output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandReadFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.read" {
			t.Fatalf("method=%q, want %q", req.Method, "service.read")
		}
		return map[string]any{
			"unchanged": []any{map[string]any{"name": "alpha", "executable": "echo"}},
			"added":     []any{map[string]any{"name": "beta", "executable": "node"}},
			"updated":   []any{},
			"removed":   []any{map[string]any{"name": "old", "executable": "sleep"}},
			"errored":   []any{map[string]any{"name": "broken", "read_error": "invalid json"}},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "read")
	if err != nil {
		t.Fatalf("service read failed: %v\n%s", err, output)
	}

	checks := []string{
		"UNCHANGED (1)",
		"- alpha exec=echo",
		"ADDED (1)",
		"- beta exec=node",
		"UPDATED (0)",
		"(none)",
		"REMOVED (1)",
		"- old exec=sleep",
		"ERRORED (1)",
		"- broken read_error=invalid json",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandReloadFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.reload" {
			t.Fatalf("method=%q, want %q", req.Method, "service.reload")
		}
		return map[string]any{
			"actions": []any{
				map[string]any{"name": "alpha", "action": "UPDATE stop"},
				map[string]any{"name": "alpha", "action": "UPDATE start"},
			},
			"services": []any{
				map[string]any{"config": map[string]any{"name": "alpha", "enable": true, "executable": "echo"}, "status": "running", "pid": 91},
			},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "service", "--controller="+addr, "reload")
	if err != nil {
		t.Fatalf("service reload failed: %v\n%s", err, output)
	}

	checks := []string{
		"ACTIONS (2)",
		"NAME",
		"ACTION",
		"alpha",
		"UPDATE stop",
		"UPDATE start",
		"SERVICES",
		"EXECUTABLE",
		"running",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandControllerEndToEnd(t *testing.T) {
	workDir := t.TempDir()
	servicesDir := filepath.Join(workDir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir services: %v", err)
	}
	writeServiceJSON(t, filepath.Join(servicesDir, "alpha.json"), map[string]any{
		"name":        "alpha",
		"enable":      false,
		"auto_start":  false,
		"working_dir": "/work",
		"executable":  "echo",
		"args":        []any{"hello"},
	})

	ctl, err := jshservice.NewController(&jshservice.ControllerConfig{
		ConfigDir: "/work/services",
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(workDir)},
		},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	if err := ctl.Start(nil); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ctl.Stop(nil)

	controllerAddr := "127.0.0.1:" + strconv.Itoa(ctl.Port())

	listOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "list")
	if err != nil {
		t.Fatalf("service list failed: %v\n%s", err, listOutput)
	}
	if !strings.Contains(listOutput, "alpha") || !strings.Contains(listOutput, "stopped") {
		t.Fatalf("list output=%q, want alpha/stopped", listOutput)
	}

	readOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "read")
	if err != nil {
		t.Fatalf("service read failed: %v\n%s", err, readOutput)
	}
	if !strings.Contains(readOutput, "UNCHANGED (1)") || !strings.Contains(readOutput, "- alpha exec=echo") {
		t.Fatalf("read output=%q, want unchanged alpha", readOutput)
	}

	writeServiceJSON(t, filepath.Join(servicesDir, "alpha.json"), map[string]any{
		"name":        "alpha",
		"enable":      true,
		"auto_start":  false,
		"working_dir": "/work",
		"executable":  "echo",
		"args":        []any{"hello", "world"},
	})

	reloadOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "reload")
	if err != nil {
		t.Fatalf("service reload failed: %v\n%s", err, reloadOutput)
	}
	if !strings.Contains(reloadOutput, "ACTIONS") || !strings.Contains(reloadOutput, "UPDATE stop") || !strings.Contains(reloadOutput, "UPDATE start") {
		t.Fatalf("reload output=%q, want update actions", reloadOutput)
	}

	statusOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "status", "alpha")
	if err != nil {
		t.Fatalf("service status failed: %v\n%s", err, statusOutput)
	}
	checks := []string{"[alpha] ENABLED", "status: running", "cwd: /work", "start: echo [ hello, world ]"}
	for _, check := range checks {
		if !strings.Contains(statusOutput, check) {
			t.Fatalf("status output missing %q:\n%s", check, statusOutput)
		}
	}

	stopOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "stop", "alpha")
	if err != nil {
		t.Fatalf("service stop failed: %v\n%s", err, stopOutput)
	}
	for _, check := range []string{"RESULT", "stop", "alpha", "stopped", "SERVICE", "[alpha] ENABLED"} {
		if !strings.Contains(stopOutput, check) {
			t.Fatalf("stop output missing %q:\n%s", check, stopOutput)
		}
	}

	startOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "start", "alpha")
	if err != nil {
		t.Fatalf("service start failed: %v\n%s", err, startOutput)
	}
	for _, check := range []string{"RESULT", "start", "alpha", "running", "SERVICE", "[alpha] ENABLED"} {
		if !strings.Contains(startOutput, check) {
			t.Fatalf("start output missing %q:\n%s", check, startOutput)
		}
	}

	uninstallOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "uninstall", "alpha")
	if err != nil {
		t.Fatalf("service uninstall failed: %v\n%s", err, uninstallOutput)
	}
	for _, check := range []string{"RESULT", "uninstall", "alpha", "yes", "removed"} {
		if !strings.Contains(uninstallOutput, check) {
			t.Fatalf("uninstall output missing %q:\n%s", check, uninstallOutput)
		}
	}

	listAfterUninstallOutput, err := runCommand(workDir, nil, "service", "--controller="+controllerAddr, "list")
	if err != nil {
		t.Fatalf("service list after uninstall failed: %v\n%s", err, listAfterUninstallOutput)
	}
	if !strings.Contains(listAfterUninstallOutput, "No services") {
		t.Fatalf("list after uninstall output=%q, want no services", listAfterUninstallOutput)
	}
}

func startMockServiceRPCServer(t *testing.T, handler func(serviceRPCRequest) any) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					t.Errorf("accept: %v", err)
					return
				}
			}

			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()

				reader := bufio.NewReader(conn)
				line, err := reader.ReadBytes('\n')
				if err != nil {
					t.Errorf("read request: %v", err)
					return
				}

				var req serviceRPCRequest
				if err := json.Unmarshal(line, &req); err != nil {
					t.Errorf("unmarshal request: %v", err)
					return
				}

				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      json.RawMessage("1"),
					"result":  handler(req),
				}
				if len(req.ID) > 0 {
					resp["id"] = req.ID
				}
				if err := json.NewEncoder(conn).Encode(resp); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}(conn)
		}
	}()

	return ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
		wg.Wait()
	}
}

func nonEmptyLines(output string) []string {
	rawLines := strings.Split(output, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func writeServiceJSON(t *testing.T, filePath string, value any) {
	t.Helper()
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(filePath, append(bytes, '\n'), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
