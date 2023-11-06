package action

import (
	"fmt"
	"sort"
	"strings"

	"github.com/machbase/neo-server/mods/util"
)

func init() {
	RegisterCmd(&Cmd{
		Name:         "help",
		PcFunc:       pcHelp,
		Action:       doHelp,
		Desc:         "Display this message, use 'help [command]'",
		ClientAction: true,
		Spec: &CmdSpec{
			Syntax:      "help [command|argument]",
			Description: "Display help message",
			Args: []*CmdArg{
				{Name: "timeformat", Description: "show specifiction"},
				{Name: "tz", Description: "show specifiction"},
				{Name: "keyboard", Description: "show keyboard shortcuts"},
			},
		},
	})
}

func pcHelp() PrefixCompleterInterface {
	return PcItem("help", PcItemDynamic(func(line string) []string {
		lst := make([]string, 0)
		for k := range globalCommands {
			lst = append(lst, k)
		}
		lst = append(lst, "timeformat")
		lst = append(lst, "tz")
		lst = append(lst, "keyboard")
		return lst
	}))
}

func doHelp(ctx *ActionContext) {
	fields := util.SplitFields(ctx.Line, true)
	if len(fields) > 0 {
		if cmd, ok := globalCommands[strings.ToLower(fields[0])]; ok {
			if cmd.Spec != nil {
				fmt.Println(cmd.Spec.String())
				return
			} else {
				ctx.Println(cmd.Desc)

				if len(cmd.Usage) > 0 {
					ctx.Println("Usage:")
					lines := strings.Split(cmd.Usage, "\n")
					for _, l := range lines {
						ctx.Println(strings.ReplaceAll(l, "\t", "    "))
					}
					ctx.Println()
				}
				return
			}
		}
		switch fields[0] {
		case "timeformat":
			ctx.Println("  timeformats:\n" + util.HelpTimeformats())
			return
		case "tz":
			ctx.Println("  timezones:\n" + util.HelpTimeZones())
			return
		case "keyboard":
			ctx.Println("  keybaord:\n" + util.HelpShortcuts())
			return
		}
	}
	ctx.Println("  commands:")
	width := 12
	keys := make([]string, 0, len(globalCommands))
	for k := range globalCommands {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "help" {
			return false
		} else if keys[j] == "help" {
			return true
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		cmd := globalCommands[k]
		if cmd.Experimental {
			// do not expose experimental command
			continue
		}
		aux := ""
		if cmd.Deprecated {
			aux = "// DEPRECATED"
		}
		if cmd.Spec != nil {
			ctx.Printfln("    %-*s %s %s", width, cmd.Name, cmd.Spec.Description, aux)
		} else {
			ctx.Printfln("    %-*s %s %s", width, cmd.Name, cmd.Desc, aux)
		}
	}
	ctx.Println(fmt.Sprintf("    %-*s %s", width, "keyboard", "Show shortcut keys"))
	ctx.Println(fmt.Sprintf("    %-*s %s", width, "clear", "Reset and clear screen"))
	ctx.Println(fmt.Sprintf("    %-*s %s", width, "exit", "Exit shell"))
}
