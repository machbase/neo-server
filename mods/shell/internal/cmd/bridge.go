package cmd

import (
	"fmt"
	"strings"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:         "bridge",
		PcFunc:       pcBridge,
		Action:       doBridge,
		Desc:         "Manage bridges",
		Usage:        helpBridge,
		Experimental: true,
	})
}

const helpBridge = `  bridge command [options]
  commands:
    list                            shows registered bridges
    add [options] <name>  <conn>    add bridage
        options:
            -t,--type <type>        bridge type [ sqlite, postgres, mysql, mqtt ]
        args:
            name                    name of the connection
            conn                    connection string
    del      <name>                 remove bridage
    test     <name>                 test connectivity of the bridage
    exec     <name> <command>       execute on the bridge
    query    <name> <command>       query on the bridge
`

type BridgeCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name string `arg:"" name:"name" help:"bridge name"`
		Path string `arg:"" name:"conn" help:"bridge connection string"`
		Type string `name:"type" short:"t" required:"" enum:"sqlite,postgres,mysql,mqtt" help:"bridge type"`
	} `cmd:"" name:"add"`
	Test struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"test"`
	Exec struct {
		Name  string   `arg:"" name:"name"`
		Query []string `arg:"" name:"command" passthrough:""`
	} `cmd:"" name:"exec"`
	Query struct {
		Name  string   `arg:"" name:"name"`
		Query []string `arg:"" name:"command" passthrough:""`
	} `cmd:"" name:"query"`
	Help bool `kong:"-"`
}

func pcBridge() readline.PrefixCompleterInterface {
	return readline.PcItem("bridge",
		readline.PcItem("list"),
		readline.PcItem("add"),
		readline.PcItem("del"),
		readline.PcItem("test"),
		readline.PcItem("exec"),
		readline.PcItem("query"),
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
		doBridgeAdd(ctx, cmd.Add.Name, cmd.Add.Type, util.StripQuote(cmd.Add.Path))
	case "del <name>":
		doBridgeDel(ctx, cmd.Del.Name)
	case "test <name>":
		doBridgeTest(ctx, cmd.Test.Name)
	case "exec <name> <command>":
		sqlText := util.StripQuote(strings.Join(cmd.Exec.Query, " "))
		doBridgeExec(ctx, cmd.Exec.Name, sqlText)
	case "query <name> <command>":
		sqlText := util.StripQuote(strings.Join(cmd.Query.Query, " "))
		doBridgeQuery(ctx, cmd.Query.Name, sqlText)
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
	rsp, err := mgmtCli.ListBridge(ctx, &bridgerpc.ListBridgeRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"NAME", "TYPE", "CONNECTION"})
	for _, c := range rsp.Bridges {
		box.AppendRow(c.Name, c.Type, c.Path)
	}
	box.Render()
}

func doBridgeDel(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelBridge(ctx, &bridgerpc.DelBridgeRequest{
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
	rsp, err := mgmtCli.AddBridge(ctx, &bridgerpc.AddBridgeRequest{
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
	rsp, err := mgmtCli.TestBridge(ctx, &bridgerpc.TestBridgeRequest{Name: name})
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
	cmd := &bridgerpc.ExecRequest_SqlExec{SqlExec: &bridgerpc.SqlRequest{}}
	cmd.SqlExec.SqlText = command
	rsp, err := bridgeRuntime.Exec(ctx, &bridgerpc.ExecRequest{Name: name, Command: cmd})
	if err != nil {
		ctx.Println("ERR", "exec bridge", name, err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", "exec bridge fail", rsp.Reason)
		return
	}
	result := rsp.GetSqlExecResult()
	if result != nil {
		ctx.Println("executed.")
		return
	}
}

func doBridgeQuery(ctx *client.ActionContext, name string, command string) {
	bridgeRuntime, err := ctx.Client.BridgeRuntimeClient()
	if err != nil {
		ctx.Println("ERR bridge service is not avaliable;", err.Error())
		return
	}
	cmd := &bridgerpc.ExecRequest_SqlQuery{SqlQuery: &bridgerpc.SqlRequest{}}
	cmd.SqlQuery.SqlText = command
	rsp, err := bridgeRuntime.Exec(ctx, &bridgerpc.ExecRequest{Name: name, Command: cmd})
	if err != nil {
		ctx.Println("ERR", "query bridge", name, err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", "query bridge fail", rsp.Reason)
		return
	}
	result := rsp.GetSqlQueryResult()
	defer bridgeRuntime.SqlQueryResultClose(ctx, result)

	if rsp.Result != nil && len(result.Fields) == 0 {
		ctx.Println("executed.")
		return
	}

	header := []string{}
	for _, col := range result.Fields {
		header = append(header, col.Name)
	}
	rownum := 0
	box := ctx.NewBox(header)
	for {
		fetch, err0 := bridgeRuntime.SqlQueryResultFetch(ctx, result)
		if err0 != nil {
			err = err0
			break
		}
		if !fetch.Success {
			err = fmt.Errorf("fetch failed; %s", fetch.Reason)
			break
		}
		if fetch.HasNoRows {
			break
		}
		rownum++
		vals, err0 := bridge.ConvertFromDatum(fetch.Values...)
		if err0 != nil {
			err = err0
			break
		}
		box.AppendRow(vals...)
	}
	box.Render()
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
}
