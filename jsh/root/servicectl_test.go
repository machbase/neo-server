package root_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/service"
)

type serviceRPCRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func TestServiceCommandStatusListFormatting(t *testing.T) {
	for _, network := range []string{"tcp", "unix"} {
		t.Run(network, func(t *testing.T) {
			addr, shutdown := startMockServiceRPCServerTransport(t, network, func(req serviceRPCRequest) any {
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

			output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "status")
			if err != nil {
				t.Fatalf("service status failed: %v\n%s", err, output)
			}

			lines := nonEmptyLines(output)
			if len(lines) < 5 {
				t.Fatalf("service status output too short: %q", output)
			}
			if lines[0] != "SERVICES (2)" {
				t.Fatalf("header=%q, want %q", lines[0], "SERVICES (2)")
			}
			if !strings.Contains(output, "NAME") || !strings.Contains(output, "EXECUTABLE") {
				t.Fatalf("missing table header columns:\n%s", output)
			}
			if !strings.Contains(output, "alpha") || !strings.Contains(output, "running") || !strings.Contains(output, "101") {
				t.Fatalf("missing alpha/running/101 row:\n%s", output)
			}
			if !strings.Contains(output, "beta") || !strings.Contains(output, "stopped") || !strings.Contains(output, "/bin/date") {
				t.Fatalf("missing beta/stopped/date row:\n%s", output)
			}
		})
	}
}

func TestServiceCommandListRemoved(t *testing.T) {
	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller=127.0.0.1:1", "list")
	if err == nil {
		t.Fatalf("service list unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "Unknown command 'list'.") {
		t.Fatalf("output=%q, want unknown command error", output)
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
				"working_dir": "/work",
				"environment": map[string]any{"A": "1", "B": "2"},
				"executable":  "echo",
				"args":        []any{"hello", "world"},
			},
			"status":    "running",
			"exit_code": 0,
			"pid":       55,
			"output": []any{
				"line-1", "line-2", "line-3", "line-4", "line-5",
				"line-6", "line-7", "line-8", "line-9", "line-10",
				"line-11", "line-12", "line-13", "line-14", "line-15",
				"line-16", "line-17", "line-18", "line-19", "line-20",
				"line-21", "line-22", "line-23", "line-24", "line-25",
			},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "status", "alpha")
	if err != nil {
		t.Fatalf("service status failed: %v\n%s", err, output)
	}

	checks := []string{
		"SERVICE",
		"KEY",
		"VALUE",
		"name",
		"alpha",
		"enabled",
		"yes",
		"status",
		"running",
		"exit_code",
		"0",
		"pid",
		"55",
		"cwd",
		"/work",
		"ENVIRONMENT",
		"A",
		"1",
		"B",
		"2",
		"OUTPUT",
		"line-6",
		"line-25",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("output missing %q:\n%s", check, output)
		}
	}
	lines := nonEmptyLines(output)
	for _, unwanted := range []string{"line-1", "line-2", "line-3", "line-4", "line-5"} {
		for _, line := range lines {
			if strings.TrimSpace(line) == unwanted {
				t.Fatalf("output unexpectedly contains exact line %q:\n%s", unwanted, output)
			}
		}
	}
}

func TestServiceCommandInstallFromFile(t *testing.T) {
	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "svc.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"name\": \"svc-file\",\n  \"enable\": true,\n  \"working_dir\": \"/srv\",\n  \"environment\": {\"A\": \"1\"},\n  \"executable\": \"echo\",\n  \"args\": [\"hello\"]\n}\n"), 0o644); err != nil {
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

	output, err := runCommand(workDir, nil, "servicectl", "--controller="+addr, "install", "svc.json")
	if err != nil {
		t.Fatalf("service install file failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "OPERATION", "install", "svc-file", "yes", "running", "SERVICE", "KEY", "name", "enabled", "OUTPUT"}
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
		"servicectl",
		"--controller="+addr,
		"install",
		"--name", "svc-inline",
		"--executable", "node",
		"--working-dir", "/work/app",
		"--enable",
		"--arg", "app.js",
		"--arg", "--port",
		"--arg", "8080",
		"--env", "A=1",
		"--env", "B=2",
	)
	if err != nil {
		t.Fatalf("service install inline failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "install", "svc-inline", "yes", "stopped", "SERVICE", "KEY", "name", "enabled", "OUTPUT"}
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

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "start", "alpha")
	if err != nil {
		t.Fatalf("service start failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "start", "alpha", "yes", "running", "88", "SERVICE", "KEY", "name", "enabled", "OUTPUT"}
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

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "stop", "alpha")
	if err != nil {
		t.Fatalf("service stop failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "stop", "alpha", "yes", "stopped", "0", "SERVICE", "KEY", "name", "enabled", "OUTPUT"}
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

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "uninstall", "alpha")
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

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "read")
	if err != nil {
		t.Fatalf("service read failed: %v\n%s", err, output)
	}

	checks := []string{
		"STATUS",
		"NAME",
		"EXECUTABLE",
		"UNCHANGED",
		"alpha",
		"echo",
		"ADDED",
		"beta",
		"node",
		"REMOVED",
		"old",
		"sleep",
		"ERRORED",
		"broken",
		"invalid json",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("output missing %q:\n%s", check, output)
		}
	}
	if !strings.Contains(output, "┌") || !strings.Contains(output, "└") {
		t.Fatalf("output missing pretty box borders:\n%s", output)
	}
	if !strings.Contains(output, "NAME") || !strings.Contains(output, "STATUS") || strings.Index(output, "NAME") > strings.Index(output, "STATUS") {
		t.Fatalf("output header order mismatch, want NAME before STATUS:\n%s", output)
	}
}

func TestServiceCommandReloadFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.reload" {
			t.Fatalf("method=%q, want %q", req.Method, "service.reload")
		}
		return map[string]any{
			"actions": []any{
				map[string]any{"name": "alpha", "action": "RELOAD stop"},
				map[string]any{"name": "alpha", "action": "RELOAD start"},
			},
			"services": []any{
				map[string]any{"config": map[string]any{"name": "alpha", "enable": true, "executable": "echo"}, "status": "running", "pid": 91},
			},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "reload")
	if err != nil {
		t.Fatalf("service reload failed: %v\n%s", err, output)
	}

	checks := []string{
		"ACTIONS (2)",
		"NAME",
		"ACTION",
		"alpha",
		"RELOAD stop",
		"RELOAD start",
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

func TestServiceCommandDetailsGetFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.runtime.get" {
			t.Fatalf("method=%q, want %q", req.Method, "service.runtime.get")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "alpha" {
			t.Fatalf("details get name=%v, want alpha", params["name"])
		}
		return map[string]any{
			"output": []any{},
			"details": map[string]any{
				"enabled": true,
				"labels":  map[string]any{"tier": "gold"},
				"retries": 3,
			},
		}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "details", "get", "alpha")
	if err != nil {
		t.Fatalf("service details get failed: %v\n%s", err, output)
	}
	checks := []string{"DETAILS (3)", "KEY", "TYPE", "VALUE", "enabled", "boolean", "true", "labels", "object", "{\"tier\":\"gold\"}", "retries", "number", "3"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("details get output missing %q:\n%s", check, output)
		}
	}

	output, err = runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "details", "get", "alpha", "labels")
	if err != nil {
		t.Fatalf("service details get key failed: %v\n%s", err, output)
	}
	if strings.Contains(output, "retries") || strings.Contains(output, "enabled") {
		t.Fatalf("details get key output should only contain selected key:\n%s", output)
	}
	if !strings.Contains(output, "labels") || !strings.Contains(output, "tier") || !strings.Contains(output, "gold") {
		t.Fatalf("details get key output missing labels object:\n%s", output)
	}

	output, err = runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "details", "get", "alpha", "labels", "--format", "json")
	if err != nil {
		t.Fatalf("service details get json failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(output) != "{\n  \"labels\": {\n    \"tier\": \"gold\"\n  }\n}" {
		t.Fatalf("details get json output=%q, want wrapped object json", output)
	}
}

func TestServiceCommandDetailsSetTypedValues(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		expected   any
		typeName   string
		resultText string
	}{
		{name: "string default", args: []string{"details", "set", "alpha", "mode", "warm"}, expected: "warm", typeName: "string", resultText: "warm"},
		{name: "string explicit", args: []string{"details", "set", "alpha", "mode", "warm", "--detail-type", "string"}, expected: "warm", typeName: "string", resultText: "warm"},
		{name: "number", args: []string{"details", "set", "alpha", "retries", "3.5", "--detail-type", "number"}, expected: 3.5, typeName: "number", resultText: "3.5"},
		{name: "boolean alias", args: []string{"details", "set", "alpha", "enabled", "true", "--detail-type", "bool"}, expected: true, typeName: "boolean", resultText: "true"},
		{name: "object alias", args: []string{"details", "set", "alpha", "labels", "{\"tier\":\"gold\"}", "--detail-type", "json"}, expected: map[string]any{"tier": "gold"}, typeName: "object", resultText: "{\"tier\":\"gold\"}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
				if req.Method != "service.runtime.detail.set" {
					t.Fatalf("method=%q, want %q", req.Method, "service.runtime.detail.set")
				}
				var params map[string]any
				if err := json.Unmarshal(req.Params, &params); err != nil {
					t.Fatalf("unmarshal params: %v", err)
				}
				if params["name"] != "alpha" {
					t.Fatalf("set name=%v, want alpha", params["name"])
				}
				if gotKey := params["key"]; gotKey != tc.args[3] {
					t.Fatalf("set key=%v, want %s", gotKey, tc.args[3])
				}
				assertDetailValue(t, params["value"], tc.expected)
				return map[string]any{"output": []any{}, "details": map[string]any{tc.args[3]: tc.expected}}
			})
			defer shutdown()

			args := append([]string{"servicectl", "--controller=" + addr}, tc.args...)
			output, err := runCommand(t.TempDir(), nil, args...)
			if err != nil {
				t.Fatalf("service details set failed: %v\n%s", err, output)
			}
			checks := []string{"RESULT", "details set", "alpha", tc.args[3], tc.typeName, tc.resultText, "yes", "DETAILS (1)"}
			for _, check := range checks {
				if !strings.Contains(output, check) {
					t.Fatalf("details set output missing %q:\n%s", check, output)
				}
			}
		})
	}
}

func TestServiceCommandDetailsDeleteFormatting(t *testing.T) {
	addr, shutdown := startMockServiceRPCServer(t, func(req serviceRPCRequest) any {
		if req.Method != "service.runtime.detail.delete" {
			t.Fatalf("method=%q, want %q", req.Method, "service.runtime.detail.delete")
		}
		var params map[string]any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["name"] != "alpha" || params["key"] != "labels" {
			t.Fatalf("delete params=%v, want alpha/labels", params)
		}
		return map[string]any{"output": []any{}, "details": map[string]any{}}
	})
	defer shutdown()

	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller="+addr, "details", "delete", "alpha", "labels")
	if err != nil {
		t.Fatalf("service details delete failed: %v\n%s", err, output)
	}
	checks := []string{"RESULT", "details delete", "alpha", "labels", "yes", "DETAILS (0)", "(none)"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("details delete output missing %q:\n%s", check, output)
		}
	}
}

func TestServiceCommandDetailsSetRejectsInvalidTypeInput(t *testing.T) {
	output, err := runCommand(t.TempDir(), nil, "servicectl", "--controller=127.0.0.1:1", "details", "set", "alpha", "retries", "abc", "--detail-type", "number")
	if err == nil {
		t.Fatalf("service details set unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "Failed to parse number detail value 'abc'") {
		t.Fatalf("output=%q, want number parse error", output)
	}

	output, err = runCommand(t.TempDir(), nil, "servicectl", "--controller=127.0.0.1:1", "details", "set", "alpha", "labels", "[]", "--detail-type", "object")
	if err == nil {
		t.Fatalf("service details set object unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "is not a valid JSON object") {
		t.Fatalf("output=%q, want object validation error", output)
	}

	output, err = runCommand(t.TempDir(), nil, "servicectl", "--controller=127.0.0.1:1", "details", "get", "alpha", "--format", "yaml")
	if err == nil {
		t.Fatalf("service details get with invalid format unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "Invalid --format 'yaml'. Use box or json.") {
		t.Fatalf("output=%q, want invalid format error", output)
	}
}

func TestServiceCommandControllerEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		name    string
		address func(string) string
	}{
		{
			name:    "tcp",
			address: func(string) string { return "" },
		},
		{
			name: "unix",
			address: func(string) string {
				return "unix://" + filepath.Join(os.TempDir(), fmt.Sprintf("service-controller-%d.sock", time.Now().UnixNano()))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			servicesDir := filepath.Join(workDir, "services")
			if err := os.MkdirAll(servicesDir, 0o755); err != nil {
				t.Fatalf("mkdir services: %v", err)
			}
			writeServiceJSON(t, filepath.Join(servicesDir, "alpha.json"), map[string]any{
				"name":        "alpha",
				"enable":      false,
				"working_dir": "/work",
				"executable":  "echo",
				"args":        []any{"hello"},
			})

			ctl, err := service.NewController(&service.ControllerConfig{
				ConfigDir: "/work/services",
				Mounts: []engine.FSTab{
					{MountPoint: "/work", FS: os.DirFS(workDir)},
				},
				Address: tc.address(workDir),
			})
			if err != nil {
				t.Fatalf("NewController() error: %v", err)
			}
			if err := ctl.Start(nil); err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			defer ctl.Stop(nil)

			controllerAddr := ctl.Address()

			listOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "status")
			if err != nil {
				t.Fatalf("service status failed: %v\n%s", err, listOutput)
			}
			if !strings.Contains(listOutput, "alpha") || !strings.Contains(listOutput, "stopped") {
				t.Fatalf("status output=%q, want alpha/stopped", listOutput)
			}

			readOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "read")
			if err != nil {
				t.Fatalf("service read failed: %v\n%s", err, readOutput)
			}
			if !strings.Contains(readOutput, "UNCHANGED") || !strings.Contains(readOutput, "alpha") || !strings.Contains(readOutput, "echo") {
				t.Fatalf("read output=%q, want unified unchanged alpha", readOutput)
			}

			writeServiceJSON(t, filepath.Join(servicesDir, "alpha.json"), map[string]any{
				"name":        "alpha",
				"enable":      true,
				"working_dir": "/work",
				"executable":  "echo",
				"args":        []any{"hello", "world"},
			})

			reloadOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "reload")
			if err != nil {
				t.Fatalf("service reload failed: %v\n%s", err, reloadOutput)
			}
			if !strings.Contains(reloadOutput, "ACTIONS") || !strings.Contains(reloadOutput, "RELOAD start") {
				t.Fatalf("reload output=%q, want reload start action", reloadOutput)
			}

			statusOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "status", "alpha")
			if err != nil {
				t.Fatalf("service status failed: %v\n%s", err, statusOutput)
			}
			checks := []string{"SERVICE", "name", "alpha", "status", "running", "cwd", "/work", "start", "echo [ hello, world ]", "OUTPUT"}
			for _, check := range checks {
				if !strings.Contains(statusOutput, check) {
					t.Fatalf("status output missing %q:\n%s", check, statusOutput)
				}
			}

			stopOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "stop", "alpha")
			if err != nil {
				t.Fatalf("service stop failed: %v\n%s", err, stopOutput)
			}
			for _, check := range []string{"RESULT", "stop", "alpha", "stopped", "SERVICE", "name", "enabled", "OUTPUT"} {
				if !strings.Contains(stopOutput, check) {
					t.Fatalf("stop output missing %q:\n%s", check, stopOutput)
				}
			}

			startOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "start", "alpha")
			if err != nil {
				t.Fatalf("service start failed: %v\n%s", err, startOutput)
			}
			for _, check := range []string{"RESULT", "start", "alpha", "running", "SERVICE", "name", "enabled", "OUTPUT"} {
				if !strings.Contains(startOutput, check) {
					t.Fatalf("start output missing %q:\n%s", check, startOutput)
				}
			}

			stopBeforeUninstallOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "stop", "alpha")
			if err != nil {
				t.Fatalf("service stop before uninstall failed: %v\n%s", err, stopBeforeUninstallOutput)
			}
			for _, check := range []string{"RESULT", "stop", "alpha", "stopped"} {
				if !strings.Contains(stopBeforeUninstallOutput, check) {
					t.Fatalf("stop before uninstall output missing %q:\n%s", check, stopBeforeUninstallOutput)
				}
			}

			uninstallOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "uninstall", "alpha")
			if err != nil {
				t.Fatalf("service uninstall failed: %v\n%s", err, uninstallOutput)
			}
			for _, check := range []string{"RESULT", "uninstall", "alpha", "yes", "removed"} {
				if !strings.Contains(uninstallOutput, check) {
					t.Fatalf("uninstall output missing %q:\n%s", check, uninstallOutput)
				}
			}

			listAfterUninstallOutput, err := runCommand(workDir, nil, "servicectl", "--controller="+controllerAddr, "status")
			if err != nil {
				t.Fatalf("service status after uninstall failed: %v\n%s", err, listAfterUninstallOutput)
			}
			if !strings.Contains(listAfterUninstallOutput, "SERVICES (0)") || !strings.Contains(listAfterUninstallOutput, "(none)") {
				t.Fatalf("status after uninstall output=%q, want no services", listAfterUninstallOutput)
			}
		})
	}
}

func startMockServiceRPCServer(t *testing.T, handler func(serviceRPCRequest) any) (string, func()) {
	t.Helper()
	return startMockServiceRPCServerTransport(t, "tcp", handler)
}

func startMockServiceRPCServerTransport(t *testing.T, network string, handler func(serviceRPCRequest) any) (string, func()) {
	t.Helper()

	listenAddr := "127.0.0.1:0"
	listenURLPrefix := "tcp://"
	if network == "unix" {
		listenAddr = filepath.Join(os.TempDir(), fmt.Sprintf("service-rpc-%d.sock", time.Now().UnixNano()))
		listenURLPrefix = "unix://"
		_ = os.Remove(listenAddr)
	}
	ln, err := net.Listen(network, listenAddr)
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
				}
				resp["result"] = handler(req)
				if len(req.ID) > 0 {
					resp["id"] = req.ID
				}
				if err := json.NewEncoder(conn).Encode(resp); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}(conn)
		}
	}()

	return listenURLPrefix + ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
		wg.Wait()
		if network == "unix" {
			_ = os.Remove(listenAddr)
		}
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

func assertDetailValue(t *testing.T, got any, want any) {
	t.Helper()
	switch wantValue := want.(type) {
	case string:
		if got != wantValue {
			t.Fatalf("detail value=%#v, want %#v", got, wantValue)
		}
	case bool:
		if got != wantValue {
			t.Fatalf("detail value=%#v, want %#v", got, wantValue)
		}
	case float64:
		gotNumber, ok := got.(float64)
		if !ok || gotNumber != wantValue {
			t.Fatalf("detail value=%#v, want %#v", got, wantValue)
		}
	case map[string]any:
		gotMap, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("detail value=%#v, want object", got)
		}
		gotBytes, err := json.Marshal(gotMap)
		if err != nil {
			t.Fatalf("marshal got map: %v", err)
		}
		wantBytes, err := json.Marshal(wantValue)
		if err != nil {
			t.Fatalf("marshal want map: %v", err)
		}
		if string(gotBytes) != string(wantBytes) {
			t.Fatalf("detail value=%s, want %s", gotBytes, wantBytes)
		}
	default:
		t.Fatalf("unsupported expected detail type %T", want)
	}
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
