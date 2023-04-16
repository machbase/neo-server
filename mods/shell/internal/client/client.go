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

	"github.com/chzyer/readline"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"golang.org/x/net/context"
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
	ServerAddr   string
	Stdin        io.ReadCloser
	Stdout       io.Writer
	Stderr       io.Writer
	Prompt       string
	PromptCont   string
	QueryTimeout time.Duration
	Lang         language.Tag
}

type client struct {
	conf *Config
	db   spi.DatabaseClient
	pref *Pref

	interactive   bool
	remoteSession bool
}

func DefaultConfig() *Config {
	return &Config{
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Prompt:       "\033[31mmachbase-neo»\033[0m ",
		PromptCont:   "\033[37m>\033[0m  ",
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
		cli.db.Disconnect()
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

	machcli := machrpc.NewClient()
	err := machcli.Connect(cli.conf.ServerAddr, machrpc.QueryTimeout(cli.conf.QueryTimeout))
	if err != nil {
		return err
	}

	// check connectivity to server
	serverInfo, err := machcli.GetServerInfo()
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

	conn, err := machrpc.MakeGrpcConn(cli.conf.ServerAddr)
	if err != nil {
		return err
	}
	mgmtcli := mgmt.NewManagementClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = mgmtcli.Shutdown(ctx, &mgmt.ShutdownRequest{})
	if err != nil {
		return err
	}
	return nil
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

type ActionContext struct {
	Line         string
	Client       Client
	DB           spi.Database
	Lang         language.Tag
	TimeLocation *time.Location
	TimeFormat   string
	Interactive  bool

	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer

	parent     context.Context
	cancelFunc func()
	cli        *client
}

func (ctx *ActionContext) Deadline() (deadline time.Time, ok bool) {
	return ctx.parent.Deadline()
}

func (ctx *ActionContext) Done() <-chan struct{} {
	return ctx.parent.Done()
}

func (ctx *ActionContext) Err() error {
	return ctx.parent.Err()
}

func (ctx *ActionContext) Value(key any) any {
	return ctx.parent.Value(key)
}

func (ctx *ActionContext) Cancel() {
	ctx.cancelFunc()
}

func (ctx *ActionContext) Write(p []byte) (int, error) {
	return ctx.Client.Write(p)
}
func (ctx *ActionContext) Print(args ...any) {
	ctx.Client.Print(args...)
}
func (ctx *ActionContext) Printf(format string, args ...any) {
	ctx.Client.Printf(format, args...)
}
func (ctx *ActionContext) Println(args ...any) {
	ctx.Client.Println(args...)
}
func (ctx *ActionContext) Printfln(format string, args ...any) {
	ctx.Client.Printfln(format, args...)
}

func (ctx *ActionContext) Config() *Config {
	return ctx.cli.conf
}

func (ctx *ActionContext) Pref() *Pref {
	return ctx.cli.pref
}

func (ctx *ActionContext) NewManagementClient() (mgmt.ManagementClient, error) {
	conn, err := machrpc.MakeGrpcConn(ctx.cli.conf.ServerAddr)
	if err != nil {
		return nil, err
	}
	return mgmt.NewManagementClient(conn), nil
}

// ShutdownServerFunc returns callable function to shutdown server if this instance has ability of shutdown server
// otherwise return nil
func (ctx *ActionContext) ShutdownServerFunc() ShutdownServerFunc {
	return ctx.cli.ShutdownServer
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

	cmdName := fields[0]
	var cmd *Cmd
	var ok bool
	if cmd, ok = commands[cmdName]; ok {
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

	actCtx := &ActionContext{
		Line:         line,
		Client:       cli,
		DB:           cli.db,
		Lang:         cli.conf.Lang,
		TimeLocation: time.UTC,
		TimeFormat:   "ns",
		Interactive:  cli.interactive,
		Stdin:        cli.conf.Stdin,
		Stdout:       cli.conf.Stdout,
		Stderr:       cli.conf.Stderr,
	}
	actCtx.parent, actCtx.cancelFunc = context.WithCancel(context.Background())
	actCtx.cli = cli

	defer actCtx.cancelFunc()

	cmd.Action(actCtx)
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
		Stdin:                  cli.conf.Stdin,
		Stdout:                 cli.conf.Stdout,
		Stderr:                 cli.conf.Stderr,
		HistorySearchFold:      true,
		FuncFilterInputRune:    filterInput,
	}

	if runtime.GOOS == "windows" {
		// TODO on windows, up/down arrow keys for the history is not working if stdin is set
		readlineCfg.Stdin = nil
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
			if len(line) == 0 {
				break
			} else {
				continue
			}
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
			if line == "exit" || line == "exit;" || line == "quit" || line == "quit;" {
				goto exit
			} else if strings.HasPrefix(line, "help") {
				goto madeline
			} else if line == "set" || strings.HasPrefix(line, "set ") {
				goto madeline
			}
		}

		parts = append(parts, line)
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
	}
exit:
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
