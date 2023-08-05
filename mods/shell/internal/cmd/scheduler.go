package cmd

import (
	"fmt"
	"strings"

	schedrpc "github.com/machbase/neo-grpc/schedule"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:         "scheduler",
		PcFunc:       pcScheduler,
		Action:       doScheduler,
		Desc:         "Manage schedulers",
		Usage:        helpScheduler,
		Experimental: true,
	})
}

// Timer spec
//
//	0 30 * * * *           Every hour on the half hour
//	@every 1h30m           Every hour thirty
//	@daily                 Every day
//
// CRON expression
//
//	Field name   | Mandatory? | Allowed values  | Allowed special characters
//	----------   | ---------- | --------------  | --------------------------
//	Seconds      | Yes        | 0-59            | * / , -
//	Minutes      | Yes        | 0-59            | * / , -
//	Hours        | Yes        | 0-23            | * / , -
//	Day of month | Yes        | 1-31            | * / , - ?
//	Month        | Yes        | 1-12 or JAN-DEC | * / , -
//	Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?
//
//	Asterisk ( * )
//	  The asterisk indicates that the cron expression will match for all values of the field;
//	  e.g., using an asterisk in the 5th field (month) would indicate every month.
//	Slash ( / )
//	  Slashes are used to describe increments of ranges. For example 3-59/15 in the 1st field
//	  (minutes) would indicate the 3rd minute of the hour and every 15 minutes thereafter.
//	  The form "*/..." is equivalent to the form "first-last/...", that is, an increment over
//	  the largest possible range of the field. The form "N/..." is accepted as meaning "N-MAX/...",
//	  that is, starting at N, use the increment until the end of that specific range. It does not
//	  wrap around.
//	Comma ( , )
//	  Commas are used to separate items of a list. For example, using "MON,WED,FRI" in the 5th
//	  field (day of week) would mean Mondays, Wednesdays and Fridays.
//	Hyphen ( - )
//	  Hyphens are used to define ranges. For example, 9-17 would indicate every hour between
//	  9am and 5pm inclusive.
//	Question mark ( ? )
//	  Question mark may be used instead of '*' for leaving either day-of-month or day-of-week
//	  blank.
//
// Predefined schedules
//
//	Entry                  | Description                                | Equivalent To
//	-----                  | -----------                                | -------------
//	@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 0 1 1 *
//	@monthly               | Run once a month, midnight, first of month | 0 0 0 1 * *
//	@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 0 * * 0
//	@daily (or @midnight)  | Run once a day, midnight                   | 0 0 0 * * *
//	@hourly                | Run once an hour, beginning of hour        | 0 0 * * * *
//
// Intervals
//
//	@every <duration>      where "duration" is a string accepted by time.ParseDuration
//	                       (http://golang.org/pkg/time/#ParseDuration).

const helpScheduler = `  scheduler command [options]
  commands:
    list                            shows registered schedulers
    del <name>                      remove scheduler
	start <name>                    start the scheduler if it is not in RUNNING state
	stop <name>                     stop the scheduler if it is in RUNNING state
	add [options] <name> <spec> <tql-path>
									add a scheduler that executes the specified tql script in the given period.
        options:
            --autostart             enable auto start
        args:
            name                    name of the scheduler
			spec                    timing spec
				                    ex) '@every 60s' '@daily' '@hourly' '0 30 * * * *'
			tql-path                the relative path of tql script
		ex)
			scheduler add-timer --auto-start my_sched '@every 10s' /hello.tql
`

type SchedulerCmd struct {
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
	Add  SchedulerAddCmd `cmd:"" name:"add"`
	Help bool            `kong:"-"`
}

type SchedulerAddCmd struct {
	Name      string `arg:"" name:"name" help:"scheduler name"`
	Spec      string `arg:"" name:"spec" help:"timing spec"`
	TqlPath   string `arg:"" name:"tql-path" help:"relative path to tql script"`
	AutoStart bool   `name:"autostart"`
}

func pcScheduler() readline.PrefixCompleterInterface {
	return readline.PcItem("scheduler",
		readline.PcItem("list"),
		readline.PcItem("add"),
		readline.PcItem("del"),
		readline.PcItem("start"),
		readline.PcItem("stop"),
	)
}

func doScheduler(ctx *client.ActionContext) {
	cmd := &SchedulerCmd{}
	parser, err := client.Kong(cmd, func() error {
		ctx.Println(strings.ReplaceAll(helpScheduler, "\t", "    "))
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
		doSchedulerList(ctx)
	case "del <name>":
		doSchedulerDel(ctx, cmd.Del.Name)
	case "start <name>":
		doSchedulerStart(ctx, cmd.Start.Name)
	case "stop <name>":
		doSchedulerStop(ctx, cmd.Stop.Name)
	case "add <name> <spec> <tql-path>":
		doSchedulerAdd(ctx, &cmd.Add)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doSchedulerList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListSchedule(ctx, &schedrpc.ListScheduleRequest{})
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

func doSchedulerDel(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelSchedule(ctx, &schedrpc.DelScheduleRequest{
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

func doSchedulerStart(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.StartSchedule(ctx, &schedrpc.StartScheduleRequest{
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

func doSchedulerStop(ctx *client.ActionContext, name string) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.StopSchedule(ctx, &schedrpc.StopScheduleRequest{
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

func doSchedulerAdd(ctx *client.ActionContext, cmd *SchedulerAddCmd) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddSchedule(ctx, &schedrpc.AddScheduleRequest{
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
