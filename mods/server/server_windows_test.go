//go:build windows

package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/svc"
)

type serviceTestLog struct {
	errors []string
}

func (*serviceTestLog) Close() error                 { return nil }
func (*serviceTestLog) Info(uint32, string) error    { return nil }
func (*serviceTestLog) Warning(uint32, string) error { return nil }
func (l *serviceTestLog) Error(_ uint32, msg string) error {
	l.errors = append(l.errors, msg)
	return nil
}

type serviceResult struct {
	specific bool
	code     uint32
}

func runTestService(t *testing.T, service *proxyService) (chan svc.ChangeRequest, chan svc.Status, chan serviceResult, *serviceTestLog) {
	t.Helper()
	oldLog := elog
	testLog := &serviceTestLog{}
	elog = testLog
	t.Cleanup(func() { elog = oldLog })
	requests := make(chan svc.ChangeRequest, 1)
	statuses := make(chan svc.Status, 4)
	result := make(chan serviceResult, 1)
	go func() {
		specific, code := service.Execute(nil, requests, statuses)
		result <- serviceResult{specific, code}
	}()
	require.Equal(t, svc.StartPending, receiveServiceStatus(t, statuses).State)
	return requests, statuses, result, testLog
}

func receiveServiceStatus(t *testing.T, statuses <-chan svc.Status) svc.Status {
	t.Helper()
	select {
	case status := <-statuses:
		return status
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for service status")
		return svc.Status{}
	}
}

func receiveServiceResult(t *testing.T, result <-chan serviceResult) serviceResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for service exit")
		return serviceResult{}
	}
}

func TestWindowsServiceReadyAndStop(t *testing.T) {
	for _, command := range []svc.Cmd{svc.Stop, svc.Shutdown} {
		t.Run(commandName(command), func(t *testing.T) {
			allowStart := make(chan struct{})
			allowStop := make(chan struct{})
			stopping := make(chan struct{})
			stopRequest := make(chan struct{}, 1)
			service := &proxyService{
				stopRequest: stopRequest,
				serve: func(ready func() error) error {
					<-allowStart
					if err := ready(); err != nil {
						return err
					}
					<-stopRequest
					close(stopping)
					<-allowStop
					return nil
				},
			}
			requests, statuses, result, _ := runTestService(t, service)
			select {
			case status := <-statuses:
				t.Fatalf("premature status: %+v", status)
			default:
			}
			close(allowStart)
			running := receiveServiceStatus(t, statuses)
			require.Equal(t, svc.Running, running.State)
			require.Equal(t, svc.AcceptStop|svc.AcceptShutdown, running.Accepts)
			requests <- svc.ChangeRequest{Cmd: command}
			require.Equal(t, svc.StopPending, receiveServiceStatus(t, statuses).State)
			select {
			case <-stopping:
			case <-time.After(5 * time.Second):
				t.Fatal("stop was not delivered to server")
			}
			select {
			case got := <-result:
				t.Fatalf("returned before server cleanup: %+v", got)
			default:
			}
			close(allowStop)
			require.Equal(t, serviceResult{}, receiveServiceResult(t, result))
			select {
			case status := <-statuses:
				t.Fatalf("unexpected final status: %+v", status)
			default:
			}
		})
	}
}

func commandName(command svc.Cmd) string {
	if command == svc.Stop {
		return "stop"
	}
	return "shutdown"
}

func TestWindowsServiceStartupFailure(t *testing.T) {
	service := &proxyService{
		stopRequest: make(chan struct{}, 1),
		serve: func(func() error) error {
			return errors.New("configuration file missing")
		},
	}
	_, statuses, result, testLog := runTestService(t, service)
	require.Equal(t, serviceResult{specific: true, code: 1}, receiveServiceResult(t, result))
	require.Contains(t, strings.Join(testLog.errors, "\n"), "configuration file missing")
	select {
	case status := <-statuses:
		t.Fatalf("unexpected status after failed startup: %+v", status)
	default:
	}
}

func TestWindowsServiceInvalidBootOption(t *testing.T) {
	processArgs := []string{"neo.exe", "service", "run", "neo.exe", "serve", "--config"}
	service := newProxyService(processArgs, processArgs[3:])
	_, statuses, result, testLog := runTestService(t, service)
	require.Equal(t, serviceResult{specific: true, code: 1}, receiveServiceResult(t, result))
	require.Contains(t, strings.Join(testLog.errors, "\n"), "--config requires a value")
	select {
	case status := <-statuses:
		t.Fatalf("unexpected status after failed startup: %+v", status)
	default:
	}
}

func TestWindowsServiceStopDuringStartup(t *testing.T) {
	allowStart := make(chan struct{})
	stopRequest := make(chan struct{}, 1)
	service := &proxyService{
		stopRequest: stopRequest,
		serve: func(ready func() error) error {
			<-allowStart
			if err := ready(); err != nil {
				return err
			}
			<-stopRequest
			return nil
		},
	}
	requests, statuses, result, _ := runTestService(t, service)
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	require.Equal(t, svc.StopPending, receiveServiceStatus(t, statuses).State)
	close(allowStart)
	require.Equal(t, serviceResult{}, receiveServiceResult(t, result))
	select {
	case status := <-statuses:
		t.Fatalf("unexpected status after stop: %+v", status)
	default:
	}
}

func TestWindowsServiceShutdownFailure(t *testing.T) {
	stopRequest := make(chan struct{}, 1)
	service := &proxyService{
		stopRequest: stopRequest,
		serve: func(ready func() error) error {
			if err := ready(); err != nil {
				return err
			}
			<-stopRequest
			return errors.New("shutdown cleanup failed")
		},
	}
	requests, statuses, result, testLog := runTestService(t, service)
	require.Equal(t, svc.Running, receiveServiceStatus(t, statuses).State)
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	require.Equal(t, svc.StopPending, receiveServiceStatus(t, statuses).State)
	require.Equal(t, serviceResult{specific: true, code: 1}, receiveServiceResult(t, result))
	require.Contains(t, strings.Join(testLog.errors, "\n"), "shutdown cleanup failed")
}

func TestWindowsServiceSpontaneousShutdown(t *testing.T) {
	stopRequest := make(chan struct{}, 1)
	service := &proxyService{
		stopRequest: stopRequest,
		serve: func(ready func() error) error {
			return ready()
		},
	}
	_, statuses, result, _ := runTestService(t, service)
	got := receiveServiceStatus(t, statuses)
	if got.State == svc.Running {
		got = receiveServiceStatus(t, statuses)
	}
	require.Equal(t, svc.StopPending, got.State)
	require.Equal(t, serviceResult{}, receiveServiceResult(t, result))
	select {
	case status := <-statuses:
		t.Fatalf("unexpected final status: %+v", status)
	default:
	}
}

func TestWindowsServicePreservesConfigArguments(t *testing.T) {
	for _, tt := range []struct {
		name       string
		expression string
		want       string
	}{
		{"executable", `arg(0, "missing")`, `C:\neo\machbase-neo.exe`},
		{"command", `arg(1, "missing")`, "service"},
		{"subcommand", `arg(2, "missing")`, "run"},
		{"count", `"${arglen()}"`, "7"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "server.conf")
			content := `module "machbase.com/neo-server" {
  config { DataDir = flagOrError(` + tt.expression + `) }
}`
			require.NoError(t, os.WriteFile(configFile, []byte(content), 0o600))
			processArgs := []string{`C:\neo\machbase-neo.exe`, "service", "run", `C:\neo\machbase-neo.exe`, "serve", "--config", configFile}
			service := newProxyService(processArgs, processArgs[3:])
			err := service.serve(func() error {
				t.Error("server should not start with invalid configuration")
				return nil
			})
			require.ErrorContains(t, err, "required flag "+tt.want+" missing")
		})
	}
}
