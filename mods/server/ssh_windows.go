//go:build windows

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

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/v8/mods/util/conpty"
)

func (svr *sshd) shellHandler(ss ssh.Session) {
	user, shell, _ := svr.findShell(ss)
	svr.log.Debugf("session open %s from %s", user, ss.RemoteAddr())

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
