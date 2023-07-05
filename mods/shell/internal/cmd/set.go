package cmd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
)

func init() {
	lines := []string{}
	if pref, err := client.LoadPref(); err == nil {
		for _, itm := range pref.Items() {
			lines = append(lines, fmt.Sprintf("  set %-10s  %s", itm.Name, itm.Description()))
		}
	}

	client.RegisterCmd(&client.Cmd{
		Name:         "set",
		PcFunc:       pcSet,
		Action:       doSet,
		Desc:         "Settings of the shell",
		Usage:        fmt.Sprintf("  set <key> <value>\n%s\n", strings.Join(lines, "\n")),
		ClientAction: true,
	})
}

func pcSet() readline.PrefixCompleterInterface {
	top := readline.PcItem("set")
	if pref, err := client.LoadPref(); err == nil {
		for _, itm := range pref.Items() {
			pc := readline.PcItem(itm.Name)
			for _, en := range itm.Enum {
				ec := readline.PcItem(en)
				pc.Children = append(pc.Children, ec)
			}
			top.Children = append(top.Children, pc)
		}
	}
	return top
}

func doSet(ctx *client.ActionContext) {
	args := util.SplitFields(ctx.Line, true)
	pref := ctx.Pref()
	if len(args) == 0 {
		box := ctx.NewBox([]string{"NAME", "VALUE", "DESCRIPTION"})
		itms := pref.Items()
		for _, itm := range itms {
			box.AppendRow(itm.Name, itm.Value(), itm.Description())
		}
		box.Render()
		return
	}

	if len(args) == 2 {
		itm := pref.Item(strings.ToLower(args[0]))
		if itm == nil {
			ctx.Println("unknown set key '%s'", args[0])
		} else {
			value := util.StripQuote(args[1])
			if err := itm.SetValue(value); err != nil {
				ctx.Println("ERR", err.Error())
			} else {
				pref.Save()
			}
		}
	}
}
