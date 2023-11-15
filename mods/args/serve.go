package args

import (
	"fmt"
	"os"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
	shell "github.com/machbase/neo-server/mods/shellV2"
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
		return doServe(cli.Serve.Preset)
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(server.DefaultFallbackConfig))
	case "version":
		fmt.Println(server.GenBanner())
	case "help":
		doHelp(cli.Help.Command, cli.Help.SubCommand)
	case "serve":
		doServe(cli.Serve.Preset)
	case "shell":
		shell.Shell(&cli.Shell)
	case "service":
		doService(&cli.Service)
	case "help <command> <sub-command>":
	}
	return 0
}

func doServe(preset string) int {
	server.PreferredPreset = preset

	booter.SetConfiFileSuffix(".conf")
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + mach.LinkInfo())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
	return 0
}
