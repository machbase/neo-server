package cmd

import (
	"strings"

	"github.com/machbase/neo-server/mods/shell/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "connect",
		PcFunc: pcConnect,
		Action: doConnect,
		Desc:   "Reconnect to another user",
		Usage:  strings.ReplaceAll(helpConnect, "\t", "    "),
	})
}

const helpConnect = `  connect <username>/<password>`

func pcConnect() action.PrefixCompleterInterface {
	return action.PcItem("connect")
}

type ConnectCmd struct {
	Identifier string `arg:"" name:"username/password"`
	Help       bool   `kong:"-"`
}

func doConnect(ctx *action.ActionContext) {
	cmd := &ConnectCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpConnect); cmd.Help = true; return nil })
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
	var username string
	var password string
	if toks := strings.SplitN(cmd.Identifier, "/", 2); len(toks) == 2 {
		username = toks[0]
		password = toks[1]
	} else {
		ctx.Println("ERR", "no username/password is specified")
		return
	}

	ok, err := ctx.Actor.Reconnect(username, password)
	if err != nil || !ok {
		ctx.Println("ERR", err.Error())
		return
	}
	ctx.Println("Connected successfully.")
}
