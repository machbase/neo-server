package sshsvr

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/sshsvr/sshd"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Server interface {
	GetGrpcAddresses() []string
	ValidateSshPublicKey(keyType string, key string) bool
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

	db spi.Database

	versionString string
	gitSHA        string
	editionString string

	shellCmd []string

	Server Server // injection point
}

func New(db spi.Database, conf *Config) *MachShell {
	sh := &MachShell{
		conf: conf,
		db:   db,
	}
	if len(os.Args) > 0 {
		sh.shellCmd = []string{os.Args[0]}

		if strings.HasSuffix(sh.shellCmd[0], "machbase-neo") {
			sh.shellCmd = append(sh.shellCmd, "shell")
		}
	}
	return sh
}

func (svr *MachShell) Start() error {
	svr.log = logging.GetLog("neoshell")
	svr.sshds = make([]sshd.Server, 0)

	if svr.db == nil {
		return errors.New("no database instance")
	}
	if nfo, err := svr.db.GetServerInfo(); err != nil {
		return errors.Wrap(err, "no database info")
	} else {
		svr.editionString = nfo.Version.Engine
		svr.versionString = fmt.Sprintf("v%d.%d.%d",
			nfo.Version.Major, nfo.Version.Minor, nfo.Version.Patch)
		svr.gitSHA = nfo.Version.GitSHA
	}

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
		s.SetCommandParser(svr.commandParser)
		s.SetMotdProvider(svr.motdProvider)
		s.SetPasswordHandler(svr.passwordProvider)
		s.SetPublicKeyHandler(svr.publicKeyProvider)
		go func() {
			err := s.ListenAndServe()
			if err != nil {
				if !errors.Is(err, ssh.ErrServerClosed) {
					svr.log.Warnf("machshell-listen %s", err.Error())
				}
			}
		}()
		svr.sshds = append(svr.sshds, s)
		svr.log.Infof("SSHD Listen %s", listen)
	}
	return nil
}

func (svr *MachShell) Stop() {
	for _, s := range svr.sshds {
		s.Stop()
	}
}

func (svr *MachShell) makeShellCommand(user string, args ...string) []string {
	grpcAddrs := svr.Server.GetGrpcAddresses()
	if len(grpcAddrs) == 0 {
		return nil
	}
	result := append(svr.shellCmd,
		"--server", grpcAddrs[0],
	)
	if len(args) > 0 {
		result = append(result, args...)
	}
	return result
}

func (svr *MachShell) shellProvider(user string) *sshd.Shell {
	parsed := svr.makeShellCommand(user)
	if len(parsed) == 0 {
		return nil
	}
	return &sshd.Shell{
		Cmd:  parsed[0],
		Args: parsed[1:],
	}
}

func (svr *MachShell) commandParser(user string, cmd []string) []string {
	return svr.makeShellCommand(user, cmd...)
}

func (svr *MachShell) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\nmachbase-neo %s (%s) %s\r\n",
		strings.ToUpper(user), svr.versionString, svr.gitSHA, svr.editionString)
}

func (svr *MachShell) passwordProvider(ctx ssh.Context, password string) bool {
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

func (svr *MachShell) publicKeyProvider(ctx ssh.Context, key ssh.PublicKey) bool {
	if svr.Server == nil {
		return false
	}
	return svr.Server.ValidateSshPublicKey(key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
}
