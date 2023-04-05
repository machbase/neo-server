package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/service/httpsvr"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "serve-web",
		PcFunc: pcServeWeb,
		Action: doServeWeb,
		Desc:   "Serve Web UI",
		Usage:  helpServeWeb,

		Experimental: true,
	})
}

const helpServeWeb = `  serve-web [options]
  options:
    -h,--host            bind address  (default: "127.0.0.1")
    -p,--port            bind port     (default: 5650)
       --prefix          web prefix    (default: "/")
       --log-filename    log file path (default: "-" stdout)
    -v,--verbose         verbose mode  (default: false)
`

type ServeWebCmd struct {
	Host        string `name:"host" short:"h" default:"127.0.0.1"`
	Port        int    `name:"port" short:"p" default:"5650"`
	Prefix      string `name:"prefix" default:"/"`
	LogFilename string `name:"logFilename" default:"-"`
	Verbose     bool   `name:"verbose" short:"v" default:"false"`
	Help        bool   `kong:"-"`
}

func pcServeWeb() readline.PrefixCompleterInterface {
	return readline.PcItem("serve-web")
}

func doServeWeb(ctx *client.ActionContext) {
	cmd := &ServeWebCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpServeWeb); cmd.Help = true; return nil })
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

	if cmd.Verbose {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

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
	r.Use(gin.CustomRecoveryWithWriter(logWriter, func(c *gin.Context, err any) {
		ctx.Println("Panic recovery")
		ctx.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	if cmd.Verbose {
		r.Use(weblogfunc(ctx))
	}
	webConf := &httpsvr.Config{
		Handlers: []httpsvr.HandlerConfig{
			{Prefix: cmd.Prefix, Handler: "web"},
		},
	}
	svr, err := httpsvr.New(ctx.DB, webConf)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	svr.Route(r)

	httpd := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cmd.Host, cmd.Port),
		Handler: r,
	}

	ctx.Printfln("serve-web > listening http://%s:%d%s", cmd.Host, cmd.Port, cmd.Prefix)

	serverQuit := make(chan bool, 1)

	if ctx.Interactive {
		go func() {
			rl, err := readline.NewEx(&readline.Config{
				Prompt:                 "serve-web > ",
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
			ctx.Println("serve-web > closing...")
			httpd.Shutdown(context.Background())
			<-serverQuit
			ctx.Println("serve-web > closed.")
		}()

		err = httpd.ListenAndServe()
		if err != nil && err.Error() != "http: Server closed" {
			ctx.Println("ERR", err.Error())
		}
		serverQuit <- true
	} else {
		httpd.ListenAndServe()
	}
}

func weblogfunc(ctx *client.ActionContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// ignore health checker
		if strings.HasSuffix(c.Request.URL.Path, "/healthz") && c.Request.Method == http.MethodGet {
			return
		}

		// Stop timer
		TimeStamp := time.Now()
		Latency := TimeStamp.Sub(start)

		StatusCode := c.Writer.Status()

		url := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if len(raw) > 0 {
			url = url + "?" + raw
		}

		ClientIP := c.ClientIP()
		Proto := c.Request.Proto
		Method := c.Request.Method
		ErrorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if len(ErrorMessage) > 0 {
			ErrorMessage = "\n" + ErrorMessage
		}
		ContentType := c.Writer.Header().Get("Content-Type")
		color := ""
		reset := "\033[0m"

		switch {
		case StatusCode >= http.StatusContinue && StatusCode < http.StatusOK:
			color, reset = "", "" // 1xx
		case StatusCode >= http.StatusOK && StatusCode < http.StatusMultipleChoices:
			color = "\033[97;42m" // 2xx green
		case StatusCode >= http.StatusMultipleChoices && StatusCode < http.StatusBadRequest:
			color = "\033[90;47m" // 3xx white
		case StatusCode >= http.StatusBadRequest && StatusCode < http.StatusInternalServerError:
			color = "\033[90;43m" // 4xx yellow
		default:
			color = "\033[97;41m" // 5xx red
		}

		Extra := ""
		if ContentType != "" {
			Extra = fmt.Sprintf("(%s)", ContentType)
		}

		ctx.Printfln("serve-web > %-15s | %s %3d %s| %13v | %s | %-7s %s%s %s",
			ClientIP,
			color, StatusCode, reset,
			Latency,
			Proto,
			Method,
			url,
			ErrorMessage,
			Extra,
		)
	}
}
