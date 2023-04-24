//go:build windows

package sshd

import (
	"fmt"
	"io"
	"os"
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
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		userHomeDir = "."
	}
	cmd.Env = append(cmd.Env, "USERPROFILE="+userHomeDir)
	cmd.Env = append(cmd.Env, "TERM=VT100")
	cmd.Env = append(cmd.Env, "NEOSHELL_KEEP_STDIN=1")
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

	err = cmd.Start()
	if err != nil {
		svr.log.Infof("session terminated %s from %s %s", ss.User(), ss.RemoteAddr(), err.Error())
		io.WriteString(ss, "No Shell started.\n")
		ss.Exit(1)
		return
	}

	// register child process after Start()
	svr.addChild(cmd.Process)
	defer func() {
		svr.removeChild(cmd.Process)
	}()

	cmd.Wait()
	svr.log.Infof("session close %s from %s '%v' ", ss.User(), ss.RemoteAddr(), cmd.ProcessState)
}
