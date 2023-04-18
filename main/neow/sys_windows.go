//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/machbase/booter"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func WindowsServe() error {
	ws := &winService{
		name:        "machbase-neo",
		description: "machbase-neo serve",
	}
	// service called from windows service manager
	// use API provided by golang.org/x/sys/windows
	err := svc.Run(ws.name, &winServe{})
	if err != nil {
		return getWindowsError(err)
	}
	return nil
}

type winServe struct {
}

func (svr *winServe) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}

	fasttick := time.Tick(500 * time.Millisecond)
	slowtick := time.Tick(2 * time.Second)
	tick := fasttick

	// go doServe()
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case <-tick:
			break
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				booter.Shutdown()
				// sh.executable.Stop()
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				tick = slowtick
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				tick = fasttick
			default:
				continue loop
			}
		}
	}
	return
}

type winService struct {
	name         string
	description  string
	dependencies []string
}

var ErrAlreadyRunning = errors.New("already running")

func (ws *winService) Install(args ...string) (string, error) {
	installAction := "Install " + ws.description + ":"
	execp, err := execPath()
	if err != nil {
		return installAction + " failed", err
	}
	m, err := mgr.Connect()
	if err != nil {
		return installAction + " failed", err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ws.name)
	if err == nil {
		s.Close()
		return installAction + " failed", ErrAlreadyRunning
	}

	s, err = m.CreateService(ws.name, execp, mgr.Config{
		DisplayName:  ws.name,
		Description:  ws.description,
		StartType:    mgr.StartAutomatic,
		Dependencies: ws.dependencies,
	}, args...)
	if err != nil {
		return installAction + " failed", err
	}
	defer s.Close()

	// set recovery
	// restart after 10 seconds for the first 3 times
	// restart after 1 minute, otherwise
	r := []mgr.RecoveryAction{
		{
			Type:  mgr.ServiceRestart,
			Delay: 10 * time.Second,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 10 * time.Second,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 10 * time.Second,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 60 * time.Second,
		},
	}
	s.SetRecoveryActions(r, uint32(86400))
	return installAction + " completed.", nil
}

func (ws *winService) Remove(args ...string) (string, error) {
	removeAction := "Removing " + ws.description + ":"
	m, err := mgr.Connect()
	if err != nil {
		return removeAction + " failed", getWindowsError(err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(ws.name)
	if err != nil {
		return removeAction + " failed", getWindowsError(err)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return removeAction + " failed", getWindowsError(err)
	}
	return removeAction + " completed.", nil
}

func (ws *winService) Start() (string, error) {
	startAction := "Starting " + ws.description + ":"
	m, err := mgr.Connect()
	if err != nil {
		return startAction + " failed", getWindowsError(err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(ws.name)
	if err != nil {
		return startAction + " failed", getWindowsError(err)
	}
	defer s.Close()
	if err = s.Start(); err != nil {
		return startAction + " failed", getWindowsError(err)
	}
	return startAction + " completed.", nil
}

func (ws *winService) Stop() (string, error) {
	stopAction := "Stopping " + ws.description + ":"
	m, err := mgr.Connect()
	if err != nil {
		return stopAction + " failed", getWindowsError(err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(ws.name)
	if err != nil {
		return stopAction + " failed", getWindowsError(err)
	}
	defer s.Close()
	if err := stopAndWait(s); err != nil {
		return stopAction + " failed", getWindowsError(err)
	}
	return stopAction + " completed.", nil
}

func stopAndWait(s *mgr.Service) error {
	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}
	timeDuration := time.Millisecond * 50
	timeout := time.After(getStopTimeout() + (timeDuration * 2))
	tick := time.NewTicker(timeDuration)
	defer tick.Stop()
	for status.State != svc.Stopped {
		select {
		case <-tick.C:
			status, err = s.Query()
			if err != nil {
				return err
			}
		case <-timeout:
			break
		}
	}
	return nil
}

func getStopTimeout() time.Duration {
	// For default and paths see https://support.microsoft.com/en-us/kb/146092
	defaultTimeout := time.Millisecond * 20000
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control`, registry.READ)
	if err != nil {
		return defaultTimeout
	}
	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return defaultTimeout
	}
	v, err := strconv.Atoi(sv)
	if err != nil {
		return defaultTimeout
	}
	return time.Millisecond * time.Duration(v)
}

func (ws *winService) Status() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "Getting status: failed", getWindowsError(err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(ws.name)
	if err != nil {
		return "Getting status: failed", getWindowsError(err)
	}
	defer s.Close()
	status, err := s.Query()
	if err != nil {
		return "Getting status: failed", getWindowsError(err)
	}
	return "Status: " + getWindowsServiceStateFromUint32(status.State), nil
}

// Get executable path
func execPath() (string, error) {
	var n uint32
	b := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(b))

	r0, _, e1 := syscall.MustLoadDLL(
		"kernel32.dll",
	).MustFindProc(
		"GetModuleFileNameW",
	).Call(0, uintptr(unsafe.Pointer(&b[0])), uintptr(size))
	n = uint32(r0)
	if n == 0 {
		return "", e1
	}
	return string(utf16.Decode(b[0:n])), nil
}

func getWindowsServiceStateFromUint32(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "SERVICE_STOPPED"
	case svc.StartPending:
		return "SERVICE_START_PENDING"
	case svc.StopPending:
		return "SERVICE_STOP_PENDING"
	case svc.Running:
		return "SERVICE_RUNNING"
	case svc.ContinuePending:
		return "SERVICE_CONTINUE_PENDING"
	case svc.PausePending:
		return "SERVICE_PAUSE_PENDING"
	case svc.Paused:
		return "SERVICE_PAUSED"
	}
	return "SERVICE_UNKNOWN"
}

func getWindowsError(inputError error) error {
	if exiterr, ok := inputError.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			if sysErr, ok := WinErrCode[status.ExitStatus()]; ok {
				return errors.New(fmt.Sprintf("\n %s: %s \n %s", sysErr.Title, sysErr.Description, sysErr.Action))
			}
		}
	}
	return inputError
}

// SystemError contains error description and corresponded action helper to fix it
type SystemError struct {
	Title       string
	Description string
	Action      string
}

var (
	// WinErrCode - List of system errors from Microsoft source:
	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681385(v=vs.85).aspx
	WinErrCode = map[int]SystemError{
		5: {
			Title:       "ERROR_ACCESS_DENIED",
			Description: "Access denied.",
			Action:      "Administrator access is needed to install a service.",
		},
		1051: {
			Title:       "ERROR_DEPENDENT_SERVICES_RUNNING",
			Description: "A stop control has been sent to a service that other running services are dependent on.",
		},
		1052: {
			Title:       "ERROR_INVALID_SERVICE_CONTROL",
			Description: "The requested control is not valid for this service.",
		},
		1053: {
			Title:       "ERROR_SERVICE_REQUEST_TIMEOUT",
			Description: "The service did not respond to the start or control request in a timely fashion.",
		},
		1054: {
			Title:       "ERROR_SERVICE_NO_THREAD",
			Description: "A thread could not be created for the service.",
		},
		1055: {
			Title:       "ERROR_SERVICE_DATABASE_LOCKED",
			Description: "The service database is locked.",
		},
		1056: {
			Title:       "ERROR_SERVICE_ALREADY_RUNNING",
			Description: "An instance of the service is already running.",
		},
		1057: {
			Title:       "ERROR_INVALID_SERVICE_ACCOUNT",
			Description: "The account name is invalid or does not exist, or the password is invalid for the account name specified.",
		},
		1058: {
			Title:       "ERROR_SERVICE_DISABLED",
			Description: "The service cannot be started, either because it is disabled or because it has no enabled devices associated with it.",
		},
		1060: {
			Title:       "ERROR_SERVICE_DOES_NOT_EXIST",
			Description: "The specified service does not exist as an installed service.",
		},
		1061: {
			Title:       "ERROR_SERVICE_CANNOT_ACCEPT_CTRL",
			Description: "The service cannot accept control messages at this time.",
		},
		1062: {
			Title:       "ERROR_SERVICE_NOT_ACTIVE",
			Description: "The service has not been started.",
		},
		1063: {
			Title:       "ERROR_FAILED_SERVICE_CONTROLLER_CONNECT",
			Description: "The service process could not connect to the service controller.",
		},
		1064: {
			Title:       "ERROR_EXCEPTION_IN_SERVICE",
			Description: "An exception occurred in the service when handling the control request.",
		},
		1066: {
			Title:       "ERROR_SERVICE_SPECIFIC_ERROR",
			Description: "The service has returned a service-specific error code.",
		},
		1068: {
			Title:       "ERROR_SERVICE_DEPENDENCY_FAIL",
			Description: "The dependency service or group failed to start.",
		},
		1069: {
			Title:       "ERROR_SERVICE_LOGON_FAILED",
			Description: "The service did not start due to a logon failure.",
		},
		1070: {
			Title:       "ERROR_SERVICE_START_HANG",
			Description: "After starting, the service hung in a start-pending state.",
		},
		1071: {
			Title:       "ERROR_INVALID_SERVICE_LOCK",
			Description: "The specified service database lock is invalid.",
		},
		1072: {
			Title:       "ERROR_SERVICE_MARKED_FOR_DELETE",
			Description: "The specified service has been marked for deletion.",
		},
		1073: {
			Title:       "ERROR_SERVICE_EXISTS",
			Description: "The specified service already exists.",
		},
		1075: {
			Title:       "ERROR_SERVICE_DEPENDENCY_DELETED",
			Description: "The dependency service does not exist or has been marked for deletion.",
		},
		1077: {
			Title:       "ERROR_SERVICE_NEVER_STARTED",
			Description: "No attempts to start the service have been made since the last boot.",
		},
		1078: {
			Title:       "ERROR_DUPLICATE_SERVICE_NAME",
			Description: "The name is already in use as either a service name or a service display name.",
		},
		1079: {
			Title:       "ERROR_DIFFERENT_SERVICE_ACCOUNT",
			Description: "The account specified for this service is different from the account specified for other services running in the same process.",
		},
		1083: {
			Title:       "ERROR_SERVICE_NOT_IN_EXE",
			Description: "The executable program that this service is configured to run in does not implement the service.",
		},
		1084: {
			Title:       "ERROR_NOT_SAFEBOOT_SERVICE",
			Description: "This service cannot be started in Safe Mode.",
		},
	}
)
