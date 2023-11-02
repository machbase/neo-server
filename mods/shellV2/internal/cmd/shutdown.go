package cmd

import (
	client "github.com/machbase/neo-server/mods/shellV2/internal/action"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "shutdown",
		PcFunc: pcShutdown,
		Action: doShutdown,
		Desc:   "Shutdown server process",
		Usage:  helpShutdown,
	})
}

const helpShutdown string = `  shutdown    stop the server process
`

type ShutdownCmd struct {
	Interactive bool `kong:"-"`
	Help        bool `kong:"-"`
}

func pcShutdown() client.PrefixCompleterInterface {
	return client.PcItem("shutdown")
}

func doShutdown(ctx *client.ActionContext) {
	f := ctx.ShutdownServerFunc()
	if f == nil {
		ctx.Println("ERR", "server shutdown is not allowed")
	} else {
		err := f()
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		ctx.Println("server shutting down...")
	}
}
