//go:build windows

package engine_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
	"golang.org/x/sys/windows"
)

func prepareSignalHelperCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func sendTestSignal(cmd *exec.Cmd, signalName string) error {
	switch signalName {
	case "SIGINT":
		return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
	default:
		return fmt.Errorf("unsupported external signal delivery on windows: %s", signalName)
	}
}

func TestProcessKillSigintUnavailableIntegration(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessSignalHelper$", "--")
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL=SIGINT",
		"JSH_TEST_LISTEN_SIGNAL=1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	linesCh := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		close(linesCh)
	}()

	ready := false
	var lines []string
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for !ready {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("helper exited before readiness\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if line == "ready: SIGINT" {
				ready = true
			}
		case <-timer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for readiness\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderr.String())
		}
	}

	assertKillErrorContains(t, cmd.Process.Pid, `"SIGINT"`, "SIGINT delivery on windows requires a console process group")

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func assertKillErrorContains(t *testing.T, pid int, signalExpr string, want string) {
	t.Helper()
	writer := &bytes.Buffer{}
	conf := engine.Config{
		Name: "process_kill_sigint_windows_error",
		Code: fmt.Sprintf(`
			const process = require("process");
			const result = process.kill(%d, %s);
			if (result instanceof Error) {
				console.println("error:", result.message.includes(%q));
			} else {
				console.println("error:", false);
			}
		`, pid, signalExpr, want),
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Reader: bytes.NewBuffer(nil),
		Writer: writer,
	}

	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		t.Fatalf("run process.kill error script: %v", err)
	}
	if !strings.Contains(writer.String(), "error: true") {
		t.Fatalf("unexpected process.kill error output: %s", writer.String())
	}
}
