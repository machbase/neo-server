package sshd

import (
	"io"
	"os/exec"
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/errors"
)

func (svr *sshd) commandHandler(ss ssh.Session) {
	shell := svr.shell(ss.User(), "SHELL")
	if shell == nil {
		io.WriteString(ss, "No shell found\n")
		ss.Exit(1)
		return
	}
	cmdArr := []string{shell.Cmd}
	cmdArr = append(cmdArr, shell.Args...)
	cmdArr = append(cmdArr, ss.Command()...)
	if len(cmdArr) == 0 {
		io.WriteString(ss, "Invalid Command\n")
		ss.Exit(1)
		return
	}

	// []string{"scp", "-t", "/data/logs/a.txt"}
	svr.log.Infof("%s command %+v", ss.User(), cmdArr)

	var cmd *exec.Cmd
	if len(cmdArr) > 1 {
		cmd = exec.Command(cmdArr[0], cmdArr[1:]...)
	} else {
		cmd = exec.Command(cmdArr[0])
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		svr.log.Infof("%s could not open stdin pipe %s", ss.User(), err.Error())
		io.WriteString(ss, "Can not open stdin pipe\n")
		ss.Exit(1)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		svr.log.Infof("%s could not open stdout pipe %s", ss.User(), err.Error())
		io.WriteString(ss, "Can not open stdout pipe\n")
		ss.Exit(1)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		svr.log.Infof("%s could not open stderr pipe %s", ss.User(), err.Error())
		io.WriteString(ss, "Can not open stderr pipe\n")
		ss.Exit(1)
		return
	}

	breakChan := make(chan bool)
	sigChan := make(chan ssh.Signal)
	cmdChan := make(chan int)

	ss.Break(breakChan)
	ss.Signals(sigChan)

	var once sync.Once
	onceClose := func() { once.Do(func() { ss.Close() }) }

	go func() {
		_, err := io.Copy(stdin, ss)
		if err != nil {
			svr.log.Infof("%s stdin  err %s", ss.User(), err.Error())
		}
		svr.log.Infof("%s stdin  close", ss.User())
		stdin.Close()
		onceClose()
	}()
	go func() {
		_, err := io.Copy(ss, stdout)
		if err != nil && !errors.Is(err, io.EOF) {
			svr.log.Infof("%s stdout err %s", ss.User(), err.Error())
		}
		svr.log.Infof("%s stdout close", ss.User())
		stdout.Close()
		onceClose()
	}()
	go func() {
		_, err := io.Copy(ss, stderr)
		if err != nil {
			svr.log.Infof("%s stderr err %s", ss.User(), err.Error())
		}
		svr.log.Infof("%s stderr close", ss.User())
		stderr.Close()
		onceClose()
	}()
	go func() {
		err = cmd.Run()
		if err != nil {
			svr.log.Infof("%s command fail %s", ss.User(), err.Error())
			ss.Exit(1)
		}
		if cmd.ProcessState == nil {
			svr.log.Infof("%s command state error:%s", ss.User(), cmd.Err)
			ss.Exit(1)
		}
		exitCode := cmd.ProcessState.ExitCode()
		ss.Exit(exitCode)

		cmdChan <- exitCode
		svr.log.Infof("%s exit %d", ss.User(), exitCode)
		onceClose()
	}()

eventLoop:
	for {
		svr.log.Infof("%s wait command to finish", ss.User())
		select {
		case sig := <-sigChan:
			svr.log.Info("%s signal %d", ss.User(), sig)
			break eventLoop
		case flag := <-breakChan:
			svr.log.Infof("%s break %t", ss.User(), flag)
			break eventLoop
		case <-cmdChan:
			break eventLoop
		}
	}
	svr.log.Infof("%s done  %+v", ss.User(), cmdArr)
	onceClose()
}
