package sbin_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runReplCLI runs `/sbin/repl` through the standalone jsh binary with workDir
// mounted at /work and returns the combined output and exit code.
func runReplCLI(t *testing.T, workDir string, args ...string) (string, int) {
	t.Helper()
	cmdArgs := append([]string{"-v", "/work=" + workDir, "repl"}, args...)
	cmd := exec.Command(pkgTestJshBinPath, cmdArgs...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "HOME="+workDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run repl %v: %v\n%s", args, err, out)
		}
		return string(out), exitErr.ExitCode()
	}
	return string(out), 0
}

func TestReplCLIExitCodes(t *testing.T) {
	workDir := t.TempDir()
	tests := []struct {
		name     string
		args     []string
		wantCode int
		contains string
	}{
		{"print success", []string{"--print", "6 * 7"}, 0, "42"},
		{"eval error", []string{"--eval", "throw new Error('boom')"}, 1, "boom"},
		{"eval timeout", []string{"--timeout", "50", "--eval", "while (true) {}"}, 1, "execution timeout"},
		{"json eval error", []string{"--json", "--eval", "throw new Error('boom')"}, 1, `"ok":false`},
		{"help", []string{"-h"}, 0, "Usage: repl"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runReplCLI(t, workDir, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\n%s", code, tc.wantCode, out)
			}
			if !strings.Contains(out, tc.contains) {
				t.Fatalf("output should contain %q\n%s", tc.contains, out)
			}
			if strings.Contains(out, "{value:") {
				t.Fatalf("error output should be plain text\n%s", out)
			}
		})
	}
}

func TestReplCLIRejectsUnknownOptions(t *testing.T) {
	workDir := t.TempDir()
	for _, opt := range []string{"--profile", "--no-such-option"} {
		t.Run(opt, func(t *testing.T) {
			out, code := runReplCLI(t, workDir, opt, "user", "--eval", "1")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1\n%s", code, out)
			}
			if !strings.Contains(out, "Unknown option: "+opt) {
				t.Fatalf("expected unknown option error\n%s", out)
			}
		})
	}
}

func TestReplCLIPreloadsInEvalMode(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "pre.js"), []byte("globalThis.preloaded = 7;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runReplCLI(t, workDir, "--load", "/work/pre.js", "--print", "preloaded * 6")
	if code != 0 || strings.TrimSpace(out) != "42" {
		t.Fatalf("load + print: code=%d output=%q", code, out)
	}

	out, code = runReplCLI(t, workDir, "--require", "path", "--print", "typeof require('path').join")
	if code != 0 || strings.TrimSpace(out) != "function" {
		t.Fatalf("require + print: code=%d output=%q", code, out)
	}

	out, code = runReplCLI(t, workDir, "--load", "/work/missing.js", "--eval", "1")
	if code != 1 || !strings.Contains(out, "missing.js") {
		t.Fatalf("missing preload: code=%d output=%q", code, out)
	}
}

func TestReplCLIJSONTranscript(t *testing.T) {
	workDir := t.TempDir()
	out, code := runReplCLI(t, workDir, "--json", "--transcript", "/work/t.ndjson", "--eval", "({answer: 42, token: 'x'})")
	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, out)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, out)
	}
	value, _ := result["value"].(map[string]any)
	if result["ok"] != true || value["answer"] != float64(42) {
		t.Fatalf("unexpected JSON output: %s", out)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "t.ndjson"))
	if err != nil {
		t.Fatalf("transcript should be written through the /work mount: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected input and result events, got:\n%s", data)
	}
	var input, res map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &input); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &res); err != nil {
		t.Fatal(err)
	}
	recorded, _ := res["result"].(map[string]any)
	if input["event"] != "input" || res["event"] != "result" || recorded["token"] != "[REDACTED]" {
		t.Fatalf("unexpected transcript:\n%s", data)
	}

	out, code = runReplCLI(t, workDir, "--transcript", "/work/missing-dir/t.ndjson", "--eval", "1")
	if code != 1 || !strings.Contains(out, "transcript") {
		t.Fatalf("unwritable transcript: code=%d output=%q", code, out)
	}
}
