//go:build windows

package server

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestWindowsSCMHelper(t *testing.T) {
	args := flag.Args()
	if len(args) < 2 || args[0] != "neo-scm-helper" {
		t.Skip("SCM subprocess helper")
	}
	inService, err := svc.IsWindowsService()
	require.NoError(t, err)
	require.True(t, inService)

	executable, err := os.Executable()
	require.NoError(t, err)
	serverArgs := append([]string{executable, "serve"}, args[2:]...)
	processArgs := append([]string{executable, "service", "run"}, serverArgs...)
	runService(args[1], false, processArgs, serverArgs...)
}

// TestWindowsSCMIntegration exercises SCM controls, not operating system shutdown.
func TestWindowsSCMIntegration(t *testing.T) {
	if os.Getenv("CI") != "true" || os.Getenv("RUN_WINDOWS_SCM_TESTS") != "1" {
		t.Skip("requires an isolated Windows CI runner")
	}
	require.Equal(t, "1", os.Getenv("GO_WANT_DO_RESTORE_HELPER"))

	t.Run("startup and stop", func(t *testing.T) {
		dir := t.TempDir()
		ports := reserveSCMTestPorts(t, 4)
		config := string(DefaultFallbackConfig)
		for _, listener := range []string{`"unix://${VARS_HTTP_LISTEN_SOCK}",`, `"unix://${VARS_MQTT_LISTEN_SOCK}",`} {
			require.Contains(t, config, listener)
			config = strings.Replace(config, listener, "", 1)
		}
		configPath := filepath.Join(dir, "server.conf")
		require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
		workspace := filepath.Join(dir, "workspace")
		require.NoError(t, os.Mkdir(workspace, 0o700))

		args := []string{
			"--config", configPath,
			"--data", filepath.Join(dir, "data"),
			"--pref", filepath.Join(dir, "pref"),
			"--file", workspace,
			"--backup-dir", filepath.Join(dir, "backup"),
			"--log-filename", filepath.Join(dir, "server.log"),
			"--mach-port", strconv.Itoa(ports[0]),
			"--http-port", strconv.Itoa(ports[1]),
			"--mqtt-port", strconv.Itoa(ports[2]),
			"--shell-port", strconv.Itoa(ports[3]),
			"--machbase-init-option", "1",
		}
		service := createSCMTestService(t, args)
		require.NoError(t, service.Start())
		status, err := waitSCMTestState(service, svc.Running, 2*time.Minute)
		require.NoError(t, err, "server log: %s", readSCMTestLog(dir))
		require.NotZero(t, status.ProcessId)
		require.Equal(t, svc.AcceptStop|svc.AcceptShutdown, status.Accepts&(svc.AcceptStop|svc.AcceptShutdown))

		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(ports[1])), 2*time.Second)
		require.NoError(t, err)
		require.NoError(t, conn.Close())

		_, err = service.Control(svc.Stop)
		require.NoError(t, err)
		status, err = waitSCMTestState(service, svc.Stopped, 2*time.Minute)
		require.NoError(t, err, "server log: %s", readSCMTestLog(dir))
		require.Zero(t, status.Win32ExitCode)
		require.Zero(t, status.ServiceSpecificExitCode)
	})

	t.Run("startup failure", func(t *testing.T) {
		missingConfig := filepath.Join(t.TempDir(), "missing.conf")
		service := createSCMTestService(t, []string{"--config", missingConfig})
		require.NoError(t, service.Start())
		status, err := waitSCMTestState(service, svc.Stopped, 30*time.Second)
		require.NoError(t, err)
		require.Equal(t, uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR), status.Win32ExitCode)
		require.Equal(t, uint32(1), status.ServiceSpecificExitCode)
	})
}

func createSCMTestService(t *testing.T, serverArgs []string) *mgr.Service {
	t.Helper()
	id := make([]byte, 16)
	_, err := rand.Read(id)
	require.NoError(t, err)
	name := fmt.Sprintf("neo-scm-test-%x", id)
	executable, err := os.Executable()
	require.NoError(t, err)
	manager, err := mgr.Connect()
	require.NoError(t, err)
	args := append([]string{"-test.run=^TestWindowsSCMHelper$", "--", "neo-scm-helper", name}, serverArgs...)
	service, err := manager.CreateService(name, executable, mgr.Config{}, args...)
	if err != nil {
		require.NoError(t, manager.Disconnect())
		require.NoError(t, err)
	}

	eventSourceInstalled := false
	t.Cleanup(func() {
		if err := stopSCMTestService(service, 2*time.Minute); err != nil {
			t.Errorf("stop service %s during cleanup: %v", name, err)
			status, queryErr := service.Query()
			if queryErr != nil {
				t.Errorf("query service %s during cleanup: %v", name, queryErr)
			} else if status.ProcessId != 0 {
				process, killErr := os.FindProcess(int(status.ProcessId))
				if killErr == nil {
					killErr = errors.Join(process.Kill(), process.Release())
				}
				if killErr != nil {
					t.Errorf("terminate service %s: %v", name, killErr)
				} else if _, waitErr := waitSCMTestState(service, svc.Stopped, 10*time.Second); waitErr != nil {
					t.Errorf("wait for terminated service %s: %v", name, waitErr)
				}
			}
		}
		if err := service.Delete(); err != nil {
			t.Errorf("delete service %s: %v", name, err)
		}
		if err := service.Close(); err != nil {
			t.Errorf("close service %s: %v", name, err)
		}
		if eventSourceInstalled {
			if err := eventlog.Remove(name); err != nil {
				t.Errorf("remove event source %s: %v", name, err)
			}
		}
		if err := manager.Disconnect(); err != nil {
			t.Errorf("disconnect SCM: %v", err)
		}
	})
	require.NoError(t, eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info))
	eventSourceInstalled = true
	return service
}

func stopSCMTestService(service *mgr.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	stopRequested := false
	for {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		if status.State == svc.Running && !stopRequested {
			if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
				return err
			}
			stopRequested = true
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service did not stop; last state %v", status.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitSCMTestState(service *mgr.Service, want svc.State, timeout time.Duration) (svc.Status, error) {
	deadline := time.Now().Add(timeout)
	for {
		status, err := service.Query()
		if err != nil {
			return status, err
		}
		if status.State == want {
			return status, nil
		}
		if want == svc.Running && status.State == svc.Stopped {
			return status, fmt.Errorf("service stopped during startup: win32=%d service=%d", status.Win32ExitCode, status.ServiceSpecificExitCode)
		}
		if time.Now().After(deadline) {
			return status, fmt.Errorf("service did not reach %v; last state %v", want, status.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func reserveSCMTestPorts(t *testing.T, count int) []int {
	t.Helper()
	listeners := make([]net.Listener, 0, count)
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	ports := make([]int, 0, count)
	for range count {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports
}

func readSCMTestLog(dir string) string {
	content, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil {
		return err.Error()
	}
	return string(content)
}
