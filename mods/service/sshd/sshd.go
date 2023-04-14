package sshd

import (
	"encoding/base64"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *sshd)

// Factory
func New(db spi.Database, options ...Option) (Service, error) {
	s := &sshd{
		log: logging.GetLog("sshd"),
		db:  db,
	}

	if len(os.Args) > 0 {
		s.shellCmd = []string{os.Args[0]}
		if strings.HasSuffix(s.shellCmd[0], "machbase-neo") {
			s.shellCmd = append(s.shellCmd, "shell")
		}
	}

	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// ListenAddresses
func OptionListenAddress(addrs ...string) Option {
	return func(s *sshd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// ServerKeyPath
func OptionServerKeyPath(path string) Option {
	return func(s *sshd) {
		s.serverKeyPath = path
	}
}

// IdleTimeout
func OptionIdleTimeout(timeout time.Duration) Option {
	return func(s *sshd) {
		s.idleTimeout = timeout
	}
}

// AuthServer
func OptionAuthServer(authSvc security.AuthServer) Option {
	return func(s *sshd) {
		s.authServer = authSvc
	}
}

// GrpcServerAddress
func OptionGrpcServerAddress(addr ...string) Option {
	return func(s *sshd) {
		s.grpcAddresses = append(s.grpcAddresses, addr...)
	}
}

// MotdMessage
func OptionMotdMessage(msg string) Option {
	return func(s *sshd) {
		s.motdMessage = msg
	}
}

type sshd struct {
	log   logging.Log
	db    spi.Database
	alive bool

	listenAddresses []string
	idleTimeout     time.Duration
	serverKeyPath   string
	motdMessage     string
	authServer      security.AuthServer
	grpcAddresses   []string

	enablePortForward        bool
	enableReversePortForward bool

	sshServer *ssh.Server
	listeners []net.Listener

	childrenLock sync.Mutex
	children     map[int]*os.Process

	shellCmd []string
}

type Shell struct {
	Cmd  string
	Args []string
	Envs map[string]string
}

func (svr *sshd) Start() error {
	if svr.db == nil {
		return errors.New("no database instance")
	}

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

	if svr.enablePortForward {
		svr.sshServer.LocalPortForwardingCallback = ssh.LocalPortForwardingCallback(svr.portForwardingCallback)
		svr.sshServer.ChannelHandlers = map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		}
	}

	if svr.enableReversePortForward {
		svr.sshServer.ReversePortForwardingCallback = ssh.ReversePortForwardingCallback(svr.reversePortForwardingCallback)
		forwardHandler := &ssh.ForwardedTCPHandler{}
		svr.sshServer.RequestHandlers = map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		}
	}

	for _, listen := range svr.listenAddresses {
		listenAddress := strings.TrimPrefix(listen, "tcp://")

		ln, err := net.Listen("tcp", listenAddress)
		if err != nil {
			return errors.Wrap(err, "machshell")
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
	if len(ss.Command()) > 0 {
		svr.commandHandler(ss)
	} else {
		svr.shellHandler(ss)
	}
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

func (svr *sshd) makeShellCommand(user string, args ...string) []string {
	if len(svr.grpcAddresses) == 0 {
		return nil
	}
	candidates := []string{}
	for _, addr := range svr.grpcAddresses {
		if runtime.GOOS == "windows" && strings.HasPrefix(addr, "unix://") {
			continue
		}
		candidates = append(candidates, addr)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if strings.HasPrefix(candidates[i], "unix://") {
			return true
		}
		if candidates[i] == "127.0.0.1" || candidates[i] == "localhost" {
			return true
		}
		return false
	})
	result := append(svr.shellCmd,
		"--server", candidates[0],
	)
	if len(args) > 0 {
		result = append(result, args...)
	}
	return result
}

func (svr *sshd) shellProvider(user string) *Shell {
	parsed := svr.makeShellCommand(user)
	if len(parsed) == 0 {
		return nil
	}
	return &Shell{
		Cmd:  parsed[0],
		Args: parsed[1:],
	}
}

func (svr *sshd) passwordHandler(ctx ssh.Context, password string) bool {
	mdb, ok := svr.db.(spi.DatabaseAuth)
	if !ok {
		svr.log.Errorf("user auth - unknown database instance")
		return false
	}
	user := ctx.User()
	ok, err := mdb.UserAuth(user, password)
	if err != nil {
		svr.log.Errorf("user auth", err.Error())
		return false
	}
	return ok
}

func (svr *sshd) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	if svr.authServer == nil {
		return false
	}
	ok, err := svr.authServer.ValidateSshPublicKey(key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
	if err != nil {
		svr.log.Error("ERR", err.Error())
		return false
	}
	return ok
}
