package cmd

import (
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "ping",
		PcFunc: pcPing,
		Action: doPing,
		Desc:   "Test connection to server",
		Usage:  helpPing,
	})
}

const helpPing string = `  ping                      Test connection to server`

// const helpPing string = `  ping  [options]           Test connection to server
//   options:
//     -n,--count <count>      repeat count (default:1 )
// `

type PingCmd struct {
	Repeat int  `name:"repeat" short:"n" default:"1"`
	Help   bool `kong:"-"`
}

func pcPing() action.PrefixCompleterInterface {
	return action.PcItem("ping")
}

func doPing(ctx *action.ActionContext) {
	cmd := &PingCmd{}
	parser, err := action.Kong(cmd, func() error { fmt.Println(helpPing); cmd.Help = true; return nil })
	if err != nil {
		fmt.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		fmt.Println("ERR", err.Error())
		return
	}

	for i := 0; i < cmd.Repeat && ctx.Ctx.Err() == nil; i++ {
		if i != 0 {
			time.Sleep(time.Second)
		}
		latency, err := ctx.Actor.Database().Ping(ctx.Ctx)
		if err != nil {
			fmt.Println("ping", err.Error())
		} else {
			fmt.Printf("seq=%d time=%s\n", i, latency)
		}
	}
}
