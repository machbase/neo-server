package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	bridgerpc "github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "bridge",
		PcFunc: pcBridge,
		Action: doBridge,
		Desc:   "Manage bridges",
		Usage:  strings.ReplaceAll(helpBridge, "\t", "    "),
	})
}

const helpBridge = `  bridge command [options]
  commands:
    list                           shows registered bridges
    add [options] <name>  <conn>   add bridge
        options:
            -t,--type <type>       bridge type [ sqlite, mqtt, ... (see below) ]
        args:
            name                   name of the connection
            conn                   connection string
    del     <name>                 remove bridge
    test    <name>                 test connectivity of the bridge
    exec    <name> <command>       execute command on the bridge
    query   <name> <command>       query the bridge with command

  bridge types (-t,--type):
    sqlite        SQLite            https://sqlite.org
	    ex) bridge add -t sqlite my_memory file::memory:?cache=shared
			bridge add -t sqlite my_sqlite file:/tmp/sqlitefile.db
	postgres      PostgreSQL        https://postgresql.org
	    ex) bridge add -t postgres my_pg host=127.0.0.1 port=5432 user=dbuser dbname=postgres sslmode=disable
	mysql         MySQL             https://mysql.com
		ex) bridge add -t mysql my_sql root:passwd@tcp(127.0.0.1:3306)/testdb?parseTime=true
	mqtt          MQTT (v3.1.1)     https://mqtt.org
		ex) bridge add -t mqtt my_mqtt broker=127.0.0.1:1883 id=client-id
`

// mssql         MSSQL
//      ex) bridge add -t mssql  ms server=127.0.0.1:1433 user=sa pass=changeme database=master connection-timeout=5 dial-timeout=3 encrypt=disable
// python        Python            https://python.org
// 		ex) bridge add -t python py-local bin=/usr/local/bin/python3
// 		ex) bridge add -t python py-myenv bin=/bin/python dir=/work env="API_KEY=api_token" env="VAR=VALUE"

type BridgeCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Add struct {
		Name string   `arg:"" name:"name" help:"bridge name"`
		Path []string `arg:"" name:"conn" passthrough:"" help:"connection string"`
		Type string   `name:"type" short:"t" required:"" enum:"sqlite,postgres,mysql,mssql,mqtt,python" help:"bridge type"`
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

func pcBridge() action.PrefixCompleterInterface {
	return action.PcItem("bridge",
		action.PcItem("list"),
		action.PcItem("add",
			action.PcItem("--type",
				action.PcItem("sqlite"),
				action.PcItem("poastgres"),
				action.PcItem("mysql"),
				action.PcItem("mqtt"),
				action.PcItem("python"),
			)),
		action.PcItem("del"),
		action.PcItem("test"),
		action.PcItem("exec"),
		action.PcItem("query"),
	)
}

func doBridge(ctx *action.ActionContext) {
	cmd := &BridgeCmd{}
	parser, err := action.Kong(cmd, func() error {
		ctx.Println(strings.ReplaceAll(helpBridge, "\t", "    "))
		cmd.Help = true
		return nil
	})
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
		doBridgeAdd(ctx, cmd.Add.Name, cmd.Add.Type, util.StripQuote(strings.Join(cmd.Add.Path, " ")))
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

func doBridgeList(ctx *action.ActionContext) {
	mgmtCli, err := ctx.Actor.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListBridge(ctx.Ctx, &bridgerpc.ListBridgeRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	sort.Slice(rsp.Bridges, func(i, j int) bool { return rsp.Bridges[i].Name < rsp.Bridges[j].Name })

	box := ctx.NewBox([]string{"NAME", "TYPE", "CONNECTION"})
	for _, c := range rsp.Bridges {
		box.AppendRow(c.Name, c.Type, c.Path)
	}
	box.Render()
}

func getBridgeType(ctx *action.ActionContext, name string) (string, error) {
	mgmtCli, err := ctx.Actor.BridgeManagementClient()
	if err != nil {
		return "", err
	}
	rsp, err := mgmtCli.ListBridge(ctx.Ctx, &bridgerpc.ListBridgeRequest{})
	if err != nil {
		return "", err
	}
	if !rsp.Success {
		return "", errors.New(rsp.Reason)
	}
	for _, c := range rsp.Bridges {
		if strings.EqualFold(c.Name, name) {
			return c.Type, nil
		}
	}
	return "", fmt.Errorf("bridge '%s' not found", name)
}

func doBridgeDel(ctx *action.ActionContext, name string) {
	mgmtCli, err := ctx.Actor.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelBridge(ctx.Ctx, &bridgerpc.DelBridgeRequest{
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

func doBridgeAdd(ctx *action.ActionContext, name string, typ string, path string) {
	name = strings.ToLower(name)
	mgmtCli, err := ctx.Actor.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddBridge(ctx.Ctx, &bridgerpc.AddBridgeRequest{
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

func doBridgeTest(ctx *action.ActionContext, name string) {
	mgmtCli, err := ctx.Actor.BridgeManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.TestBridge(ctx.Ctx, &bridgerpc.TestBridgeRequest{Name: name})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	ctx.Println("Test bridge", name, "connectivity...", rsp.Reason, rsp.Elapse)
}

func doBridgeExec(ctx *action.ActionContext, name string, command string) {
	bridgeRuntime, err := ctx.Actor.BridgeRuntimeClient()
	if err != nil {
		ctx.Println("ERR bridge service is not avaliable;", err.Error())
		return
	}
	brType, err := getBridgeType(ctx, name)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	switch brType {
	case "python":
		cmd := &bridgerpc.ExecRequest_Invoke{Invoke: &bridgerpc.InvokeRequest{}}
		cmd.Invoke.Args = []string{command}
		rsp, err := bridgeRuntime.Exec(ctx.Ctx, &bridgerpc.ExecRequest{Name: name, Command: cmd})
		result := rsp.GetInvokeResult()
		if result != nil && len(result.Stdout) > 0 {
			ctx.Println("stdout", string(result.Stdout))
		}
		if result != nil && len(result.Stderr) > 0 {
			ctx.Println("stderr", string(result.Stderr))
		}
		if err != nil {
			ctx.Println("ERR", "exec bridge", name, err.Error())
			return
		}
		if !rsp.Success {
			ctx.Println("ERR", "exec bridge fail,", rsp.Reason)
			return
		}
	default:
		cmd := &bridgerpc.ExecRequest_SqlExec{SqlExec: &bridgerpc.SqlRequest{}}
		cmd.SqlExec.SqlText = command
		rsp, err := bridgeRuntime.Exec(ctx.Ctx, &bridgerpc.ExecRequest{Name: name, Command: cmd})
		if err != nil {
			ctx.Println("ERR", "exec bridge", name, err.Error())
			return
		}
		if !rsp.Success {
			ctx.Println("ERR", "exec bridge fail,", rsp.Reason)
			return
		}
		result := rsp.GetSqlExecResult()
		if result != nil {
			ctx.Println("executed.")
		}
	}
}

func doBridgeQuery(ctx *action.ActionContext, name string, command string) {
	bridgeRuntime, err := ctx.Actor.BridgeRuntimeClient()
	if err != nil {
		ctx.Println("ERR bridge service is not avaliable;", err.Error())
		return
	}
	cmd := &bridgerpc.ExecRequest_SqlQuery{SqlQuery: &bridgerpc.SqlRequest{}}
	cmd.SqlQuery.SqlText = command
	rsp, err := bridgeRuntime.Exec(ctx.Ctx, &bridgerpc.ExecRequest{Name: name, Command: cmd})
	if err != nil {
		ctx.Println("ERR", "query bridge", name, err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", "query bridge fail", rsp.Reason)
		return
	}
	result := rsp.GetSqlQueryResult()
	defer bridgeRuntime.SqlQueryResultClose(ctx.Ctx, result)

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
		fetch, err0 := bridgeRuntime.SqlQueryResultFetch(ctx.Ctx, result)
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
