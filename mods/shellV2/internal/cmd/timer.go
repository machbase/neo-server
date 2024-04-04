package cmd

import (
	"fmt"
	"strings"

	schedrpc "github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:         "timer",
		PcFunc:       pcTimer,
		Action:       doTimer,
		Desc:         "Manage schedule of timers",
		Usage:        strings.ReplaceAll(helpTimer, "\t", "    "),
		Experimental: true,
	})
}

const helpTimer = `  timer command [options]
  commands:
    list                            shows registered timers
    del <name>                      remove timer
	start <name>                    start the timer if it is not in RUNNING state
	stop <name>                     stop the timer if it is in RUNNING state
	add [options] <name> <spec> <tql-path>
									add a timer that executes the specified tql script in the given period.
        options:
            --autostart             enable auto start
        args:
            name                    name of the timer
			spec                    timer spec
				                    ex) '@every 60s' '@daily' '@hourly' '0 30 * * * *'
			tql-path                the relative path of tql script
		ex)
			timer add-timer --auto-start my_sched '@every 10s' /hello.tql
`

type TimerCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"del"`
	Start struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"start"`
	Stop struct {
		Name string `arg:"" name:"name"`
	} `cmd:"" name:"stop"`
	Add  TimerAddCmd `cmd:"" name:"add"`
	Help bool        `kong:"-"`
}

type TimerAddCmd struct {
	Name      string `arg:"" name:"name" help:"timer name"`
	Spec      string `arg:"" name:"spec" help:"timer spec"`
	TqlPath   string `arg:"" name:"tql-path" help:"relative path to tql script"`
	AutoStart bool   `name:"autostart"`
}

func pcTimer() action.PrefixCompleterInterface {
	return action.PcItem("timer",
		action.PcItem("add"),
		action.PcItem("list"),
		action.PcItem("del"),
		action.PcItem("start"),
		action.PcItem("stop"),
	)
}

func doTimer(ctx *action.ActionContext) {
	cmd := &TimerCmd{}
	parser, err := action.Kong(cmd, func() error {
		ctx.Println(strings.ReplaceAll(helpTimer, "\t", "    "))
		cmd.Help = true
		return nil
	})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	parseCtx, err := parser.Parse(util.SplitFields(ctx.Line, true))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	switch parseCtx.Command() {
	case "list":
		doTimerList(ctx)
	case "del <name>":
		doTimerDel(ctx, cmd.Del.Name)
	case "start <name>":
		doTimerStart(ctx, cmd.Start.Name)
	case "stop <name>":
		doTimerStop(ctx, cmd.Stop.Name)
	case "add <name> <spec> <tql-path>":
		doTimerAdd(ctx, &cmd.Add)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doTimerList(ctx *action.ActionContext) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListSchedule(ctx.Ctx, &schedrpc.ListScheduleRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}
	lst := []*schedrpc.Schedule{}

	for _, c := range rsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "TIMER" {
			continue
		}
		lst = append(lst, c)
	}
	box := ctx.NewBox([]string{
		"NAME", "SPEC", "TQL", "AUTOSTART", "STATE",
	})
	if len(lst) > 0 {
		for _, c := range lst {
			box.AppendRow(c.Name, c.Schedule, c.Task, c.AutoStart, c.State)
		}
	}
	box.Render()
}

func doTimerDel(ctx *action.ActionContext, name string) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelSchedule(ctx.Ctx, &schedrpc.DelScheduleRequest{
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

func doTimerStart(ctx *action.ActionContext, name string) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.StartSchedule(ctx.Ctx, &schedrpc.StartScheduleRequest{
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
	ctx.Println("start", name)
}

func doTimerStop(ctx *action.ActionContext, name string) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.StopSchedule(ctx.Ctx, &schedrpc.StopScheduleRequest{
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
	ctx.Println("stop", name)
}

func doTimerAdd(ctx *action.ActionContext, cmd *TimerAddCmd) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddSchedule(ctx.Ctx, &schedrpc.AddScheduleRequest{
		Name:      strings.ToLower(cmd.Name),
		Type:      "timer",
		AutoStart: cmd.AutoStart,
		Schedule:  cmd.Spec,
		Task:      cmd.TqlPath,
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
