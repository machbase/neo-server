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

const helpConnector = `  connector command [options] [args...]
  commands:
    list         list registered connectors
    del <name>   remove connector
    add <name>   add connector
    test <name>  test connectivity of the connector
  options:
    -t,--type <type>   connector type ['sqlite']
    -c,--path <string> connection string
`

type ConnectorCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name string `arg:"" name:"name"`
		Type string `name:"type" short:"t" required:"" enum:"sqlite" help:"connector type"`
		Path string `name:"path" short:"c" help:"connector connection string"`
	} `cmd:"" name:"add"`
	Test struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"test"`
	Help bool `kong:"-"`
}

func pcConnector() readline.PrefixCompleterInterface {
	return readline.PcItem("connector")
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
	case "add <name>":
		doConnectorAdd(ctx, cmd.Add.Name, cmd.Add.Type, cmd.Add.Path)
	case "del <name>":
		doConnectorDel(ctx, cmd.Del.Name)
	case "test <name>":
		doConnectorTest(ctx, cmd.Test.Name)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doConnectorList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.NewManagementClient()
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
	mgmtCli, err := ctx.NewManagementClient()
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
	mgmtCli, err := ctx.NewManagementClient()
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
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.TestConnector(ctx, &mgmt.TestConnectorRequest{Name: name})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	ctx.Println("Testing connector", name, rsp.Reason, rsp.Elapse)
}
