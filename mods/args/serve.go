package args

import (
	"fmt"
	"os"

	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/server"
	"github.com/machbase/neo-server/v8/mods/shell"
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
		return doServe(cli.Serve.Preset, false, false)
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(server.DefaultFallbackConfig))
	case "version":
		fmt.Println(mods.GenBanner())
	case "help":
		doHelp(cli.Help.Command, cli.Help.SubCommand)
	case "serve":
		return doServe(cli.Serve.Preset, false, false)
	case "serve-headless":
		return doServe(cli.Serve.Preset, true, false)
	case "restore":
		return doRestore(&cli.Restore)
	case "shell":
		shell.Shell(&cli.Shell)
	case "service":
		doService(&cli.Service)
	case "help <command> <sub-command>":
	}
	return 0
}

func doServe(preset string, headless bool, doNotExit bool) int {
	server.PreferredPreset = preset
	server.Headless = headless

	booter.SetConfigFileSuffix(".conf")
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + machsvr.LinkInfo())
	booter.Startup()
	booter.WaitSignal()
	if doNotExit {
		// If process is running as an Windows Service, it should not call os.Exit()
		// before send the notification report to the service manager.
		// Otherwise Windows service control panel reports "Error 1067, the process terminated unexpectedly"
		booter.Shutdown()
	} else {
		// The other cases, when process is running in foreground or other OS escept Windows.
		// it can shutdown and exit.
		booter.ShutdownAndExit(0)
	}
	return 0
}

func doRestore(r *Restore) int {
	if err := server.Restore(r.DataDir, r.BackupDir); err != nil {
		fmt.Println("ERR", err.Error())
		return -1
	}
	return 0
}
