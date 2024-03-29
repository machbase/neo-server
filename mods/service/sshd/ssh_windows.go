//go:build windows

package sshd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/mods/util/conpty"
)

func (svr *sshd) shellHandler(ss ssh.Session) {
	user, shell := svr.findShell(ss)
	svr.log.Debugf("session open %s from %s", user, ss.RemoteAddr())

	if shell == nil {
		io.WriteString(ss, "No Shell configured.\n")
		ss.Exit(1)
		return
	}

	ptyReq, winCh, isPty := ss.Pty()
	if !isPty {
		io.WriteString(ss, "No PTY requested.\n")
		ss.Exit(1)
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

	if _, ok := shell.Envs["TERM"]; !ok {
		shell.Envs["TERM"] = ptyReq.Term
	}

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
		// At the time the session closed by exceeding Idletimeout,
		// First, this go-routine's io.Copy() returned.
		// Then the shell process should be killed by force
		// so that io.Copy() below can be returned and relase go-routine and resources.
		//
		// If we do not EXPLICITLY kill the process here, the go-routine below's io.Copy(ss,fn) keep remaining
		// and cmd.Wait() is blocked, which leads shell processes will be cummulated on the OS.
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
	env := []string{}
	for k, v := range shell.Envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	pid, _, err := cpty.Spawn(path, argv, &syscall.ProcAttr{Env: env})
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
