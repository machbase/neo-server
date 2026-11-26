//go:build !windows

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDaemonHelper(t *testing.T) {
	scenario := os.Getenv("NEO_DAEMON_TEST")
	if scenario == "" {
		t.Skip("subprocess helper")
	}
	pidPath := os.Getenv("NEO_DAEMON_TEST_PID")
	wasChild := os.Getenv(daemonChildEnv) == "1"
	err := daemonizeWithError(os.Getenv("NEO_DAEMON_TEST_LOG"), pidPath, func(ready func() error, file *os.File) (err error) {
		remove, err := createPIDFile(pidPath, file)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, remove()) }()
		if err := os.WriteFile(os.Getenv("NEO_DAEMON_TEST_MARKER"), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
			return err
		}
		switch scenario {
		case "failure", "failure-no-pid":
			return errors.New("startup failure")
		case "exit":
			os.Exit(37)
		}
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		defer signal.Stop(signals)
		if scenario == "blocked" {
			<-signals
			return errors.New("startup cancelled")
		}
		listener, err := net.Listen("unix", os.Getenv("NEO_DAEMON_TEST_SOCKET"))
		if err != nil {
			return err
		}
		defer listener.Close()
		if err := ready(); err != nil {
			return err
		}
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if wasChild {
		if err := os.WriteFile(os.Getenv("NEO_DAEMON_TEST_MARKER")+".done", nil, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func TestDaemonStopHelper(t *testing.T) {
	if os.Getenv("NEO_DAEMON_STOP") != "1" {
		t.Skip("subprocess helper")
	}
	conn, err := net.Dial("unix", "stop.sock")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestHeadlessDaemonHelper(t *testing.T) {
	dir := os.Getenv("NEO_HEADLESS_DAEMON_TEST_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	dataDir := filepath.Join(dir, "data")
	if os.Getenv("NEO_HEADLESS_DAEMON_TEST_FAIL") == "1" {
		dataDir = "machbase://127.0.0.1:1"
	}
	args := []string{
		"neo", "serve-headless", "--daemon",
		"--data", dataDir,
		"--pref", filepath.Join(dir, "pref"),
		"--mach-port", os.Getenv("NEO_HEADLESS_DAEMON_TEST_PORT"),
		"--machbase-init-option", "1",
		"--pid", filepath.Join(dir, "server.pid"),
		"--bootlog", filepath.Join(dir, "boot.log"),
		"--log-filename", "-",
	}
	os.Exit(Main(args))
}

func TestHeadlessDaemonStartup(t *testing.T) {
	if os.Getenv("GO_WANT_DO_RESTORE_HELPER") != "1" {
		t.Skip("requires isolated server lifecycle")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "server.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHeadlessDaemonHelper$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_DO_RESTORE_HELPER=1",
		"NEO_HEADLESS_DAEMON_TEST_DIR="+dir,
		"NEO_HEADLESS_DAEMON_TEST_PORT="+strconv.Itoa(port),
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	pidText, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	pidInfo, err := os.Lstat(pidPath)
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(pidText))
	require.NoError(t, err)
	require.Positive(t, pid)
	process, err := os.FindProcess(pid)
	require.NoError(t, err)
	t.Cleanup(func() {
		current, err := os.Lstat(pidPath)
		if os.IsNotExist(err) {
			return
		}
		require.NoError(t, err)
		require.True(t, os.SameFile(pidInfo, current), "daemon PID file was replaced")
		require.NoError(t, process.Signal(syscall.SIGTERM))
		require.Eventually(t, func() bool {
			_, err := os.Stat(pidPath)
			return os.IsNotExist(err)
		}, 30*time.Second, 20*time.Millisecond, "daemon did not remove its PID file")
	})
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.NoError(t, process.Signal(syscall.SIGTERM))
	require.Eventually(t, func() bool {
		_, err := os.Stat(pidPath)
		return os.IsNotExist(err)
	}, 30*time.Second, 20*time.Millisecond, "daemon did not remove its PID file")
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	}, 5*time.Second, 20*time.Millisecond, "database port remained open")
}

func TestHeadlessDaemonStartupFailure(t *testing.T) {
	if os.Getenv("GO_WANT_DO_RESTORE_HELPER") != "1" {
		t.Skip("requires isolated server lifecycle")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHeadlessDaemonHelper$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_DO_RESTORE_HELPER=1",
		"NEO_HEADLESS_DAEMON_TEST_DIR="+dir,
		"NEO_HEADLESS_DAEMON_TEST_PORT=1",
		"NEO_HEADLESS_DAEMON_TEST_FAIL=1",
	)
	output, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(output), "headless mode is not possible")
	_, err = os.Lstat(filepath.Join(dir, "server.pid"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDaemonStartupInterrupted(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "server.pid")
	marker := filepath.Join(dir, "started")
	cmd := exec.Command(os.Args[0], "-test.run=^TestDaemonHelper$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_DO_RESTORE_HELPER=1",
		"NEO_DAEMON_TEST=blocked",
		"NEO_DAEMON_TEST_PID="+pidPath,
		"NEO_DAEMON_TEST_MARKER="+marker,
	)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
	})
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	require.Error(t, cmd.Wait())
	_, err := os.Stat(pidPath)
	require.True(t, os.IsNotExist(err), "%v", err)
}

func TestDaemonLifecycle(t *testing.T) {
	for _, scenario := range []string{"success", "success-no-pid", "success-relative", "failure", "failure-no-pid", "exit", "existing", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			pidPath := filepath.Join(dir, "server.pid")
			pidArg := pidPath
			if strings.HasSuffix(scenario, "-no-pid") {
				pidArg = ""
			} else if scenario == "success-relative" {
				pidArg = "server.pid"
			}
			if scenario == "existing" {
				require.NoError(t, os.WriteFile(pidPath, []byte("other process"), 0o644))
			}
			if scenario == "missing" {
				pidPath = filepath.Join(dir, "missing", "server.pid")
				pidArg = pidPath
			}
			logPath := filepath.Join(dir, "boot.log")
			logArg := logPath
			if scenario == "success-relative" {
				logArg = "boot.log"
			} else if scenario == "success-no-pid" {
				logArg = ""
			}
			marker := filepath.Join(dir, "started")
			cmd := exec.Command(os.Args[0], "-test.run=^TestDaemonHelper$")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"GO_WANT_DO_RESTORE_HELPER=1",
				"NEO_DAEMON_TEST="+scenario,
				"NEO_DAEMON_TEST_PID="+pidArg,
				"NEO_DAEMON_TEST_LOG="+logArg,
				"NEO_DAEMON_TEST_MARKER="+marker,
				"NEO_DAEMON_TEST_SOCKET=stop.sock",
			)
			output, err := cmd.CombinedOutput()
			if logArg != "" {
				_, statErr := os.Stat(logPath)
				require.NoError(t, statErr)
			}
			if strings.HasPrefix(scenario, "success") {
				require.NoError(t, err, string(output))
				pidText, err := os.ReadFile(marker)
				require.NoError(t, err)
				pid, err := strconv.Atoi(string(pidText))
				require.NoError(t, err)
				require.Positive(t, pid)
				if pidArg != "" {
					pidFile, err := os.ReadFile(pidPath)
					require.NoError(t, err)
					require.Equal(t, string(pidText), string(pidFile))
				}
				running := true
				stop := func() error {
					stopCmd := exec.Command(os.Args[0], "-test.run=^TestDaemonStopHelper$")
					stopCmd.Dir = dir
					stopCmd.Env = append(os.Environ(), "GO_WANT_DO_RESTORE_HELPER=1", "NEO_DAEMON_STOP=1")
					return stopCmd.Run()
				}
				t.Cleanup(func() {
					if running {
						_ = stop()
					}
				})
				require.NoError(t, stop())
				require.Eventually(t, func() bool {
					_, err := os.Stat(marker + ".done")
					return err == nil
				}, 10*time.Second, 10*time.Millisecond)
				running = false
				if pidArg != "" {
					_, err := os.Stat(pidPath)
					require.True(t, os.IsNotExist(err), "%v", err)
				}
			} else {
				require.Error(t, err, string(output))
				if scenario == "existing" {
					data, readErr := os.ReadFile(pidPath)
					require.NoError(t, readErr)
					require.Equal(t, "other process", string(data))
				} else {
					_, statErr := os.Stat(pidPath)
					require.True(t, os.IsNotExist(statErr), "%v", statErr)
				}
			}
		})
	}
}

func TestPIDFileOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	remove, err := createPIDFile(path, nil)
	require.NoError(t, err)
	_, err = createPIDFile(path, nil)
	require.Error(t, err)
	replacement := filepath.Join(filepath.Dir(path), "replacement")
	require.NoError(t, os.WriteFile(replacement, []byte("replacement"), 0o644))
	require.NoError(t, os.Rename(replacement, path))
	require.ErrorContains(t, remove(), "was replaced")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "replacement", string(data))
}
