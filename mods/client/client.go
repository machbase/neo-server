package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-grpc/machrpc"
)

type Client interface {
	Close()
	RunInteractive()
	RunSql(sqlText string)
}

type Config struct {
	ServerAddr string
	Stdin      io.ReadCloser
	Stdout     io.Writer
	VimMode    bool
}

type client struct {
	conf   *Config
	stdin  io.ReadCloser
	stdout io.Writer
	db     *machrpc.Client
}

func New(conf *Config) (Client, error) {
	machcli := machrpc.NewClient(machrpc.QueryTimeout(10 * time.Second))
	err := machcli.Connect(conf.ServerAddr)
	if err != nil {
		return nil, err
	}
	cli := &client{
		conf:   conf,
		stdin:  conf.Stdin,
		stdout: conf.Stdout,
		db:     machcli,
	}
	return cli, nil
}

func (cli *client) Close() {
	if cli.db != nil {
		cli.db.Disconnect()
	}
}

func (cli *client) SetInput(r io.ReadCloser) {
	cli.stdin = r
}

func (cli *client) Input() io.ReadCloser {
	return cli.stdin
}

func (cli *client) SetOutput(w io.Writer) {
	cli.stdout = w
}

func (cli *client) Output() io.Writer {
	return cli.stdout
}

func (cli *client) Println(args ...any) {
	fmt.Fprintln(cli.stdout, args...)
}

func (cli *client) Printf(format string, args ...any) {
	fmt.Fprintf(cli.stdout, format, args...)
}

func (cli *client) Writeln(args ...any) {
	fmt.Fprintln(cli.stdout, args...)
}

func (cli *client) Writef(format string, args ...any) {
	fmt.Fprintf(cli.stdout, format+"\r\n", args...)
}

func (cli *client) RunSql(sqlText string) {
	cli.doSql(sqlText)
}

func (cli *client) RunInteractive() {
	completer := cli.completer()
	prompt := "\033[31mmachsql»\033[0m "
	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 prompt,
		HistoryFile:            "/tmp/readline.tmp",
		DisableAutoSaveHistory: true,
		AutoComplete:           completer,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		Stdin:                  cli.stdin,
		Stdout:                 cli.stdout,
		HistorySearchFold:      true,
		FuncFilterInputRune:    filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	rl.CaptureExitSignal()
	rl.SetVimMode(cli.conf.VimMode)

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
			continue
		}
		if line == "exit" || line == "exit;" {
			goto exit
		} else if strings.HasPrefix(line, "help") {
			if !strings.HasSuffix(line, ";") {
				line = line + ";"
			}
		}
		parts = append(parts, line)
		if !strings.HasSuffix(line, ";") {
			rl.SetPrompt("         ")
			continue
		}

		line = strings.Join(parts, " ")
		rl.SaveHistory(line)

		line = strings.TrimSuffix(line, ";")
		parts = parts[:0]
		rl.SetPrompt(prompt)

		fields := splitFields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "help":
			cmd := strings.TrimSpace(strings.ToLower(line[4:]))
			usage(cli.stdout, completer, cmd)
		case "show":
			obj := strings.TrimSpace(strings.ToLower(line[4:]))
			cli.doShow(obj)
		case "explain":
			sql := line[7:]
			cli.doExplain(sql)
		default:
			cli.doSql(line)
		}
	}
exit:
}

func splitFields(line string) []string {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}
	return strings.FieldsFunc(strings.ToLower(line), f)
}

func usage(w io.Writer, completer *readline.PrefixCompleter, cmd string) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ: // block CtrlZ feature
		return r, false
	}
	return r, true
}

func (cli *client) completer() *readline.PrefixCompleter {
	var completer = readline.NewPrefixCompleter(
		readline.PcItem("show",
			readline.PcItem("tables"),
			readline.PcItem("runtime"),
			readline.PcItem("version"),
		),
		readline.PcItem("explain"),
		readline.PcItem("from",
			readline.PcItemDynamic(cli.listTables()),
		),
		readline.PcItem("mode",
			readline.PcItem("vi"),
			readline.PcItem("emacs"),
		),
		readline.PcItem("login"),
		readline.PcItem("say",
			readline.PcItemDynamic(listFiles("./"),
				readline.PcItem("with",
					readline.PcItem("following"),
					readline.PcItem("items"),
				),
			),
			readline.PcItem("hello"),
			readline.PcItem("bye"),
		),
		readline.PcItem("setprompt"),
		readline.PcItem("setpassword"),
		readline.PcItem("bye"),
		readline.PcItem("help"),
		readline.PcItem("go",
			readline.PcItem("build", readline.PcItem("-o"), readline.PcItem("-v")),
			readline.PcItem("install",
				readline.PcItem("-v"),
				readline.PcItem("-vv"),
				readline.PcItem("-vvv"),
			),
			readline.PcItem("test"),
		),
		readline.PcItem("sleep"),
	)
	return completer
}

// Function constructor - constructs new function for listing given directory
func listFiles(path string) func(string) []string {
	return func(line string) []string {
		names := make([]string, 0)
		files, _ := ioutil.ReadDir(path)
		for _, f := range files {
			names = append(names, f.Name())
		}
		return names
	}
}

func (cli *client) listTables() func(string) []string {
	return func(line string) []string {
		rows, err := cli.db.Query("select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
		if err != nil {
			// sess.log.Errorf("select m$sys_tables fail; %s", err.Error())
			return nil
		}
		defer rows.Close()
		// rt := []prompt.Suggest{}
		rt := []string{}
		for rows.Next() {
			var name string
			var typ int
			var flg int
			rows.Scan(&name, &typ, &flg)
			//desc := tableTypeDesc(typ, flg)
			// rt = append(rt, prompt.Suggest{Text: name, Description: desc})
			rt = append(rt, name)
		}
		return rt
	}
}
