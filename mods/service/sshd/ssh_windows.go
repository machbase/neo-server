//go:build windows

package sshd

import (
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/gliderlabs/ssh"
)

func (svr *sshd) shellHandler(ss ssh.Session) {
	svr.log.Infof("session open %s from %s", ss.User(), ss.RemoteAddr())
	shell := svr.shellProvider(ss.User())
	if shell == nil {
		io.WriteString(ss, "No Shell configured.\n")
		ss.Exit(1)
		return
	}
	cmd := exec.Command(shell.Cmd, shell.Args...)
	io.WriteString(ss, svr.motdProvider(ss.User()))
	cmd.Env = append(cmd.Env, "TERM=VT100")
	for k, v := range shell.Envs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		pin, err := cmd.StdinPipe()
		wg.Done()
		if err != nil {
			svr.log.Warnf("session stdin pipe %s", err.Error())
			return
		}
		_, err = io.Copy(pin, ss) // session -> stdin
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
	wg.Add(1)
	go func() {
		pout, err := cmd.StdoutPipe()
		wg.Done()
		if err != nil {
			svr.log.Warnf("session stdout pipe %s", err.Error())
			return
		}

		_, err = io.Copy(ss, pout) // stdout -> session
		if err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
			svr.log.Warnf("session pull %s", err.Error())
		}
	}()
	// wait stdin, stdout pipes before Start()
	wg.Wait()
	err := cmd.Start()

	// register child process after Start()
	svr.addChild(cmd.Process)
	defer func() {
		svr.removeChild(cmd.Process)
	}()

	if err != nil {
		io.WriteString(ss, "No Shell started.\n")
		ss.Exit(1)
		return
	}
	cmd.Wait()
	svr.log.Infof("session close %s from %s '%v' ", ss.User(), ss.RemoteAddr(), cmd.ProcessState)
}
