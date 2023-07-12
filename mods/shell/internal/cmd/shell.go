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
    del   <name>                        unregister shell
`

type ShellCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name    string   `arg:"" name:"name" help:"shell name"`
		Binpath []string `arg:"" name:"binpath" passthrough:""`
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
	case "del <name>":
		doShellDel(ctx, cmd.Del.Name)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doShellList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.Client.ManagementClient()
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

	box := ctx.NewBox([]string{"ROWNUM", "NAME", "COMMAND"})
	for i, c := range rsp.Shells {
		box.AppendRow(i+1, c.Name, strings.Join(c.Args, " "))
	}
	box.Render()
}

func doShellDel(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelShell(ctx, &mgmt.DelShellRequest{
		Name: name,
	})
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

func doShellAdd(ctx *client.ActionContext, name string, args []string) {
	name = strings.ToLower(name)
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddShell(ctx, &mgmt.AddShellRequest{
		Name: name, Args: args,
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
