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
		Name:         "subscriber",
		PcFunc:       pcSubscriber,
		Action:       doSubscriber,
		Desc:         "Manage subscribers",
		Usage:        helpSubscriber,
		Experimental: true,
	})
}

const helpSubscriber = `  subscriber command [options]
  commands:
    list                            shows registered subscriber
    del <name>                      remove subscriber
	start <name>                    start the subscriber if it is not in RUNNING state
	stop <name>                     stop the subscriber if it is in RUNNING state
	add [options] <name> <bridge> <topic> <tql-path>
							        add a subscriber to the topic via pre-defined bridge,
									then executes the given tql script whenever it receives messages.
		options:
			--autostart             enable auto start
			--qos                   (mqtt bridge only) specify QoS to subscribe (default: 0)
		args:
			name                    name of the subscriber
			bridge                  name of the bridge
			topic                   topic to subscribe (listening to)
			tql-path                the relative path of tql script
		ex)
			subscriber add --auto-start --qos=1 my_lsnr my_mqtt outer/events /my_event.tql
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
	TqlPath   string `arg:"" name:"tql-path" help:"relative path to tql script"`
	AutoStart bool   `name:"autostart"`
	QoS       int    `name:"qos" help:"(mqtt bridge only) QoS to subscribe"`
}

func pcSubscriber() readline.PrefixCompleterInterface {
	return readline.PcItem("subscriber",
		readline.PcItem("add"),
		readline.PcItem("list"),
		readline.PcItem("del"),
		readline.PcItem("start"),
		readline.PcItem("stop"),
	)
}

func doSubscriber(ctx *client.ActionContext) {
	cmd := &SubscriberCmd{}
	parser, err := client.Kong(cmd, func() error {
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
	case "add <name> <bridge> <topic> <tql-path>":
		doSubscriberAdd(ctx, &cmd.Add)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doSubscriberList(ctx *client.ActionContext) {
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
		if typ != "SUBSCRIBER" {
			continue
		}
		lst = append(lst, c)
	}
	box := ctx.NewBox([]string{
		"NAME", "BRIDGE", "TOPIC", "TQL", "AUTOSTART", "STATE",
	})
	if len(lst) > 0 {
		for _, c := range lst {
			box.AppendRow(c.Name, c.Bridge, c.Topic, c.Task, c.AutoStart, c.State)
		}
	}
	box.Render()
}

func doSubscriberDel(ctx *client.ActionContext, name string) {
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

func doSubscriberStart(ctx *client.ActionContext, name string) {
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

func doSubscriberStop(ctx *client.ActionContext, name string) {
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

func doSubscriberAdd(ctx *client.ActionContext, cmd *SubscriberAddCmd) {
	mgmtCli, err := ctx.Client.ScheduleManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddSchedule(ctx, &schedrpc.AddScheduleRequest{
		Name:      strings.ToLower(cmd.Name),
		Type:      "subscriber",
		AutoStart: cmd.AutoStart,
		Bridge:    cmd.Bridge,
		Topic:     cmd.Topic,
		QoS:       int32(cmd.QoS),
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
