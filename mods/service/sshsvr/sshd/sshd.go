package sshd

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/pkg/errors"
)

type Config struct {
	ListenAddress       string
	AdvertiseAddress    string
	ServerKey           string // path to server key (.pem)
	ServerKeyPassphrase string // empty if no password protected
	IdleTimeout         time.Duration
	PortForward         bool
	ReversePortForward  bool
	Password            string
	AuthorizedKeysDir   string
	AutoListenAndServe  bool
}

type ShellProvider func(user string) *Shell

type Shell struct {
	Cmd  string
	Args []string
	Envs map[string]string
}

type CommandParser func(user string, cmd []string) []string

// message of the day provider
type MotdProvider func(user string) string

type Server interface {
	Start() error
	Stop()

	ListenAndServe() error

	ListenAddress() string
	AdvertiseAddress() string

	AuthorizedKeysDir() string

	SetHandler(handler ssh.Handler)
	SetShellProvider(provider ShellProvider)
	SetCommandParser(parser CommandParser)
	SetMotdProvider(provider MotdProvider)
	SetPasswordHandler(func(ctx ssh.Context, password string) bool)
	SetPublicKeyHandler(func(ctx ssh.Context, key ssh.PublicKey) bool)

	SetReversePortforwardingCallback(callback ssh.ReversePortForwardingCallback)
	SetLocalPortForwardingCallback(callback ssh.LocalPortForwardingCallback)
}

type server struct {
	Server
	log           logging.Log
	conf          *Config
	listenerLock  sync.Mutex
	listenerAddr  net.Addr
	svr           *ssh.Server
	alive         bool
	shellProvider ShellProvider
	commandParser CommandParser
	motdProvider  MotdProvider
	childrenLock  sync.Mutex
	children      map[int]*os.Process
}

func New(conf *Config) Server {
	svr := &server{
		log:      logging.GetLog("sshd"),
		conf:     conf,
		alive:    true,
		children: make(map[int]*os.Process, 0),
	}
	return svr
}

func (svr *server) Start() error {
	signers := []ssh.Signer{}
	if signer, err := signerFromPath(svr.conf.ServerKey, svr.conf.ServerKeyPassphrase); err != nil {
		return err
	} else if signer != nil {
		signers = append(signers, signer)
	}

	svr.svr = &ssh.Server{
		IdleTimeout: svr.conf.IdleTimeout,
		HostSigners: signers,
	}

	if len(svr.conf.Password) > 0 {
		svr.svr.PasswordHandler = svr.passwordHandler
	}
	if len(svr.conf.AuthorizedKeysDir) > 0 {
		svr.svr.PublicKeyHandler = svr.publicKeyHandler
	}

	svr.svr.Handler = svr.defaultHandler

	if svr.conf.PortForward {
		svr.svr.LocalPortForwardingCallback = ssh.LocalPortForwardingCallback(svr.portForwardingCallback)
		svr.svr.ChannelHandlers = map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		}
	}

	if svr.conf.ReversePortForward {
		svr.svr.ReversePortForwardingCallback = ssh.ReversePortForwardingCallback(svr.reversePortForwardingCallback)
		forwardHandler := &ssh.ForwardedTCPHandler{}
		svr.svr.RequestHandlers = map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		}
	}

	if svr.conf.AutoListenAndServe {
		go svr.ListenAndServe()
	}
	return nil
}

func (svr *server) Stop() {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	svr.alive = false

	if svr.svr != nil {
		svr.svr.Close()
	}

	for _, child := range svr.children {
		err := child.Kill()
		if err != nil {
			svr.log.Infof("kill remaining shell %d %s", child.Pid, err.Error())
		} else {
			svr.log.Infof("kill remaining shell %d", child.Pid)
		}
	}
}

func (svr *server) ListenAndServe() error {
	svr.listenerLock.Lock()
	defer svr.listenerLock.Unlock()

	if svr.listenerAddr != nil {
		return errors.New("already listen and served")
	}
	ln, err := net.Listen("tcp", svr.conf.ListenAddress)
	if err != nil {
		return err
	}
	svr.listenerAddr = ln.Addr()
	svr.log.Tracef("SSHD Listen tcp://%s", svr.listenerAddr.String())
	err = svr.svr.Serve(ln)
	return err
}

func (svr *server) ListenAddress() string {
	return svr.conf.ListenAddress
}

func (svr *server) AdvertiseAddress() string {
	return svr.conf.AdvertiseAddress
}

func (svr *server) AuthorizedKeysDir() string {
	return svr.conf.AuthorizedKeysDir
}

func (svr *server) SetShellProvider(provider ShellProvider) {
	svr.shellProvider = provider
}

func (svr *server) SetCommandParser(parser CommandParser) {
	svr.commandParser = parser
}

func (svr *server) SetMotdProvider(provider MotdProvider) {
	svr.motdProvider = provider
}

func (svr *server) SetPasswordHandler(handler func(ctx ssh.Context, password string) bool) {
	svr.svr.PasswordHandler = handler
}

func (svr *server) SetPublicKeyHandler(handler func(ctx ssh.Context, key ssh.PublicKey) bool) {
	svr.svr.PublicKeyHandler = handler
}

func (svr *server) SetHandler(handler ssh.Handler) {
	svr.svr.Handler = handler
}

func (svr *server) SetReversePortforwardingCallback(callback ssh.ReversePortForwardingCallback) {
	svr.svr.ReversePortForwardingCallback = callback
}

func (svr *server) SetLocalPortForwardingCallback(callback ssh.LocalPortForwardingCallback) {
	svr.svr.LocalPortForwardingCallback = callback
}

func (svr *server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	claim := fmt.Sprintf("%s %s", key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
	user := ctx.User()
	authfile := filepath.Join(svr.conf.AuthorizedKeysDir, fmt.Sprintf("%s.pub", user))
	file, err := os.Open(authfile)
	if err != nil {
		svr.log.Tracef("authorized key file %s", err)
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == claim {
			return true
		}
	}
	svr.log.Tracef("unauthorized %s", ctx.User())
	return false
}

func (svr *server) reversePortForwardingCallback(ctx ssh.Context, bindHost string, bindPort uint32) bool {
	svr.log.Infof("start reverse port forwarding bindHost:%s bindPort:%d", bindHost, bindPort)
	go func() {
		<-ctx.Done()
		svr.log.Infof("done  reverse port forwarding bindHost:%s bindPort:%d", bindHost, bindPort)
	}()
	return true
}

func (svr *server) portForwardingCallback(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
	svr.log.Infof("start port forwarding destHost:%s destPort:%d", destinationHost, destinationPort)
	go func() {
		<-ctx.Done()
		svr.log.Infof("done  port forwarding destHost:%s destPort:%d", destinationHost, destinationPort)
	}()
	return true
}

func (svr *server) addChild(child *os.Process) {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	if !svr.alive {
		return
	}
	svr.children[child.Pid] = child
}

func (svr *server) removeChild(child *os.Process) {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	if !svr.alive {
		return
	}
	delete(svr.children, child.Pid)
}

func (svr *server) passwordHandler(ctx ssh.Context, password string) bool {
	pass := password == svr.conf.Password

	passString := "denied"
	if pass {
		passString = "success"
	}

	svr.log.Infof("session auth %s %s from %s %s",
		passString, ctx.User(), ctx.RemoteAddr(), ctx.ClientVersion())

	return pass
}

func (svr *server) defaultHandler(ss ssh.Session) {
	if len(ss.Command()) > 0 {
		svr.commandHandler(ss)
	} else {
		svr.shellHandler(ss)
	}
}

func (svr *server) commandHandler(ss ssh.Session) {
	cmdArr := ss.Command()
	if svr.commandParser != nil {
		cmdArr = svr.commandParser(ss.User(), cmdArr)
	}
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

func (svr *server) shellHandler(ss ssh.Session) {
	svr.log.Infof("session open %s from %s", ss.User(), ss.RemoteAddr())
	if svr.shellProvider == nil {
		io.WriteString(ss, "No Shell Provider.\n")
		ss.Exit(1)
		return
	}
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
	if svr.motdProvider != nil {
		io.WriteString(ss, svr.motdProvider(ss.User()))
	}
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
