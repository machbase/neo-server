package cmd

import (
	"strings"

	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "explain",
		PcFunc: pcExplain,
		Action: doExplain,
		Desc:   "Display execution plan of query",
		Usage:  helpExplain,
	})
}

const helpExplain string = `  explain <query>
  arguments:
    query       query statement to display the execution plan
  options:
    --full      full explain
`

type ExplainCmd struct {
	Help  bool     `kong:"-"`
	Full  bool     `name:"full"`
	Query []string `arg:"" name:"query" passthrough:""`
}

func pcExplain() readline.PrefixCompleterInterface {
	return readline.PcItem("explain")
}

func doExplain(ctx *client.ActionContext) {
	cmd := &ExplainCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpExplain); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	sqlText := util.StripQuote(strings.Join(cmd.Query, " "))
	plan, err := ctx.DB.Explain(sqlText, cmd.Full)
	if err != nil {
		ctx.Println(err.Error())
		return
	}
	ctx.Println(plan)
}
