package args

import (
	"fmt"
	"os"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
	"github.com/machbase/neo-server/mods/shell"
)

func Main() int {
	cli, err := ParseCommand(os.Args)
	if err != nil {
		if cli != nil {
			doHelp(cli.Command, "")
		} else {
			doHelp("", "")
		}
		fmt.Println("ERR", err.Error())
		return 1
	}
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		return doServe()
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(server.DefaultFallbackConfig))
	case "version":
		fmt.Println(server.GenBanner())
	case "help":
		doHelp(cli.Help.Command, cli.Help.SubCommand)
	case "serve":
		doServe()
	case "shell":
		shell.Shell(&cli.Shell)
	case "help <command> <sub-command>":
	}
	return 0
}

func doServe() int {
	booter.SetConfiFileSuffix(".conf")
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + mach.LinkInfo())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
	return 0
}
