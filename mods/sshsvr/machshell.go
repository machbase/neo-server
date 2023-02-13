package shell

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/cemlib/logging"
	"github.com/machbase/cemlib/ssh/sshd"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Server interface {
	GetGrpcAddresses() []string
}

type Config struct {
	Listeners     []string
	IdleTimeout   time.Duration
	ServerKeyPath string
}

type MachShell struct {
	conf  *Config
	log   logging.Log
	sshds []sshd.Server

	Server Server // injection point
}

func New(conf *Config) *MachShell {
	return &MachShell{
		conf: conf,
	}
}

func (svr *MachShell) Start() error {
	svr.log = logging.GetLog("neoshell")
	svr.sshds = make([]sshd.Server, 0)

	for _, listen := range svr.conf.Listeners {
		listenAddress := strings.TrimPrefix(listen, "tcp://")
		cfg := sshd.Config{
			ListenAddress:      listenAddress,
			ServerKey:          svr.conf.ServerKeyPath,
			IdleTimeout:        svr.conf.IdleTimeout,
			AutoListenAndServe: false,
		}
		s := sshd.New(&cfg)
		err := s.Start()
		if err != nil {
			return errors.Wrap(err, "machsell")
		}
		s.SetShellProvider(svr.shellProvider)
		s.SetMotdProvider(svr.motdProvider)
		s.SetPasswordHandler(svr.passwordProvider)
		go func() {
			err := s.ListenAndServe()
			if err != nil {
				svr.log.Warnf("machshell-listen %s", err.Error())
			}
		}()
		svr.log.Infof("SSHD Listen %s", listen)
	}
	return nil
}

func (svr *MachShell) Stop() {
	for _, s := range svr.sshds {
		s.Stop()
	}
}

func (svr *MachShell) shellProvider(user string) *sshd.Shell {
	grpcAddrs := svr.Server.GetGrpcAddresses()
	if len(grpcAddrs) == 0 {
		return nil
	}
	return &sshd.Shell{
		Cmd: os.Args[0],
		Args: []string{
			"shell",
			"--server", grpcAddrs[0],
			"--user", user,
		},
	}
}

func (svr *MachShell) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\nmachbase-neo %v %s\r\n", strings.ToUpper(user), mach.Edition(), mods.VersionString())
}

func (svr *MachShell) passwordProvider(ctx ssh.Context, password string) bool {
	db, err := spi.NewDatabase("engine")
	if err != nil {
		return false
	}
	mdb, ok := db.(spi.DatabaseAuth)
	if !ok {
		svr.log.Errorf("user auth - unknown database instance")
	}
	user := ctx.User()
	ok, err = mdb.UserAuth(user, password)
	if err != nil {
		svr.log.Errorf("user auth", err.Error())
		return false
	}
	return ok
}
