package action

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/api/machrpc"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/util"
	"golang.org/x/text/language"
	"google.golang.org/grpc"
)

type Config struct {
	ServerAddr     string
	Insecure       bool
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
	db     *machrpc.Client
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

	var tlsConf *machrpc.TlsConfig
	if !act.conf.Insecure {
		tlsConf = &machrpc.TlsConfig{
			ClientKey:  act.conf.ClientKeyPath,
			ClientCert: act.conf.ClientCertPath,
			ServerCert: act.conf.ServerCertPath,
		}
	}

	machcli, err := machrpc.NewClient(&machrpc.Config{
		ServerAddr:   act.conf.ServerAddr,
		QueryTimeout: act.conf.QueryTimeout,
		Tls:          tlsConf,
	})
	if err != nil {
		return err
	}

	// user authentication
	if result, reason, err := machcli.UserAuth(act.ctx, act.conf.User, act.conf.Password); err != nil {
		return err
	} else if !result {
		return errors.New(reason)
	}

	// check connectivity to server
	mgmtClient, err := act.ManagementClient()
	if err != nil {
		return err
	}
	serverInfo, err := mgmtClient.ServerInfo(act.ctx, &mgmt.ServerInfoRequest{})
	if err != nil {
		return err
	}

	act.remoteSession = true

	// do not allow "unix://" as 'remoteSession = false'
	// --> web terminal connects to server via unix domain socket
	//
	// the official shutdown command is `machbase-neo shell shutdown`
	//
	if strings.HasPrefix(act.conf.ServerAddr, "tcp://127.0.0.1:") {
		act.remoteSession = false
	} else if !strings.HasPrefix(act.conf.ServerAddr, "tcp://") {
		serverPid := int(serverInfo.Runtime.Pid)
		if os.Getppid() != serverPid {
			// if my ppid is same with server pid, this client was invoked from server directly.
			// which means connected remotely via ssh.
			act.remoteSession = false
		}
	} else if strings.HasPrefix(act.conf.ServerAddr, "tcp://") {
		localAddrs := util.GetAllAddresses()
		serverIP, _, _ := net.SplitHostPort(strings.TrimPrefix(act.conf.ServerAddr, "tcp://"))
		for _, addr := range localAddrs {
			if addr.IP.String() == serverIP {
				act.remoteSession = false
				break
			}
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

func (act *Actor) Reconnect(username string, password string) (bool, string, error) {
	ok, reason, err := act.db.UserAuth(act.ctx, username, password)
	if err == nil && ok {
		act.conf.User = strings.ToLower(username)
		act.conf.Password = password
		act.conf.Prompt = makePrompt(act.conf.User)
	}
	return ok, reason, err
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

func (act *Actor) Database() *machrpc.Client {
	if err := act.checkDatabase(); err != nil {
		fmt.Println("ERR", err.Error())
	}
	return act.db
}

func (act *Actor) ManagementClient() (mgmt.ManagementClient, error) {
	act.mgmtClientLock.Lock()
	defer act.mgmtClientLock.Unlock()
	if act.mgmtClient == nil {
		if act.conf.Insecure {
			conn, err := machrpc.MakeGrpcInsecureConn(act.conf.ServerAddr)
			if err != nil {
				return nil, err
			}
			act.mgmtClient = mgmt.NewManagementClient(conn)
		} else {
			conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
			if err != nil {
				return nil, err
			}
			act.mgmtClient = mgmt.NewManagementClient(conn)
		}
	}
	return act.mgmtClient, nil
}

func (act *Actor) makeConn() (grpc.ClientConnInterface, error) {
	if act.conf.Insecure {
		conn, err := machrpc.MakeGrpcInsecureConn(act.conf.ServerAddr)
		if err != nil {
			return nil, err
		}
		return conn, nil
	} else {
		conn, err := machrpc.MakeGrpcTlsConn(act.conf.ServerAddr, act.conf.ClientKeyPath, act.conf.ClientCertPath, act.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
}

func (act *Actor) BridgeManagementClient() (bridge.ManagementClient, error) {
	act.bridgeClientLock.Lock()
	defer act.bridgeClientLock.Unlock()
	if act.bridgeMgmtClient == nil {
		if conn, err := act.makeConn(); err != nil {
			return nil, err
		} else {
			act.bridgeMgmtClient = bridge.NewManagementClient(conn)
		}
	}
	return act.bridgeMgmtClient, nil
}

func (act *Actor) BridgeRuntimeClient() (bridge.RuntimeClient, error) {
	act.bridgeClientLock.Lock()
	defer act.bridgeClientLock.Unlock()
	if act.bridgeRuntimeClient == nil {
		if conn, err := act.makeConn(); err != nil {
			return nil, err
		} else {
			act.bridgeRuntimeClient = bridge.NewRuntimeClient(conn)
		}
	}
	return act.bridgeRuntimeClient, nil
}

func (act *Actor) ScheduleManagementClient() (schedule.ManagementClient, error) {
	act.schedClientLock.Lock()
	defer act.schedClientLock.Unlock()
	if act.schedMgmtClient == nil {
		if conn, err := act.makeConn(); err != nil {
			act.schedMgmtClient = schedule.NewManagementClient(conn)
		} else {
			act.schedMgmtClient = schedule.NewManagementClient(conn)
		}
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
