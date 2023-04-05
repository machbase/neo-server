package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/service/coapsvr"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "serve-coap",
		PcFunc: pcServeCoap,
		Action: doServeCoap,
		Desc:   "Serve CoAP",
		Usage:  helpServeCoap,

		Experimental: true,
	})
}

const helpServeCoap = `  serve-coap [options]
  options:
    -n,--network         network [tcp|udp|tls|dtls] (default: "udp")
    -h,--host            bind address  (default: "127.0.0.1")
    -p,--port            bind port     (default: 5659)
       --prefix          web prefix    (default: "/")
       --log-filename    log file path (default: "-" stdout)
    -v,--verbose         verbose mode  (default: false)
`

type ServeCoapCmd struct {
	Network     string `name:"network" short:"n" enum:"tcp,udp,tls,dtsl" default:"tcp"`
	Host        string `name:"host" short:"h" default:"127.0.0.1"`
	Port        int    `name:"port" short:"p" default:"5659"`
	Prefix      string `name:"prefix" default:"/"`
	LogFilename string `name:"log-filename" default:"-"`
	Verbose     bool   `name:"verbose" short:"v" default:"false"`
	Help        bool   `kong:"-"`
}

func pcServeCoap() readline.PrefixCompleterInterface {
	return readline.PcItem("serve-coap")
}

func doServeCoap(ctx *client.ActionContext) {
	cmd := &ServeCoapCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpServeCoap); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if cmd.Prefix == "" {
		cmd.Prefix = "/"
	}

	var logWriter io.Writer
	if cmd.LogFilename == "-" {
		logWriter = ctx.Stdout
	} else if len(cmd.LogFilename) > 0 {
		lf, err := os.OpenFile(cmd.LogFilename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		defer lf.Close()
		logWriter = lf
	} else {
		logWriter = io.Discard
	}

	if cmd.Verbose && logWriter != ctx.Stdout {
		logWriter = io.MultiWriter(logWriter, &verboseWriter{ctx: ctx})
	}
	conf := &coapsvr.Config{
		ListenAddress: []string{
			fmt.Sprintf("%s://%s:%d", cmd.Network, cmd.Host, cmd.Port),
		},
		LogWriter: logWriter,
	}

	csvr, err := coapsvr.New(ctx.DB, conf)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	prompt := "\033[33mserve-coap >\033[0m"
	ctx.Printfln("%s listening %s://%s:%d%s", prompt, cmd.Network, cmd.Host, cmd.Port, cmd.Prefix)

	serverQuit := make(chan bool, 1)

	if ctx.Interactive {
		go func() {
			rl, err := readline.NewEx(&readline.Config{
				Prompt:                 prompt + " ",
				DisableAutoSaveHistory: true,
				InterruptPrompt:        "^C",
				Stdin:                  ctx.Stdin,
				Stdout:                 ctx.Stdout,
				Stderr:                 ctx.Stderr,
			})
			if err != nil {
				panic(err)
			}
			defer rl.Close()
			rl.CaptureExitSignal()
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
				if line == "exit" {
					break
				}
			}
			ctx.Printfln("%s closing...", prompt)
			csvr.Stop()
			serverQuit <- true
			ctx.Printfln("%s closed.", prompt)
		}()

		err = csvr.Start()
		if err != nil && err.Error() != "coap: Server closed" {
			ctx.Println("ERR", err.Error())
		}
		<-serverQuit
	} else {
		csvr.Start()
	}
}

type verboseWriter struct {
	ctx *client.ActionContext
}

func (w *verboseWriter) Write(p []byte) (int, error) {
	w.ctx.Println(string(p))
	return len(p), nil
}
