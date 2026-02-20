//go:build !windows

package server

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/v8/mods/model"
)

const useAlternativeNoPtyShell = false

func (svr *sshd) shellHandler(ss ssh.Session) {
	user, shell, shellId := svr.findShell(ss)
	svr.log.Debugf("shell open %s from %s", user, ss.RemoteAddr())

	if shell == nil {
		io.WriteString(ss, "No Shell configured.\n")
		ss.Exit(1)
		return
	}

	ptyReq, winCh, isPty := ss.Pty()

	if !isPty && strings.ToLower(user) == "sys" {
		if !useAlternativeNoPtyShell {
			io.WriteString(ss, "No PTY configured.\n")
			ss.Exit(1)
			return
		}
		// If the user is sys and the pty is not requested, use the system shell.
		if osShell := os.Getenv("SHELL"); osShell != "" {
			shell.Cmd = osShell
			shell.Args = []string{}
		} else {
			shell.Cmd = "/bin/sh"
			shell.Args = []string{}
			shell.Envs = append(shell.Envs, "SHELL="+shell.Cmd)
		}
		filtered := []string{}
		for _, env := range shell.Envs {
			if strings.HasPrefix(env, "TERM") || strings.Contains(env, "TERM=") {
				continue
			}
			filtered = append(filtered, env)
		}
		shell.Envs = filtered
	} else {
		if shellId == model.SHELLID_SHELL || shellId == model.SHELLID_JSH {
			shell.Envs = append(shell.Envs, fmt.Sprintf("NEOSHELL_USER=%s", user))
			shell.Envs = append(shell.Envs, fmt.Sprintf("NEOSHELL_PASSWORD=%s", svr.authServer.neoShellAccount[strings.ToLower(user)]))
		}
	}

	cmd := exec.Command(shell.Cmd, shell.Args...)
	cmd.Env = append(cmd.Env, shell.Envs...)

	if isPty {
		io.WriteString(ss, svr.motdProvider(user))
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))

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
			// At the time the session closed by exceeding IdleTimeout,
			// First, this go-routine's io.Copy() returned.
			// Then the shell process should be killed by force
			// so that io.Copy() below can be returned and release go-routine and resources.
			//
			// If we do not EXPLICITLY kill the process here, the go-routine below's io.Copy(ss,fn) keep remaining
			// and cmd.Wait() is blocked, which leads shell processes will be cumulated on the OS.
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
	} else {
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			io.Copy(stdin, ss)
			svr.log.Tracef("shell stdin closed")
			stdin.Close()
			wg.Done()
		}()
		wg.Add(1)
		go func() {
			io.Copy(ss, stdout)
			svr.log.Tracef("shell stdout closed")
			wg.Done()
		}()
		wg.Add(1)
		go func() {
			io.Copy(ss, stderr)
			svr.log.Tracef("shell stderr closed")
			wg.Done()
		}()

		// child process should be terminated when the parent process is terminated.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
			stdout.Close()
			stderr.Close()
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			wg.Done()
		}()
		wg.Wait()
	}
	svr.log.Debugf("shell close %s from %s '%v'", user, ss.RemoteAddr(), cmd.ProcessState)
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}
