package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "shell",
		PcFunc: pcShell,
		Action: doShell,
		Desc:   "Manage shell commands",
		Usage:  strings.ReplaceAll(helpShell, "\t", "    "),
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
      shell del D85CE6E8-24FA-11EE-9B7A-8A17CAD8D69C`

type ShellCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Id string `arg:"" name:"id"`
	} `cmd:"" name:"del"`
	Add struct {
		Name    string   `arg:"" name:"name" help:"shell name"`
		Binpath []string `arg:"" name:"binpath" passthrough:""`
	} `cmd:"" name:"add"`
	Help bool `kong:"-"`
}

func pcShell() action.PrefixCompleterInterface {
	return action.PcItem("shell",
		action.PcItem("list"),
		action.PcItem("add"),
		action.PcItem("del"),
	)
}

func doShell(ctx *action.ActionContext) {
	cmd := &ShellCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpShell); cmd.Help = true; return nil })
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

func doShellList(ctx *action.ActionContext) {
	mgmtCli, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListShell(ctx.Ctx, &mgmt.ListShellRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"ROWNUM", "ID", "NAME", "COMMAND"})
	for i, c := range rsp.Shells {
		box.AppendRow(i+1, c.Id, c.Name, c.Command)
	}
	box.Render()
}

func doShellDel(ctx *action.ActionContext, id string) {
	mgmtCli, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	rsp, err := mgmtCli.DelShell(ctx.Ctx, &mgmt.DelShellRequest{Id: id})
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

func doShellAdd(ctx *action.ActionContext, name string, args []string) {
	if len(args) == 0 {
		ctx.Println("ERR shell command should be specified")
		return
	}
	name = strings.ToLower(name)
	mgmtCli, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	command := strings.Join(args, " ")
	rsp, err := mgmtCli.AddShell(ctx.Ctx, &mgmt.AddShellRequest{
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
