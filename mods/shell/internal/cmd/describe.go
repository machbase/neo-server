package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "desc",
		PcFunc: pcDescribe,
		Action: doDescribe,
		Desc:   "Describe table structure",
		Usage:  strings.ReplaceAll(helpDescribe, "\t", "    "),
	})
}

const helpDescribe = `  desc [options] <table>
  arguments:
    table        name of table to describe
  options:
    -a,--all     show all hidden columns`

type DescribeCmd struct {
	Table   string `arg:"" name:"table"`
	ShowAll bool   `name:"all" short:"a"`
	Help    bool   `kong:"-"`
}

func pcDescribe() action.PrefixCompleterInterface {
	return action.PcItem("desc")
}

func doDescribe(ctx *action.ActionContext) {
	cmd := &DescribeCmd{}

	parser, err := action.Kong(cmd, func() error { ctx.Println(helpDescribe); cmd.Help = true; return nil })
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

	tableName := strings.ToUpper(cmd.Table)
	toks := strings.Split(tableName, ".")
	if len(toks) == 1 {
		tableName = fmt.Sprintf("MACHBASEDB.%s.%s", ctx.Actor.Username(), toks[0])
	} else if len(toks) == 2 {
		tableName = fmt.Sprintf("MACHBASEDB.%s.%s", toks[0], toks[1])
	} else if len(toks) == 3 {
		tableName = fmt.Sprintf("%s.%s.%s", toks[0], toks[1], toks[2])
	}

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	desc, err := api.DescribeTable(ctx.Ctx, conn, tableName, cmd.ShowAll)
	if err != nil {
		ctx.Println("unable to describe", cmd.Table, "; ERR", err.Error())
		return
	}

	if len(desc.Indexes) > 0 {
		ctx.Println("[ COLUMN ]")
	}
	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "LENGTH", "FLAG", "INDEX"})
	for _, col := range desc.Columns {
		nrow++
		colType := col.Type.String()
		indexes := []string{}
		for _, idxDesc := range desc.Indexes {
			for _, colName := range idxDesc.Cols {
				if colName == col.Name {
					indexes = append(indexes, idxDesc.Name)
					break
				}
			}
		}
		box.AppendRow(nrow, col.Name, colType, col.Width(), col.Flag.String(), strings.Join(indexes, ","))
	}
	box.Render()
}
