package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "bridge",
		PcFunc: pcBridge,
		Action: doBridge,
		Desc:   "Manage bridges",
		Usage:  helpBridge,
	})
}

const helpBridge = `  bridge command [options]
  commands:
    list                            shows registered bridges
    add [options] <name>  <conn>    add bridage
        options:
            -t,--type <type>        bridge type ['sqlite']
        args:
            name                    name of the connection
            conn                    connection string
    del   <name>                    remove bridage
    test  <name>                    test connectivity of the bridage
    exec  <name> <command>
`

type BridgeCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name string `arg:"" name:"name" help:"bridge name"`
		Path string `arg:"" name:"conn" help:"bridge connection string"`
		Type string `name:"type" short:"t" required:"" enum:"sqlite" help:"bridge type"`
	} `cmd:"" name:"add"`
	Test struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"test"`
	Exec struct {
		Name  string   `arg:"" name:"name"`
		Query []string `arg:"" name:"command" passthrough:""`
	} `cmd:"" name:"exec"`
	Help bool `kong:"-"`
}

func pcBridge() readline.PrefixCompleterInterface {
	return readline.PcItem("bridge",
		readline.PcItem("list"),
		readline.PcItem("add"),
		readline.PcItem("del"),
		readline.PcItem("exec"),
		readline.PcItem("test"),
	)
}

func doBridge(ctx *client.ActionContext) {
	cmd := &BridgeCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpBridge); cmd.Help = true; return nil })
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
		doBridgeList(ctx)
	case "add <name> <conn>":
		doBridgeAdd(ctx, cmd.Add.Name, cmd.Add.Type, cmd.Add.Path)
	case "del <name>":
		doBridgeDel(ctx, cmd.Del.Name)
	case "test <name>":
		doBridgeTest(ctx, cmd.Test.Name)
	case "exec <name> <command>":
		sqlText := util.StripQuote(strings.Join(cmd.Exec.Query, " "))
		doBridgeExec(ctx, cmd.Exec.Name, sqlText)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doBridgeList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.Client.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListBridge(ctx, &bridge.ListBridgeRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "CONNECTION"})
	for i, c := range rsp.Bridges {
		box.AppendRow(i+1, c.Name, c.Type, c.Path)
	}
	box.Render()
}

func doBridgeDel(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelBridge(ctx, &bridge.DelBridgeRequest{
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

func doBridgeAdd(ctx *client.ActionContext, name string, typ string, path string) {
	name = strings.ToLower(name)
	mgmtCli, err := ctx.Client.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddBridge(ctx, &bridge.AddBridgeRequest{
		Name: name, Type: typ, Path: path,
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

func doBridgeTest(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.TestBridge(ctx, &bridge.TestBridgeRequest{Name: name})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	ctx.Println("Test bridge", name, "connectivity...", rsp.Reason, rsp.Elapse)
}

func doBridgeExec(ctx *client.ActionContext, name string, command string) {
	bridgeRuntime, err := ctx.Client.BridgeRuntimeClient()
	if err != nil {
		ctx.Println("ERR bridge service is not avaliable;", err.Error())
		return
	}
	rsp, err := bridgeRuntime.Exec(ctx, &bridge.ExecRequest{Name: name, Command: command})
	if err != nil {
		ctx.Println("ERR", "Exec bridge", name, err.Error())
		return
	}
	defer bridgeRuntime.ResultClose(ctx, rsp.Result)

	// cols, err := rsp.Result.Columns(ctx)
	// if err != nil {
	// 	ctx.Println("ERR", "Exec connector columns", name, err.Error())
	// 	return
	// }

	// box := ctx.NewBox(append([]string{"ROWNUM"}, cols.Names()...))
	// for {
	// 	vals, err0 := rset.Fetch(ctx)
	// 	if err0 != nil {
	// 		err = err0
	// 		break
	// 	}
	// 	if vals == nil {
	// 		break
	// 	}
	// 	box.AppendRow(vals...)
	// }
	// box.Render()
	// if err != nil {
	// 	ctx.Println("ERR", err.Error())
	// }
}
