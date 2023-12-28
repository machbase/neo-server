//go:build windows
// +build windows

package args

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/booter"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const svcName = "machbase-neo"

func doService(sc *Service) {
	inService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Println("fail to determine if process is in service:", err.Error())
		return
	}
	if inService {
		runService(svcName, false, sc.Args[1:]...)
		return
	}

	if len(sc.Args) == 0 {
		fmt.Println("Usage: machbase-neo service [install, remove, debug, start, stop]")
		return
	}

	var cmd = strings.ToLower(sc.Args[0])
	switch cmd {
	case "debug":
		runService(svcName, true, sc.Args[1:]...)
		return
	case "install":
		err = installService(svcName, "machbase-neo service", sc.Args[1:]...)
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	default:
		fmt.Println("unknown command:", sc.Args[0])
		fmt.Println("Usage: machbase-neo service [install, remove, debug, start, stop, pause, continue]")
		return
	}
	if err != nil {
		fmt.Println("fail to", cmd, svcName, "service,", err.Error())
	} else {
		fmt.Println("success to", cmd, svcName, "service.")
	}
}

func installService(name string, desc string, args ...string) error {
	exepath, err := os.Executable()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	conf := mgr.Config{
		Description:      desc,
		DelayedAutoStart: true,
	}

	// do not modify this first args
	baseArgs := []string{"service", "run", exepath, "serve"}
	// pass to service from as args
	if len(args) == 0 {
		args = append(baseArgs, []string{
			"--host", "0.0.0.0",
			"--log-filename", filepath.Join(filepath.Dir(exepath), "machbase-neo.log"),
			"--log-level", "TRACE",
		}...)
	} else {
		args = append(baseArgs, args...)
	}

	s, err = m.CreateService(name, exepath, conf, args...)
	if err != nil {
		return err
	}
	defer s.Close()

	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("fail SetupEventLogSource(): %s", err.Error())
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("fail RemoveEventSource(): %s", err.Error())
	}
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start()
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

var elog debug.Log

func runService(name string, debugMode bool, args ...string) {
	var err error
	if debugMode {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	run := svc.Run
	if debugMode {
		run = debug.Run
	}
	cli, err := ParseCommand(append([]string{"machbase-neo", "serve"}, args...))
	if err != nil {
		elog.Warning(1, err.Error())
		return
	}

	elog.Info(1, fmt.Sprintf("%s service starting", name))
	err = run(name, &proxyService{args: args, preset: cli.Serve.Preset})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
	}
	elog.Info(1, fmt.Sprintf("%s service stopped ", name))
}

type proxyService struct {
	args   []string
	preset string
}

func (m *proxyService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepts = svc.AcceptStop | svc.AcceptShutdown /*| svc.AcceptPauseAndContinue */
	elog.Info(1, fmt.Sprintf("running... %v", m.args))
	changes <- svc.Status{State: svc.StartPending}

	os.Args = m.args
	serveWg := sync.WaitGroup{}
	serveWg.Add(1)
	go func() {
		changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepts}
		doServe(m.preset, true)
		serveWg.Done()
	}()
loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			booter.NotifySignal()
			changes <- svc.Status{State: svc.StopPending}
			elog.Info(1, "shutting down...")
			serveWg.Wait()
			break loop
		case svc.Pause:
			changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepts}
		case svc.Continue:
			changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepts}
		default:
			elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}
