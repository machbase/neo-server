//go:build windows
// +build windows

package server

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods/util/conpty"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const svcName = "machbase-neo"

func doService(sc *Service) {
	if sc == nil {
		fmt.Println("Usage: machbase-neo service [install, remove, debug, start, stop]")
		return
	}

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
		doServe(m.preset, false, true)
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

func (svr *sshd) shellHandler(ss ssh.Session) {
	user, shell, cleanupSecret := svr.findAndConfigureShell(ss)
	defer cleanupSecret()

	if shell == nil {
		io.WriteString(ss, "No Shell configured.\n")
		ss.Exit(1)
		return
	}

	ptyReq, winCh, isPty := ss.Pty()
	if !isPty {
		// If the user is sys and the pty is not requested, use the system shell.
		if strings.ToLower(user) != "sys" {
			io.WriteString(ss, "PTY is required.\n")
			ss.Exit(1)
			return
		}
		svr.shellHandlerNoPty(ss, user)
		return
	}

	io.WriteString(ss, svr.motdProvider(user))
	cpty, err := conpty.New(int16(ptyReq.Window.Width), int16(ptyReq.Window.Height))
	if err != nil {
		io.WriteString(ss, fmt.Sprintf("Fail to create ConPTY: %s", err.Error()))
		ss.Exit(1)
		return
	}
	defer cpty.Close()

	shell.Envs = append(shell.Envs, fmt.Sprintf("TERM=%s", ptyReq.Term))

	go func() {
		for win := range winCh {
			cpty.Resize(uint16(win.Width), uint16(win.Height))
		}
	}()

	var process *os.Process

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		pin := cpty.InPipe()
		wg.Done()
		if err != nil {
			svr.log.Warnf("session stdin pipe %s", err.Error())
			return
		}
		var w io.Writer
		if svr.dumpInput {
			w = NewIODebugger(svr.log, "RECV:")
		} else {
			w = pin
		}
		_, err = io.Copy(w, ss) // session -> stdin
		if err != nil {
			svr.log.Warnf("session push %s", err.Error())
		}
		// At the time the session closed by exceeding IdleTimeout,
		// First, this go-routine's io.Copy() returned.
		// Then the shell process should be killed by force
		// so that io.Copy() below can be returned and release go-routine and resources.
		//
		// If we do not EXPLICITLY kill the process here, the go-routine below's io.Copy(ss,fn) keep remaining
		// and cmd.Wait() is blocked, which leads shell processes will be cumulated on the OS.
		if process != nil {
			process.Kill()
		}
	}()
	wg.Add(1)
	go func() {
		pout := cpty.OutPipe()
		wg.Done()
		if err != nil {
			svr.log.Warnf("session stdout pipe %s", err.Error())
			return
		}
		var w io.Writer
		if svr.dumpOutput {
			w = io.MultiWriter(ss, NewIODebugger(svr.log, "SEND:"))
		} else {
			w = ss
		}
		_, err = io.Copy(w, pout) // stdout -> session
		if err != nil {
			svr.log.Warnf("session pull %s", err.Error())
		}
	}()
	// wait stdin, stdout pipes before Start()
	wg.Wait()

	path := shell.Cmd
	argv := []string{filepath.Base(path)}
	argv = append(argv, shell.Args...)
	pid, _, err := cpty.Spawn(path, argv, &syscall.ProcAttr{Env: shell.Envs})
	if err != nil {
		svr.log.Errorf("ConPty spawn: %s", err.Error())
		ss.Exit(1)
		return
	}
	process, err = os.FindProcess(pid)
	if err != nil {
		svr.log.Errorf("Failed to find process: %s", err.Error())
		ss.Exit(1)
		return
	}

	// register child process after Start()
	svr.addChild(process)
	defer func() {
		svr.removeChild(process)
	}()

	ps, err := process.Wait()
	if err != nil {
		svr.log.Infof("session terminated %s from %s %s", user, ss.RemoteAddr(), err.Error())
		return
	}
	svr.log.Debugf("session close %s from %s '%v' ", user, ss.RemoteAddr(), ps)
}

func (svr *sshd) shellHandlerNoPty(ss ssh.Session, user string) {
	cmd := exec.Command("cmd.exe")
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdin = ss
	cmd.Stdout = ss
	cmd.Stderr = ss
	wg := sync.WaitGroup{}

	if err := cmd.Start(); err != nil {
		svr.log.Warnf("shell error, %s", err.Error())
		ss.Exit(-1)
		return
	}

	svr.addChild(cmd.Process)
	wg.Add(1)
	go func() {
		svr.log.Tracef("shell cmd waiting pid %d", cmd.Process.Pid)
		cmd.Wait()
		svr.log.Tracef("shell cmd waiting done.")
		svr.removeChild(cmd.Process)
		ss.Exit(0)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		svr.log.Tracef("shell session ctx waiting")
		<-ss.Context().Done()
		ss.Close()
		svr.log.Tracef("shell session ctx closed")
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		wg.Done()
	}()
	wg.Wait()
	svr.log.Debugf("shell close %s from %s '%v'", user, ss.RemoteAddr(), cmd.ProcessState)
}
