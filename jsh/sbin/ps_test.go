package sbin_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestPSCommandListsProcEntries(t *testing.T) {
	workDir := t.TempDir()
	procDir := t.TempDir()
	entryDir := filepath.Join(procDir, "process", "321")
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatalf("mkdir proc entry: %v", err)
	}

	writeProcJSON(t, filepath.Join(entryDir, "meta.json"), map[string]any{
		"pid":                          321,
		"ppid":                         123,
		"pgid":                         321,
		"command":                      "/tmp/jsh",
		"args":                         []string{"-C", "/sbin/sleep.js", "1"},
		"cwd":                          "/work",
		"started_at":                   "2026-04-07T10:11:12Z",
		"service_controller_client_id": "client-a",
	})
	writeProcJSON(t, filepath.Join(entryDir, "status.json"), map[string]any{
		"pid":        321,
		"state":      "running",
		"updated_at": "2026-04-07T10:11:13Z",
		"started_at": "2026-04-07T10:11:12Z",
	})

	output, err := runCommandWithProcMount(workDir, procDir, "ps")
	if err != nil {
		t.Fatalf("ps failed: %v\n%s", err, output)
	}

	for _, want := range []string{
		"PID",
		"PPID",
		"STATE",
		"COMMAND",
		"321",
		"123",
		"running",
		"2026-04-07 10:11:12",
		"/sbin/sleep.js 1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("ps output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "2026-04-07T10:11:12Z") {
		t.Fatalf("ps output should not include RFC3339 timestamp:\n%s", output)
	}
	if strings.Contains(output, "PGID") {
		t.Fatalf("ps output should not include PGID column:\n%s", output)
	}
	if strings.Contains(output, "/tmp/jsh -C /sbin/sleep.js 1") {
		t.Fatalf("ps output should not include launcher command:\n%s", output)
	}
}

func TestPSCommandPrintsJSON(t *testing.T) {
	workDir := t.TempDir()
	procDir := t.TempDir()
	entryDir := filepath.Join(procDir, "process", "654")
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatalf("mkdir proc entry: %v", err)
	}

	writeProcJSON(t, filepath.Join(entryDir, "meta.json"), map[string]any{
		"pid":        654,
		"ppid":       111,
		"pgid":       654,
		"command":    "/tmp/jsh",
		"args":       []string{"-C", "/sbin/echo.js", "hello"},
		"cwd":        "/work",
		"started_at": "2026-04-07T11:00:00Z",
	})
	writeProcJSON(t, filepath.Join(entryDir, "status.json"), map[string]any{
		"pid":        654,
		"state":      "running",
		"updated_at": "2026-04-07T11:00:01Z",
		"started_at": "2026-04-07T11:00:00Z",
	})

	output, err := runCommandWithProcMount(workDir, procDir, "ps", "--json")
	if err != nil {
		t.Fatalf("ps --json failed: %v\n%s", err, output)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("unmarshal ps --json output: %v\n%s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("entries length = %d, want 1", len(entries))
	}
	if entries[0]["state"] != "running" {
		t.Fatalf("state = %v, want running", entries[0]["state"])
	}
	if entries[0]["command_line"] != "/tmp/jsh -C /sbin/echo.js hello" {
		t.Fatalf("command_line = %v", entries[0]["command_line"])
	}
}

func runCommandWithProcMount(workDir string, procDir string, args ...string) (string, error) {
	env := map[string]any{
		"PATH":         "/sbin:/work",
		"PWD":          "/work",
		"HOME":         "/work",
		"LIBRARY_PATH": "./node_modules:/lib",
	}
	conf := engine.Config{
		Name: "ps-test",
		Args: args,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/work", Source: workDir},
			{MountPoint: "/tmp", Source: workDir},
			{MountPoint: "/lib", FS: lib.LibFS()},
			{MountPoint: "/proc", Source: procDir},
		},
		Env:    env,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		return "", err
	}
	lib.Enable(jr)
	err = jr.Run()
	return conf.Writer.(*bytes.Buffer).String(), err
}

func writeProcJSON(t *testing.T, filename string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", filename, err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(filename, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}
