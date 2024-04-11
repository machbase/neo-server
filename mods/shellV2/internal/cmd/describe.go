package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
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

	_desc, err := do.Describe(ctx.Ctx, api.ConnRpc(ctx.Conn), tableName, cmd.ShowAll)
	if err != nil {
		ctx.Println("unable to describe", cmd.Table, "; ERR", err.Error())
		return
	}
	desc := _desc.(*do.TableDescription)

	if len(desc.Indexes) > 0 {
		ctx.Println("[ COLUMN ]")
	}
	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "LENGTH", "DESC"})
	for _, col := range desc.Columns {
		nrow++
		colType := api.ColumnTypeStringNative(col.Type)
		box.AppendRow(nrow, col.Name, colType, col.Size(), api.ColumnFlagString(col.Flag))
	}
	box.Render()

	if len(desc.Indexes) > 0 {
		ctx.Println("[ INDEX ]")
		nrow = 0
		box = ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "COLUMN"})
		for _, idx := range desc.Indexes {
			nrow++
			idxType := api.IndexTypeString(idx.Type)
			box.AppendRow(nrow, idx.Name, idxType, strings.Join(idx.Cols, ", "))
		}
		box.Render()
	}
}
