package sshd

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/pkg/errors"
	gossh "golang.org/x/crypto/ssh"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *sshd)

// Factory
func New(db *mach.Database, options ...Option) (Service, error) {
	s := &sshd{
		log:             logging.GetLog("sshd"),
		db:              db,
		neoShellAccount: map[string]string{},

		enablePortForward:        (runtime.GOOS == "linux" || runtime.GOOS == "darwin"),
		enableReversePortForward: (runtime.GOOS == "linux" || runtime.GOOS == "darwin"),
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

// MotdMessage
func OptionMotdMessage(msg string) Option {
	return func(s *sshd) {
		s.motdMessage = msg
	}
}

func OptionShellProvider(provider func(user string, shellId string) *Shell) Option {
	return func(s *sshd) {
		s.shellProvider = provider
	}
}

type sshd struct {
	log   logging.Log
	db    *mach.Database
	alive bool

	dumpInput  bool
	dumpOutput bool

	listenAddresses []string
	idleTimeout     time.Duration
	serverKeyPath   string
	motdMessage     string
	authServer      security.AuthServer

	enablePortForward        bool
	enableReversePortForward bool

	sshServer *ssh.Server
	listeners []net.Listener

	childrenLock sync.Mutex
	children     map[int]*os.Process

	shellProvider func(user string, shellId string) *Shell

	neoShellAccount map[string]string
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
			"default": func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
				svr.log.Warnf("unknown channel type %s", newChan.ChannelType())
				newChan.Reject(gossh.UnknownChannelType, "unknown channel type")
			},
		}
	}

	if svr.enableReversePortForward {
		svr.sshServer.ReversePortForwardingCallback = ssh.ReversePortForwardingCallback(svr.reversePortForwardingCallback)
		forwardHandler := &ssh.ForwardedTCPHandler{}
		svr.sshServer.RequestHandlers = map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
			"default": func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
				svr.log.Warnf("unknown request type %s", req.Type)
				return false, nil
			},
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

func (svr *sshd) shell(user string, shellId string) *Shell {
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
