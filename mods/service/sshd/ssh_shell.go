package sshd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
)

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

func (svr *sshd) shellHandler(ss ssh.Session) {
	svr.log.Infof("session open %s from %s", ss.User(), ss.RemoteAddr())
	shell := svr.shellProvider(ss.User())
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
	io.WriteString(ss, svr.motdProvider(ss.User()))
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
		_, err := io.Copy(fn, ss) // session -> stdin
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
		_, err := io.Copy(ss, fn) // stdout -> session
		if err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
			svr.log.Warnf("session pull %s", err.Error())
		}
	}()
	cmd.Wait()
	svr.log.Infof("session close %s from %s '%v' ", ss.User(), ss.RemoteAddr(), cmd.ProcessState)
}
