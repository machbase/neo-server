package cmd

import (
	"fmt"
	"sort"
	"strings"

	schedrpc "github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "subscriber",
		PcFunc: pcSubscriber,
		Action: doSubscriber,
		Desc:   "Manage subscribers",
		Usage:  strings.ReplaceAll(helpSubscriber, "\t", "    "),
	})
}

const helpSubscriber = `  subscriber command [options]
  commands:
    list                            shows registered subscriber
    del <name>                      remove subscriber
	start <name>                    start the subscriber if it is not in RUNNING state
	stop <name>                     stop the subscriber if it is in RUNNING state
	add [options] <name> <bridge> <topic> <destination>
							        add a subscriber to the topic via pre-defined bridge,
									then executes the given tql script whenever it receives messages.
		options:
			--autostart             enable auto start
			--qos                   (mqtt bridge only) specify QoS to subscribe (default: 0)
			--queue                 (nats bridge only) specify Queue Group
		args:
			name                    name of the subscriber
			bridge                  name of the bridge
			topic                   topic to subscribe
			destination             the path of tql script or writing path descriptor
		ex)
			subscriber add --auto-start --qos=1 my_lsnr my_mqtt outer/events /my_event.tql
			subscriber add my_append nats_bridge stream.in db/append/EXAMPLE:json
			subscriber add my_writer nats_bridge topic.in  db/write/EXAMPLE:csv:gzip
`

type SubscriberCmd struct {
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
	Add  SubscriberAddCmd `cmd:"" name:"add"`
	Help bool             `kong:"-"`
}

type SubscriberAddCmd struct {
	Name      string `arg:"" name:"name" help:"schedule name"`
	Bridge    string `arg:"" name:"bridge" help:"name of bridge"`
	Topic     string `arg:"" name:"topic" help:"topic to subscribe"`
	TqlPath   string `arg:"" name:"destination" help:"the path of tql script or writing path descriptor"`
	AutoStart bool   `name:"autostart"`
	QoS       int    `name:"qos" help:"(mqtt bridge only) QoS to subscribe"`
	Queue     string `name:"queue" help:"(nats bridge only) Queue Group"`
}

func pcSubscriber() action.PrefixCompleterInterface {
	return action.PcItem("subscriber",
		action.PcItem("add"),
		action.PcItem("list"),
		action.PcItem("del"),
		action.PcItem("start"),
		action.PcItem("stop"),
	)
}

func doSubscriber(ctx *action.ActionContext) {
	cmd := &SubscriberCmd{}
	parser, err := action.Kong(cmd, func() error {
		ctx.Println(strings.ReplaceAll(helpSubscriber, "\t", "    "))
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
		doSubscriberList(ctx)
	case "del <name>":
		doSubscriberDel(ctx, cmd.Del.Name)
	case "start <name>":
		doSubscriberStart(ctx, cmd.Start.Name)
	case "stop <name>":
		doSubscriberStop(ctx, cmd.Stop.Name)
	case "add <name> <bridge> <topic> <destination>":
		doSubscriberAdd(ctx, &cmd.Add)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doSubscriberList(ctx *action.ActionContext) {
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
		if typ != "SUBSCRIBER" {
			continue
		}
		lst = append(lst, c)
	}
	box := ctx.NewBox([]string{
		"NAME", "BRIDGE", "TOPIC", "DESTINATION", "AUTOSTART", "STATE",
	})
	if len(lst) > 0 {
		sort.Slice(lst, func(i, j int) bool { return lst[i].Name < lst[j].Name })
		for _, c := range lst {
			box.AppendRow(c.Name, c.Bridge, c.Topic, c.Task, c.AutoStart, c.State)
		}
	}
	box.Render()
}

func doSubscriberDel(ctx *action.ActionContext, name string) {
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

func doSubscriberStart(ctx *action.ActionContext, name string) {
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

func doSubscriberStop(ctx *action.ActionContext, name string) {
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

func doSubscriberAdd(ctx *action.ActionContext, cmd *SubscriberAddCmd) {
	mgmtCli, err := ctx.Actor.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddSchedule(ctx.Ctx, &schedrpc.AddScheduleRequest{
		Name:      strings.ToLower(cmd.Name),
		Type:      "subscriber",
		AutoStart: cmd.AutoStart,
		Bridge:    cmd.Bridge,
		Topic:     cmd.Topic,
		QoS:       int32(cmd.QoS),
		Queue:     cmd.Queue,
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
