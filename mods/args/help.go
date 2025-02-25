package args

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/machbase/neo-server/v8/mods/shell"
	"github.com/machbase/neo-server/v8/mods/util"
)

func doHelp(command string, subcommand string) error {
	showShellHelp := true
	showServeHelp := true

	switch command {
	case "serve":
		fmt.Println(os.Args[0] + " serve [args...]")
		showShellHelp = false
	case "serve-headless":
		fmt.Println(os.Args[0] + " serve-headless [args...]")
		showShellHelp = false
	case "shell":
		fmt.Println(os.Args[0] + " shell [flags] <sub-command> [args...]")
		showServeHelp = false
	case "restore":
		fmt.Println(os.Args[0] + " restore --data <machbase_home_dir> <backup_dir>")
		showShellHelp = false
		showServeHelp = false
	case "timeformat":
		fmt.Println("  timeformats:")
		fmt.Printf("%s\n", util.HelpTimeformats())
		return nil
	case "tz":
		fmt.Println("  timezones:")
		fmt.Printf("%s\n", util.HelpTimeZones())
		return nil
	default:
		fmt.Println(filepath.Base(os.Args[0]) + helpRootText)
	}

	if showServeHelp {
		if runtime.GOOS == "windows" {
			fmt.Println(helpServeTextWindows)
		} else {
			fmt.Println(helpServeText)
		}
	}

	if showShellHelp {
		shell.PrintHelp(subcommand, helpShellText)
	}
	return nil
}

const helpRootText = ` <command> [args...]

Commands:
  serve <flags>               start machbase-neo server process
  serve-headless <flags>      start machbase-neo server process in headless mode
  shell <flags> <sub-command> run neoshell client
  gen-config                  show config template
  restore                     restore database from backup
  version                     show version`

const helpServeText = `
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name
      --pid=<path>        pid file path
      --bootlog=<path>    boot log file path
  -d, --daemon            run process in background, daemonize`

const helpServeTextWindows = `
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name`

const helpShellText = `flags:
  -s, --server=<addr>     server address (default %s)
      --user=<user>       username (default 'sys')
      --password=<pass>   password (default 'manager')
`
