package cmd

import (
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "explain",
		PcFunc: pcExplain,
		Action: doExplain,
		Desc:   "Display execution plan of query",
		Usage:  strings.ReplaceAll(helpExplain, "\t", "    "),
	})
}

const helpExplain string = `  explain <query>
  arguments:
    query       query statement to display the execution plan
  options:
    --full      full explain`

type ExplainCmd struct {
	Help  bool     `kong:"-"`
	Full  bool     `name:"full"`
	Query []string `arg:"" name:"query" passthrough:""`
}

func pcExplain() action.PrefixCompleterInterface {
	return action.PcItem("explain")
}

func doExplain(ctx *action.ActionContext) {
	cmd := &ExplainCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpExplain); cmd.Help = true; return nil })
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

	tick := time.Now()
	sqlText := util.StripQuote(strings.Join(cmd.Query, " "))
	if explainer, ok := ctx.Conn.(spi.Explainer); ok {
		plan, err := explainer.Explain(ctx.Ctx, sqlText, cmd.Full)
		if err != nil {
			ctx.Println(err.Error())
			return
		}
		elapsed := time.Since(tick).String()
		ctx.Println(plan)
		if cmd.Full {
			ctx.Printfln("Elapsed time %s", elapsed)
		}
	} else {
		ctx.Println("Explain is unavailable")
	}
}
