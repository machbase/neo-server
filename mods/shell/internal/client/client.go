package client

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-grpc/schedule"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
	spi "github.com/machbase/neo-spi"
	"golang.org/x/net/context"
	"golang.org/x/term"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Client interface {
	Start() error
	Stop()

	Run(command string)

	Interactive() bool

	Write(p []byte) (int, error)
	Print(args ...any)
	Printf(format string, args ...any)
	Println(args ...any)
	Printfln(format string, args ...any)

	Database() spi.Database
	Pref() *Pref
	ManagementClient() (mgmt.ManagementClient, error)
	BridgeManagementClient() (bridge.ManagementClient, error)
	BridgeRuntimeClient() (bridge.RuntimeClient, error)
	ScheduleManagementClient() (schedule.ManagementClient, error)
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

type Config struct {
	ServerAddr     string
	ServerCertPath string
	ClientCertPath string
	ClientKeyPath  string
	Prompt         string
	PromptCont     string
	QueryTimeout   time.Duration
	Lang           language.Tag
}

type client struct {
	conf   *Config
	db     spi.DatabaseClient
	dbLock sync.Mutex
	pref   *Pref

	rl            *readline.Instance
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

func DefaultConfig() *Config {
	return &Config{
		Prompt:       "\033[31mmachbase-neo»\033[0m ",
		PromptCont:   "\033[31m>\033[0m  ",
		QueryTimeout: 0 * time.Second,
		Lang:         language.English,
	}
}

func New(conf *Config, interactive bool) Client {
	return &client{
		conf:        conf,
		interactive: interactive,
	}
}

func (cli *client) Start() error {
	pref, err := LoadPref()
	if err != nil {
		return err
	}
	cli.pref = pref

	return nil
}

func (cli *client) Stop() {
	if cli.db != nil {
		cli.db.Close()
	}
}

func (cli *client) Database() spi.Database {
	if err := cli.checkDatabase(); err != nil {
		cli.Println("ERR", err.Error())
	}
	return cli.db
}

func (cli *client) Pref() *Pref {
	return cli.pref
}

func (cli *client) checkDatabase() error {
	if cli.db != nil {
		return nil
	}

	cli.dbLock.Lock()
	defer cli.dbLock.Unlock()
	if cli.db != nil {
		return nil
	}

	machcli, err := machrpc.NewClient(
		machrpc.WithServer(cli.conf.ServerAddr),
		machrpc.WithCertificate(cli.conf.ClientKeyPath, cli.conf.ClientCertPath, cli.conf.ServerCertPath),
		machrpc.WithQueryTimeout(cli.conf.QueryTimeout))
	if err != nil {
		return err
	}

	// check connectivity to server
	aux := machcli.(spi.DatabaseAux)
	serverInfo, err := aux.GetServerInfo()
	if err != nil {
		return err
	}

	cli.remoteSession = true
	if strings.HasPrefix(cli.conf.ServerAddr, "tcp://127.0.0.1:") {
		cli.remoteSession = false
	} else if !strings.HasPrefix(cli.conf.ServerAddr, "tcp://") {
		serverPid := int(serverInfo.Runtime.Pid)
		if os.Getppid() != serverPid {
			// if my ppid is same with server pid, this client was invoked from server directly.
			// which means connected remotely via ssh.
			cli.remoteSession = false
		}
	}

	cli.db = machcli
	return err
}

func (cli *client) ShutdownServer() error {
	if cli.remoteSession {
		return errors.New("remote session is not allowed to shutdown")
	}

	mgmtcli, err := cli.ManagementClient()
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

func (cli *client) ManagementClient() (mgmt.ManagementClient, error) {
	cli.mgmtClientLock.Lock()
	defer cli.mgmtClientLock.Unlock()
	if cli.mgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(cli.conf.ServerAddr, cli.conf.ClientKeyPath, cli.conf.ClientCertPath, cli.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		cli.mgmtClient = mgmt.NewManagementClient(conn)
	}
	return cli.mgmtClient, nil
}

func (cli *client) BridgeManagementClient() (bridge.ManagementClient, error) {
	cli.bridgeClientLock.Lock()
	defer cli.bridgeClientLock.Unlock()
	if cli.bridgeMgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(cli.conf.ServerAddr, cli.conf.ClientKeyPath, cli.conf.ClientCertPath, cli.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		cli.bridgeMgmtClient = bridge.NewManagementClient(conn)
	}
	return cli.bridgeMgmtClient, nil
}

func (cli *client) BridgeRuntimeClient() (bridge.RuntimeClient, error) {
	cli.bridgeClientLock.Lock()
	defer cli.bridgeClientLock.Unlock()
	if cli.bridgeRuntimeClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(cli.conf.ServerAddr, cli.conf.ClientKeyPath, cli.conf.ClientCertPath, cli.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		cli.bridgeRuntimeClient = bridge.NewRuntimeClient(conn)
	}
	return cli.bridgeRuntimeClient, nil
}

func (cli *client) ScheduleManagementClient() (schedule.ManagementClient, error) {
	cli.schedClientLock.Lock()
	defer cli.schedClientLock.Unlock()
	if cli.schedMgmtClient == nil {
		conn, err := machrpc.MakeGrpcTlsConn(cli.conf.ServerAddr, cli.conf.ClientKeyPath, cli.conf.ClientCertPath, cli.conf.ServerCertPath)
		if err != nil {
			return nil, err
		}
		cli.schedMgmtClient = schedule.NewManagementClient(conn)
	}
	return cli.schedMgmtClient, nil
}

func (cli *client) Run(command string) {
	if len(command) == 0 {
		cli.Prompt()
	} else {
		cli.Process(command)
	}
}

func (cli *client) Interactive() bool {
	return cli.interactive
}

func (cli *client) Config() *Config {
	return cli.conf
}

type Cmd struct {
	Name   string
	PcFunc func() readline.PrefixCompleterInterface
	Action func(ctx *ActionContext)
	Desc   string
	Usage  string

	// if the Cmd is the client side action
	ClientAction bool
	// if the Cmd is an experimental feature
	Experimental bool
	// Deprecated
	Deprecated        bool
	DeprecatedMessage string
}

var commands = make(map[string]*Cmd)

func RegisterCmd(cmd *Cmd) {
	commands[cmd.Name] = cmd
}

func FindCmd(name string) *Cmd {
	return commands[name]
}

func Commands() []*Cmd {
	list := []*Cmd{}
	for _, v := range commands {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name <= list[j].Name
	})
	return list
}

func (cli *client) completer() readline.PrefixCompleterInterface {
	pc := make([]readline.PrefixCompleterInterface, 0)
	for _, cmd := range commands {
		if cmd.PcFunc != nil {
			pc = append(pc, cmd.PcFunc())
		}
	}
	return readline.NewPrefixCompleter(pc...)
}

func (cli *client) Process(line string) {
	fields := util.SplitFields(line, true)
	if len(fields) == 0 {
		return
	}
	if runtime.GOOS == "windows" {
		// on windows, command line keeps the trailing ';'
		fields[len(fields)-1] = strings.TrimSuffix(fields[len(fields)-1], ";")
	}

	cmdName := fields[0]
	var cmd *Cmd
	var ok bool
	if cmd, ok = commands[strings.ToLower(cmdName)]; ok {
		line = strings.TrimSpace(line[len(cmdName):])
	} else {
		cmd, ok = commands["sql"]
	}

	if !ok || cmd == nil {
		cli.Println("command not found", cmdName)
		return
	}

	if !cmd.ClientAction {
		if err := cli.checkDatabase(); err != nil {
			cli.Println("ERR", err.Error())
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := cli.db.Connect(ctx, machrpc.WithPassword("sys", "manager"))
	if err != nil {
		cli.Println("ERR", err.Error())
		return
	}
	defer conn.Close()

	actCtx := &ActionContext{
		Line:         line,
		Client:       cli,
		Conn:         conn,
		Ctx:          ctx,
		Lang:         cli.conf.Lang,
		TimeLocation: time.UTC,
		TimeFormat:   "ns",
		Interactive:  cli.interactive,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	}

	if cli.rl != nil {
		actCtx.ReadLine = cli.rl
		defer cli.rl.SetPrompt(cli.conf.Prompt)
	} else {
		rl, _ := readline.NewEx(&readline.Config{
			DisableAutoSaveHistory: true,
			InterruptPrompt:        "^C",
		})
		if cli.interactive {
			// avoid print-out ansi-code on terminal when it runs on batch mode
			defer rl.Close()
		}
		actCtx.ReadLine = rl
	}

	actCtx.parent, actCtx.cancelFunc = context.WithCancel(context.Background())
	actCtx.cli = cli

	defer actCtx.cancelFunc()

	cmd.Action(actCtx)

	if cmd.Deprecated {
		cli.Println()
		cli.Printfln("    '%s' is deprecated, %s", cmd.Name, cmd.DeprecatedMessage)
	}
}

func (cli *client) Prompt() {
	historyFile := filepath.Join(PrefDir(), ".neoshell_history")
	readlineCfg := &readline.Config{
		Prompt:                 cli.conf.Prompt,
		HistoryFile:            historyFile,
		DisableAutoSaveHistory: true,
		AutoComplete:           cli.completer(),
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		HistorySearchFold:      true,
		FuncFilterInputRune:    filterInput,
	}

	if runtime.GOOS == "windows" {
		// TODO on windows,
		//      up/down arrow keys for the history is not working if stdin is set
		//      guess: underlying Windows interface requires os.Stdin.Fd() to syscall
		if oldState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}

	rl, err := readline.NewEx(readlineCfg)
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	rl.CaptureExitSignal()
	rl.SetVimMode(cli.Pref().ViMode().BoolValue())

	log.SetOutput(rl.Stderr())

	var parts []string
	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// when user send input '^C'
			// clear multi-line buffer and recover origin prompt
			parts = parts[:0]
			rl.SetPrompt(cli.conf.Prompt)
			continue
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			parts = parts[:0]
			rl.SetPrompt(cli.conf.Prompt)
			continue
		}
		if len(parts) == 0 {
			if trimLine(line) == "exit" || trimLine(line) == "quit" {
				goto exit
			} else if trimLine(line) == "clear" {
				cli.Println("\033\143")
				continue
			} else if strings.HasPrefix(line, "help") {
				goto madeline
			} else if line == "set" || strings.HasPrefix(line, "set ") {
				goto madeline
			}
		}

		parts = append(parts, strings.Clone(line))
		if !strings.HasSuffix(line, ";") {
			rl.SetPrompt(cli.conf.PromptCont)
			continue
		}
		line = strings.Join(parts, " ")

	madeline:
		rl.SaveHistory(line)

		line = strings.TrimSuffix(line, ";")
		parts = parts[:0]
		rl.SetPrompt(cli.conf.Prompt)
		cli.Process(line)
		// TODO there is a timeing issue between prompt and stdout
		// without sleep, sometimes the prompt does not display on client's terminal.
		time.Sleep(50 * time.Millisecond)
	}
exit:
}

func trimLine(line string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
}

func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ: // block CtrlZ feature
		return r, false
	}
	return r, true
}

func (cli *client) Printer() *message.Printer {
	return message.NewPrinter(cli.conf.Lang)
}

var sqlHistory = make([]string, 0)
var sqlHistoryLock = sync.Mutex{}

func AddSqlHistory(sqlText string) {
	sqlHistoryLock.Lock()
	defer sqlHistoryLock.Unlock()

	if len(sqlHistory) > 10 {
		sqlHistory = sqlHistory[len(sqlHistory)-10:]
	}

	sqlHistory = append(sqlHistory, sqlText)
}

func SqlHistory(line string) []string {
	return sqlHistory
}
