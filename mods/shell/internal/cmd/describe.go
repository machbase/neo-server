package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
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
    -a,--all     show all hidden columns
`

type DescribeCmd struct {
	Table   string `arg:"" name:"table"`
	ShowAll bool   `name:"all" short:"a"`
	Help    bool   `kong:"-"`
}

func pcDescribe() readline.PrefixCompleterInterface {
	return readline.PcItem("desc")
}

func doDescribe(ctx *client.ActionContext) {
	cmd := &DescribeCmd{}

	parser, err := client.Kong(cmd, func() error { ctx.Println(helpDescribe); cmd.Help = true; return nil })
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

	if ctx.Client.Username() != "sys" {
		cmd.Table = fmt.Sprintf("%s.%s", ctx.Client.Username(), cmd.Table)
	}
	_desc, err := do.Describe(ctx.Ctx, ctx.Conn, cmd.Table, cmd.ShowAll)
	if err != nil {
		ctx.Println("unable to describe", cmd.Table, "; ERR", err.Error())
		return
	}
	desc := _desc.(*do.TableDescription)

	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "LENGTH"})
	for _, col := range desc.Columns {
		nrow++
		colType := spi.ColumnTypeString(col.Type)
		box.AppendRow(nrow, col.Name, colType, col.Length)
	}

	box.Render()
}
