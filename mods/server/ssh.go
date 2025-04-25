package server

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

type SshOption func(s *sshd)

// Factory
func NewSsh(options ...SshOption) (*sshd, error) {
	s := &sshd{
		log:             logging.GetLog("sshd"),
		neoShellAccount: map[string]string{},
		forwardHandler:  &ssh.ForwardedTCPHandler{},
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// ListenAddresses
func WithSshListenAddress(addrs ...string) SshOption {
	return func(s *sshd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// ServerKeyPath
func WithSshServerKeyPath(path string) SshOption {
	return func(s *sshd) {
		s.serverKeyPath = path
	}
}

// IdleTimeout
func WithSshIdleTimeout(timeout time.Duration) SshOption {
	return func(s *sshd) {
		s.idleTimeout = timeout
	}
}

// AuthServer
func WithSshAuthServer(authSvc AuthServer) SshOption {
	return func(s *sshd) {
		s.authServer = authSvc
	}
}

// MotdMessage
func WithSshMotdMessage(msg string) SshOption {
	return func(s *sshd) {
		s.motdMessage = msg
	}
}

func WithSshShellProvider(provider func(user string, shellId string) *SshShell) SshOption {
	return func(s *sshd) {
		s.shellProvider = provider
	}
}

type sshd struct {
	log   logging.Log
	alive bool

	dumpInput  bool
	dumpOutput bool

	listenAddresses []string
	idleTimeout     time.Duration
	serverKeyPath   string
	motdMessage     string
	authServer      AuthServer

	sshServer *ssh.Server
	listeners []net.Listener

	forwardHandler *ssh.ForwardedTCPHandler
	childrenLock   sync.Mutex
	children       map[int]*os.Process

	shellProvider func(user string, shellId string) *SshShell

	neoShellAccount map[string]string
}

func (svr *sshd) Start() error {
	svr.alive = true

	signers := []ssh.Signer{}
	if signer, err := signerFromPath(svr.serverKeyPath, ""); err != nil {
		return err
	} else if signer != nil {
		signers = append(signers, signer)
	}

	svr.sshServer = &ssh.Server{
		IdleTimeout: svr.idleTimeout,
		HostSigners: signers,
	}
	svr.sshServer.Handler = svr.defaultHandler
	svr.sshServer.PasswordHandler = svr.passwordHandler
	svr.sshServer.PublicKeyHandler = svr.publicKeyHandler
	svr.sshServer.SubsystemHandlers = map[string]ssh.SubsystemHandler{
		"sftp": svr.SftpHandler,
	}
	svr.sshServer.LocalPortForwardingCallback = func(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
		if ctx.User() != "sys" {
			return false
		}
		svr.log.Debugf("port forwarding %s, dest-> %s:%d", ctx.RemoteAddr(), destinationHost, destinationPort)
		return true
	}
	svr.sshServer.ReversePortForwardingCallback = func(ctx ssh.Context, bindHost string, bindPort uint32) bool {
		if ctx.User() != "sys" {
			return false
		}
		svr.log.Debugf("reverse port forwarding bindHost:%s bindPort:%d", bindHost, bindPort)
		return true
	}
	svr.sshServer.ChannelHandlers = map[string]ssh.ChannelHandler{
		"direct-tcpip": ssh.DirectTCPIPHandler,
		"session":      ssh.DefaultSessionHandler,
		"default": func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
			svr.log.Debugf("unknown channel type %s", newChan.ChannelType())
			newChan.Reject(gossh.UnknownChannelType, "unknown channel type")
		},
	}
	svr.sshServer.RequestHandlers = map[string]ssh.RequestHandler{
		"tcpip-forward":        svr.forwardHandler.HandleSSHRequest,
		"cancel-tcpip-forward": svr.forwardHandler.HandleSSHRequest,
		"keepalive@openssh.com": func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
			svr.log.Trace("keepalive@openssh.com")
			return true, nil
		},
		"default": func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
			svr.log.Debugf("unknown request type %s", req.Type)
			return false, nil
		},
	}

	for _, listen := range svr.listenAddresses {
		listenAddress := strings.TrimPrefix(listen, "tcp://")

		ln, err := net.Listen("tcp", listenAddress)
		if err != nil {
			return fmt.Errorf("machshell, %s", err.Error())
		}
		svr.listeners = append(svr.listeners, ln)

		go func() {
			err := svr.sshServer.Serve(ln)
			if err != nil {
				if svr.alive && !errors.Is(err, ssh.ErrServerClosed) {
					svr.log.Warnf("machshell-listen %s", err.Error())
				}
			}
		}()
		svr.log.Infof("SSHD Listen %s", listen)
	}
	return nil
}

func (svr *sshd) Stop() {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	svr.alive = false

	for svr.sshServer != nil {
		svr.sshServer.Close()
		svr.sshServer = nil
	}

	for _, l := range svr.listeners {
		l.Close()
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

func (svr *sshd) defaultHandler(ss ssh.Session) {
	svr.log.Debug("session open", ss.RemoteAddr())
	if len(ss.Command()) > 0 {
		svr.commandHandler(ss)
	} else {
		svr.shellHandler(ss)
	}
	svr.log.Debug("session close", ss.RemoteAddr())
}

func (svr *sshd) addChild(child *os.Process) {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	if !svr.alive {
		return
	}
	if svr.children == nil {
		svr.children = make(map[int]*os.Process)
	}
	svr.children[child.Pid] = child
}

func (svr *sshd) removeChild(child *os.Process) {
	svr.childrenLock.Lock()
	defer svr.childrenLock.Unlock()
	if !svr.alive {
		return
	}
	delete(svr.children, child.Pid)
}

func (svr *sshd) shell(user string, shellId string) *SshShell {
	if svr.shellProvider == nil {
		return nil
	}
	return svr.shellProvider(user, shellId)
}

func (svr *sshd) passwordHandler(ctx ssh.Context, password string) bool {
	if svr.authServer == nil {
		return false
	}
	user := ctx.User()
	if strings.Contains(user, ":") {
		user = strings.Split(user, ":")[0]
	}
	user = strings.ToLower(user)
	ok, otp, err := svr.authServer.ValidateUserPassword(user, password)
	if err != nil {
		svr.log.Errorf("user auth", err.Error())
		return false
	}
	if !ok {
		svr.log.Tracef("'%s' login fail password mis-matched", user)
	}

	svr.neoShellAccount[user] = fmt.Sprintf("$otp$:%s", otp)
	return ok
}

func (svr *sshd) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	if svr.authServer == nil {
		return false
	}
	user := ctx.User()
	if strings.Contains(user, ":") {
		user = strings.Split(user, ":")[0]
	}
	user = strings.ToLower(user)
	ok, otp, err := svr.authServer.ValidateUserPublicKey(user, key)
	if err != nil {
		svr.log.Error("ERR", err.Error())
		return false
	}
	if ok {
		svr.neoShellAccount[user] = fmt.Sprintf("$otp$:%s", otp)
	}
	return ok
}

type SshShell struct {
	Cmd  string
	Args []string
	Envs []string
}

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

// splitUserAndShell splits USER and SHELL_ID and CMD from the user string.
func (svr *sshd) splitUserAndShell(user string) UsernameComposite {
	ret := UsernameComposite{}

	if strings.HasPrefix(strings.ToLower(user), "sys+") {
		// only sys user can use this feature
		toks := strings.SplitN(user, "+", 2)
		ret.user = toks[0]
		ret.command = toks[1]
	} else if strings.Contains(user, ":") {
		toks := strings.SplitN(user, ":", 2)
		ret.user = toks[0]
		ret.shellId = toks[1]
		if strings.Contains(ret.shellId, "@") {
			toks = strings.SplitN(ret.shellId, "@", 2)
			ret.shellId = toks[0]
			ret.consoleId = toks[1]
		}
	} else {
		ret.user = user
		ret.shellId = model.SHELLID_SHELL
	}
	return ret
}

type UsernameComposite struct {
	user      string
	shellId   string
	consoleId string
	command   string
}

func (svr *sshd) findShell(ss ssh.Session) (string, *SshShell, string) {
	user := ss.User()
	var shell *SshShell
	var shellId string
	var command string

	uc := svr.splitUserAndShell(user)
	user, shellId, command = uc.user, uc.shellId, uc.command
	if command != "" {
		shell = &SshShell{
			Cmd:  command,
			Envs: make([]string, 0),
		}
		return user, shell, ""
	}

	shell = svr.shell(user, shellId)
	if shell == nil {
		return user, nil, shellId
	}

	return user, shell, shellId
}

func (svr *sshd) SftpHandler(sess ssh.Session) {
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		svr.log.Warn("sftp server init error:", err)
		return
	}
	svr.log.Debug("sftp client start session")
	if err := server.Serve(); err == io.EOF {
		// FIXME: sess doesn't return io.EOF when client disconnects
		server.Close()
		svr.log.Debug("sftp client exited session")
	} else if err != nil {
		svr.log.Warn("sftp server completed with error:", err)
	}
}

func (svr *sshd) commandHandler(ss ssh.Session) {
	user, shell, shellId := svr.findShell(ss)
	svr.log.Debugf("shell open %s from %s", user, ss.RemoteAddr())

	if shell == nil {
		io.WriteString(ss, "No shell found\n")
		ss.Exit(1)
		return
	}

	if cmd := ss.Command(); len(cmd) > 0 && cmd[0] == "jsh" {
		jsCmd := "@.js"
		jsArgs := []string{}
		if len(cmd) > 1 {
			jsCmd = cmd[1]
		}
		if len(cmd) > 2 {
			jsArgs = cmd[2:]
		}
		svr.jshHandler(ss, jsCmd, jsArgs, shell.Envs)
		return
	}

	if shellId == model.SHELLID_SHELL {
		shell.Envs = append(shell.Envs, fmt.Sprintf("NEOSHELL_USER=%s", user))
		shell.Envs = append(shell.Envs, fmt.Sprintf("NEOSHELL_PASSWORD=%s", svr.neoShellAccount[strings.ToLower(user)]))
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
	cmd.Env = append(cmd.Env, shell.Envs...)

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

func NewIODebugger(log logging.Log, prefix string) io.Writer {
	return &inoutWriter{
		prefix: prefix,
		log:    log,
	}
}

type inoutWriter struct {
	prefix string
	log    logging.Log
}

func (iow *inoutWriter) Write(b []byte) (int, error) {
	iow.log.Infof("%s %s", iow.prefix, strings.TrimSpace(hex.Dump(b)))
	return len(b), nil
}

func signerFromPath(keypath, password string) (ssh.Signer, error) {
	var signer ssh.Signer
	if len(keypath) > 0 {
		pemBytes, err := os.ReadFile(keypath)
		if err != nil {
			return signer, fmt.Errorf("server key, %s", err.Error())
		}
		var keypass []byte
		if len(password) > 0 {
			keypass = []byte(password)
		}
		signer, err = signerFromPem(pemBytes, keypass)
		if err != nil {
			return signer, fmt.Errorf("server signer, %s", err.Error())
		}
	}
	return signer, nil
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {
	// read pem block
	err := errors.New("pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	if password != nil {
		// decrypt PEM
		// TODO legacy PEM RFC1423 is insecure
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("decrypting PEM block failed %v", err)
		}
	}

	// get RSA, EC or DSA key
	key, err := parsePemBlock(pemBlock)
	if err != nil {
		return nil, err
	}

	// generate signer instance from key
	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("creating signer from encrypted key failed %v", err)
	}

	return signer, nil
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := gossh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("parsing private key failed, unsupported key type %q", block.Type)
	}
}

func (svr *sshd) jshHandler(ss ssh.Session, cmd string, args []string, env []string) {
	_ = env

	// if ssh session is not interactive, disable echo
	echo := len(ss.Command()) == 0

	j := jsh.NewJsh(
		ss.Context(),
		jsh.WithNativeModules(jsh.NativeModuleNames()...),
		jsh.WithParent(nil),
		jsh.WithJshReader(ss),
		jsh.WithJshWriter(ss),
		jsh.WithJshEcho(echo),
	)
	err := j.Exec(append([]string{cmd}, args...))
	if err != nil {
		if cmd == "@.js" {
			cmd = "jsh"
		}
		for _, err := range j.Errors() {
			svr.log.Warnf("%s %s", cmd, jsh.ErrorToString(err))
		}
		ss.Exit(1)
		return
	} else {
		ss.Exit(0)
		return
	}
}
