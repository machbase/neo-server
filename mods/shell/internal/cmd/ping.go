package cmd

import (
	"time"

	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "ping",
		PcFunc: pcPing,
		Action: doPing,
		Desc:   "Test connection to server",
		Usage:  helpPing,
	})
}

const helpPing string = `  ping                      Test connection to server
`

// const helpPing string = `  ping  [options]           Test connection to server
//   options:
//     -n,--count <count>      repeat count (default:1 )
// `

type PingCmd struct {
	Repeat int  `name:"repeat" short:"n" default:"1"`
	Help   bool `kong:"-"`
}

func pcPing() readline.PrefixCompleterInterface {
	return readline.PcItem("ping")
}

func doPing(ctx *client.ActionContext) {
	cmd := &PingCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpPing); cmd.Help = true; return nil })
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

	if pinger, ok := ctx.Conn.(spi.Pinger); ok {
		for i := 0; i < cmd.Repeat; i++ {
			if i != 0 {
				time.Sleep(time.Second)
			}
			latency, err := pinger.Ping()
			if err != nil {
				ctx.Println("ping", err.Error())
			} else {
				ctx.Printfln("seq=%d time=%s", i, latency)
			}
		}
	} else {
		ctx.Println("ping is not avaliable")
	}
}
