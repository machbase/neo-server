//go:build !windows

package sbin_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// ptySession drives an interactive jsh process attached to a pseudo terminal.
type ptySession struct {
	t    *testing.T
	f    *os.File
	mu   sync.Mutex
	out  bytes.Buffer
	done chan struct{}
}

func startReplPTY(t *testing.T, workDir string, args ...string) (*ptySession, *exec.Cmd) {
	t.Helper()
	cmdArgs := append([]string{"-v", "/work=" + workDir, "repl"}, args...)
	cmd := exec.Command(pkgTestJshBinPath, cmdArgs...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "HOME="+workDir, "TERM=xterm")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatalf("start pty: %v", err)
	}
	s := &ptySession{t: t, f: f, done: make(chan struct{})}
	go s.readLoop()
	return s, cmd
}

func (s *ptySession) readLoop() {
	defer close(s.done)
	buf := make([]byte, 4096)
	for {
		n, err := s.f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// Answer cursor position queries issued by the line editor.
			if bytes.Contains(chunk, []byte("\x1b[6n")) {
				s.f.Write([]byte("\x1b[1;1R")) //nolint:errcheck
			}
			s.mu.Lock()
			s.out.Write(chunk)
			s.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

func (s *ptySession) waitFor(text string, after int) int {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		out := s.output()
		if idx := strings.Index(out[after:], text); idx >= 0 {
			return after + idx + len(text)
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timeout waiting for %q\noutput:\n%s", text, s.output())
	return 0
}

func (s *ptySession) send(line string) {
	s.t.Helper()
	if _, err := s.f.Write([]byte(line + "\r")); err != nil {
		s.t.Fatalf("write %q: %v", line, err)
	}
}

func TestReplInteractivePTY(t *testing.T) {
	workDir := t.TempDir()
	s, cmd := startReplPTY(t, workDir, "--no-history", "--transcript", "/work/t.ndjson")
	defer s.f.Close()

	pos := s.waitFor("JSH REPL", 0)
	s.send("let x = 20")
	s.send("x * 2 + 2")
	pos = s.waitFor("42", pos)
	s.send("throw new Error('boom')")
	pos = s.waitFor("boom", pos)
	s.send("x + 1")
	s.waitFor("21", pos)
	s.send(`\quit`)

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()
	select {
	case err := <-waitErr:
		if err != nil {
			t.Fatalf("repl exited with error: %v\n%s", err, s.output())
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill() //nolint:errcheck
		t.Fatalf("repl did not exit after \\quit\n%s", s.output())
	}
	<-s.done
	if strings.Contains(s.output(), "{value:") {
		t.Fatalf("error output should be plain text\n%s", s.output())
	}

	data, err := os.ReadFile(filepath.Join(workDir, "t.ndjson"))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var kinds []string
	var results []any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("invalid transcript line %q: %v", line, err)
		}
		kinds = append(kinds, ev["event"].(string))
		if ev["event"] == "result" {
			results = append(results, ev["result"])
		}
	}
	want := "session_start,input,result,input,result,input,error,input,result,session_end"
	if got := strings.Join(kinds, ","); got != want {
		t.Fatalf("transcript events = %s, want %s\n%s", got, want, data)
	}
	if results[1] != float64(42) || results[2] != float64(21) {
		t.Fatalf("unexpected transcript results %v\n%s", results, data)
	}
}
