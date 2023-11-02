package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-grpc/schedule"
	spi "github.com/machbase/neo-spi"
	"golang.org/x/text/language"
)

type Config struct {
	ServerAddr     string
	ServerCertPath string
	ClientCertPath string
	ClientKeyPath  string
	User           string
	Password       string
	Prompt         string
	PromptCont     string
	QueryTimeout   time.Duration
	Lang           language.Tag
}

func DefaultConfig() *Config {
	return &Config{
		Prompt:       "\033[31mmachbase-neo»\033[0m ",
		PromptCont:   "\033[31m>\033[0m  ",
		QueryTimeout: 0 * time.Second,
		Lang:         language.English,
	}
}

type ShutdownServerFunc func() error

var Formats = struct {
	Default string
	CSV     string
	JSON    string
	Parse   func(string) string
}{
	Default: "-",
	CSV:     "csv",
	JSON:    "json",
	Parse: func(str string) string {
		switch str {
		default:
			return "-"
		case "csv":
			return "csv"
		}
	},
}

type Actor struct {
	conf   *Config
	db     spi.DatabaseClient
	dbLock sync.Mutex
	pref   *Pref
	ctx    context.Context

	interactive   bool
	remoteSession bool

	mgmtClient     mgmt.ManagementClient
	mgmtClientLock sync.Mutex

	bridgeMgmtClient    bridge.ManagementClient
	bridgeRuntimeClient bridge.RuntimeClient
	bridgeClientLock    sync.Mutex

	schedMgmtClient schedule.ManagementClient
	schedClientLock sync.Mutex
}

func NewActor(conf *Config, interactive bool) *Actor {
	ret := &Actor{
		conf:        conf,
		interactive: interactive,
	}
	ret.ctx = context.Background()
	ret.conf.Prompt = makePrompt(conf.User)
	return ret
}

func (act *Actor) Start() error {
	pref, err := LoadPref()
	if err != nil {
		return err
	}
	act.pref = pref

	if err := act.checkDatabase(); err != nil {
		return err
	}
	return nil
}

func (act *Actor) Stop() {
	if act.db != nil {
		act.db.Close()
	}
}

func (act *Actor) checkDatabase() error {
	if act.db != nil {
		return nil
	}

	act.dbLock.Lock()
	defer act.dbLock.Unlock()
	if act.db != nil {
		return nil
	}

	machcli, err := machrpc.NewClient(
		machrpc.WithServer(act.conf.ServerAddr),
		machrpc.WithCertificate(act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath),
		machrpc.WithQueryTimeout(act.conf.QueryTimeout))
	if err != nil {
		return err
	}

	// user authentication
	auth := machcli.(spi.DatabaseAuth)
	if result, err := auth.UserAuth(act.conf.User, act.conf.Password); err != nil {
		return err
	} else if !result {
		return errors.New("invalid username or password")
	}

	// check connectivity to server
	aux := machcli.(spi.DatabaseAux)
	serverInfo, err := aux.GetServerInfo()
	if err != nil {
		return err
	}

	act.remoteSession = true
	if strings.HasPrefix(act.conf.ServerAddr, "tcp://127.0.0.1:") {
		act.remoteSession = false
	} else if !strings.HasPrefix(act.conf.ServerAddr, "tcp://") {
		serverPid := int(serverInfo.Runtime.Pid)
		if os.Getppid() != serverPid {
			// if my ppid is same with server pid, this client was invoked from server directly.
			// which means connected remotely via ssh.
			act.remoteSession = false
		}
	}

	act.db = machcli
	return err
}

func (act *Actor) Username() string {
	return act.conf.User
}

func makePrompt(username string) string {
	return fmt.Sprintf("\033[33m%s \033[31mmachbase-neo»\033[0m ", username)
}

func (act *Actor) Reconnect(username string, password string) (bool, error) {
	auth := act.db.(spi.DatabaseAuth)
	ok, err := auth.UserAuth(username, password)
	if err == nil && ok {
		act.conf.User = strings.ToLower(username)
		act.conf.Password = password
		act.conf.Prompt = makePrompt(act.conf.User)
	}
	return ok, err
}

func (act *Actor) ShutdownServer() error {
	if act.remoteSession {
		return errors.New("remote session is not allowed to shutdown")
	}
	if strings.ToLower(act.Username()) != "sys" {
		return fmt.Errorf("%q is not allowed to shutdown", act.Username())
	}

	mgmtcli, err := act.ManagementClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rsp, err := mgmtcli.Shutdown(ctx, &mgmt.ShutdownRequest{})
	if err != nil {
		return err
	}
	if !rsp.Success {
		return errors.New(rsp.Reason)
	}
	return nil
}

func (act *Actor) Database() spi.Database {
	if err := act.checkDatabase(); err != nil {
		fmt.Println("ERR", err.Error())
	}
	return act.db
}

func (act *Actor) ManagementClient() (mgmt.ManagementClient, error) {
	act.mgmtClientLock.Lock()
	defer act.mgmtClientLock.Unlock()
	if act.mgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		act.mgmtClient = mgmt.NewManagementClient(conn)
	}
	return act.mgmtClient, nil
}

func (act *Actor) BridgeManagementClient() (bridge.ManagementClient, error) {
	act.bridgeClientLock.Lock()
	defer act.bridgeClientLock.Unlock()
	if act.bridgeMgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		act.bridgeMgmtClient = bridge.NewManagementClient(conn)
	}
	return act.bridgeMgmtClient, nil
}

func (act *Actor) BridgeRuntimeClient() (bridge.RuntimeClient, error) {
	act.bridgeClientLock.Lock()
	defer act.bridgeClientLock.Unlock()
	if act.bridgeRuntimeClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		act.bridgeRuntimeClient = bridge.NewRuntimeClient(conn)
	}
	return act.bridgeRuntimeClient, nil
}

func (act *Actor) ScheduleManagementClient() (schedule.ManagementClient, error) {
	act.schedClientLock.Lock()
	defer act.schedClientLock.Unlock()
	if act.schedMgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		act.schedMgmtClient = schedule.NewManagementClient(conn)
	}
	return act.schedMgmtClient, nil
}

func (act *Actor) Run(command string) {
	if len(command) == 0 {
		act.Prompt()
	} else {
		act.Process(command)
	}
}
