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
	"github.com/pkg/errors"
)

type Server interface {
	GetConfig() string
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
	return &sshd.Shell{
		Cmd: os.Args[0],
		Args: []string{
			"shell",
			"--server", "tcp://127.0.0.1:5655",
			"--user", user,
		},
	}
}

func (svr *MachShell) motdProvider(user string) string {
	return fmt.Sprintf("Greeting, %s\r\nmachsvr %v\r\n", user, mods.VersionString())
}

func (svr *MachShell) passwordProvider(ctx ssh.Context, password string) bool {
	user := ctx.User()
	ok, err := mach.New().UserAuth(user, password)
	if err != nil {
		svr.log.Errorf("user auth", err.Error())
		return false
	}
	return ok
}
