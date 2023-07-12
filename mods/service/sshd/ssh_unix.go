//go:build !windows

package sshd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
)

func (svr *sshd) shellHandler(ss ssh.Session) {
	user, shellDef := svr.findShellDefinition(ss)
	if shellDef != nil {
		svr.log.Debugf("session open %s (%s) from %s", user, shellDef.Name, ss.RemoteAddr())
	} else {
		svr.log.Debugf("session open %s from %s", user, ss.RemoteAddr())
	}

	shell := svr.buildShell(user, shellDef)
	if shell == nil {
		io.WriteString(ss, "No Shell configured.\n")
		ss.Exit(1)
		return
	}

	cmd := exec.Command(shell.Cmd, shell.Args...)
	ptyReq, winCh, isPty := ss.Pty()
	if !isPty {
		io.WriteString(ss, "No PTY requested.\n")
		ss.Exit(1)
		return
	}
	io.WriteString(ss, svr.motdProvider(user))
	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	for k, v := range shell.Envs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	fn, err := pty.Start(cmd)
	if err != nil {
		io.WriteString(ss, "No PTY started.\n")
		ss.Exit(1)
		return
	}
	svr.addChild(cmd.Process)
	defer func() {
		svr.removeChild(cmd.Process)
		fn.Close()
	}()

	go func() {
		for win := range winCh {
			setWinsize(fn, win.Width, win.Height)
		}
	}()
	go func() {
		var w io.Writer
		if svr.dumpInput {
			w = io.MultiWriter(fn, NewIODebugger(svr.log, "RECV:"))
		} else {
			w = fn
		}
		_, err := io.Copy(w, ss) // session -> stdin
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
		cmd.Process.Kill()
	}()
	go func() {
		var w io.Writer
		if svr.dumpOutput {
			w = io.MultiWriter(ss, NewIODebugger(svr.log, "SEND:"))
		} else {
			w = ss
		}
		_, err := io.Copy(w, fn) // stdout -> session
		if err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
			svr.log.Warnf("session pull %s", err.Error())
		}
	}()
	cmd.Wait()
	svr.log.Debugf("session close %s from %s '%v' ", user, ss.RemoteAddr(), cmd.ProcessState)
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}
