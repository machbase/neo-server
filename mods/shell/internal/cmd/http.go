package cmd

import (
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "http",
		PcFunc: pcHttp,
		Action: doHttp,
		Desc:   "HTTP server management",
		Usage:  strings.ReplaceAll(helpHttp, "\t", "    "),
	})
}

const helpHttp = `    http command [options]
  commands:
    debug             show debug mode status
    set-debug <bool> [--log-latency=d]  set debug mode`

type HttpCmd struct {
	Debug struct {
	} `cmd:"" name:"debug"`
	SetDebug struct {
		Enable  string `arg:"" name:"enable"`
		Latency string `name:"log-latency" default:"-1"`
	} `cmd:"" name:"set-debug"`
	Help bool `kong:"-"`
}

func pcHttp() action.PrefixCompleterInterface {
	return action.PcItem("http")
}

func doHttp(ctx *action.ActionContext) {
	cmd := &HttpCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpHttp); cmd.Help = true; return nil })
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
	case "debug":
		doHttpDebug(ctx)
	case "set-debug <enable>":
		strEnable := strings.ToLower(cmd.SetDebug.Enable)
		enable := strEnable == "true" || strEnable == "on" || strEnable == "1"
		latency := int64(-1)
		if d, err := time.ParseDuration(cmd.SetDebug.Latency); err == nil {
			latency = int64(d)
		}

		doHttpDebugSet(ctx, enable, latency)
	}
}

func doHttpDebug(ctx *action.ActionContext) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	req := &mgmt.HttpDebugModeRequest{}
	req.Cmd = "get"
	rsp, err := mgmtClient.HttpDebugMode(ctx.Ctx, req)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	printHttpDebug(ctx, rsp)
}

func doHttpDebugSet(ctx *action.ActionContext, enable bool, latency int64) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	req := &mgmt.HttpDebugModeRequest{}
	req.Cmd = "set"
	req.Enable = enable
	req.LogLatency = latency

	rsp, err := mgmtClient.HttpDebugMode(ctx.Ctx, req)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	printHttpDebug(ctx, rsp)
}

func printHttpDebug(ctx *action.ActionContext, rsp *mgmt.HttpDebugModeResponse) {
	box := ctx.NewBox([]string{"NAME", "VALUE"})
	box.AppendRow("HTTP DEBUG ENABLED", rsp.Enable)
	box.AppendRow("HTTP DEBUG LOG LATENCY", time.Duration(rsp.LogLatency).String())
	box.Render()
}
