package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "shell",
		PcFunc: pcShell,
		Action: doShell,
		Desc:   "Manage shell commands",
		Usage:  helpShell,
	})
}

const helpShell = `  shell command [options]
  commands:
    list                                shows registered shells
    add   <name>  <binpath [args...]>   register shell
    del   <id>                          unregister shell by given id

   ex)
      shell add console  C:\Windows\System32\cmd.exe
      shell add bashterm /bin/bash
      shell add terminal /bin/zsh -il
    ex)
      shell del D85CE6E8-24FA-11EE-9B7A-8A17CAD8D69C
`

type ShellCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Id string `arg:"" name:"id"`
	} `cmd:"" name:"del"`
	Add struct {
		Name    string `arg:"" name:"name" help:"shell name"`
		Binpath string `arg:"" name:"binpath" passthrough:""`
	} `cmd:"" name:"add"`
	Help bool `kong:"-"`
}

func pcShell() readline.PrefixCompleterInterface {
	return readline.PcItem("shell",
		readline.PcItem("list"),
		readline.PcItem("add"),
		readline.PcItem("del"),
	)
}

func doShell(ctx *client.ActionContext) {
	cmd := &ShellCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpShell); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	parseCtx, err := parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	switch parseCtx.Command() {
	case "list":
		doShellList(ctx)
	case "add <name> <binpath>":
		doShellAdd(ctx, cmd.Add.Name, cmd.Add.Binpath)
	case "del <id>":
		doShellDel(ctx, cmd.Del.Id)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doShellList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListShell(ctx, &mgmt.ListShellRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"ROWNUM", "NAME", "ID", "COMMAND"})
	for i, c := range rsp.Shells {
		box.AppendRow(i+1, c.Name, c.Id, c.Command)
	}
	box.Render()
}

func doShellDel(ctx *client.ActionContext, id string) {
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	rsp, err := mgmtCli.DelShell(ctx, &mgmt.DelShellRequest{Id: id})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}
	ctx.Println("deleted")
}

func doShellAdd(ctx *client.ActionContext, name string, command string) {
	name = strings.ToLower(name)
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddShell(ctx, &mgmt.AddShellRequest{
		Name: name, Command: command,
	})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}
	ctx.Println("added")
}
