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
		Name:   "connector",
		PcFunc: pcConnector,
		Action: doConnector,
		Desc:   "Manage connectors",
		Usage:  helpConnector,
	})
}

const helpConnector = `  connector command [options]
  commands:
    list                           shows registered connectors
    add [options] <name>  <conn>   add connector
        options:
            -t,--type <type>        connector type ['sqlite']
        args:
            name                    name of the connection
            conn                    connection string
    del   <name>                    remove connector
    test  <name>                    test connectivity of the connector
    exec  <name> <command>
`

type ConnectorCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name string `arg:"" name:"name" help:"connector name"`
		Path string `arg:"" name:"conn" help:"connector connection string"`
		Type string `name:"type" short:"t" required:"" enum:"sqlite" help:"connector type"`
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

func pcConnector() readline.PrefixCompleterInterface {
	return readline.PcItem("connector",
		readline.PcItem("list"),
		readline.PcItem("add"),
		readline.PcItem("del"),
		readline.PcItem("exec"),
		readline.PcItem("test"),
	)
}

func doConnector(ctx *client.ActionContext) {
	cmd := &ConnectorCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpConnector); cmd.Help = true; return nil })
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
		doConnectorList(ctx)
	case "add <name> <conn>":
		doConnectorAdd(ctx, cmd.Add.Name, cmd.Add.Type, cmd.Add.Path)
	case "del <name>":
		doConnectorDel(ctx, cmd.Del.Name)
	case "test <name>":
		doConnectorTest(ctx, cmd.Test.Name)
	case "exec <name> <command>":
		sqlText := util.StripQuote(strings.Join(cmd.Exec.Query, " "))
		doConnectorExec(ctx, cmd.Exec.Name, sqlText)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doConnectorList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListConnector(ctx, &mgmt.ListConnectorRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "CONNECTION"})
	for i, c := range rsp.Connectors {
		box.AppendRow(i+1, c.Name, c.Type, c.Path)
	}
	box.Render()
}

func doConnectorDel(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelConnector(ctx, &mgmt.DelConnectorRequest{
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

func doConnectorAdd(ctx *client.ActionContext, name string, typ string, path string) {
	name = strings.ToLower(name)
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddConnector(ctx, &mgmt.AddConnectorRequest{
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

func doConnectorTest(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.TestConnector(ctx, &mgmt.TestConnectorRequest{Name: name})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	ctx.Println("Test connector", name, "...", rsp.Reason, rsp.Elapse)
}

func doConnectorExec(ctx *client.ActionContext, name string, query string) {
	var connector client.Connector
	if con, err := ctx.Client.ConnectorClient(ctx, name); err != nil {
		ctx.Println("ERR connector service is not avaliable;", err.Error())
		return
	} else {
		connector = con
	}
	rset, err := connector.Exec(ctx, query)
	if err != nil {
		ctx.Println("ERR", "Exec connector", name, err.Error())
		return
	}
	defer rset.Close(ctx)

	cols, err := rset.Columns(ctx)
	if err != nil {
		ctx.Println("ERR", "Exec connector columns", name, err.Error())
		return
	}

	box := ctx.NewBox(append([]string{"ROWNUM"}, cols.Names()...))
	box.Render()
}
