package args

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	shell "github.com/machbase/neo-server/mods/shellV2"
	"github.com/machbase/neo-server/mods/util"
)

func doHelp(command string, subcommand string) error {
	showShellHelp := true
	showServeHelp := true

	switch command {
	case "serve":
		fmt.Println(os.Args[0] + " serve [args...]")
		showShellHelp = false
	case "shell":
		fmt.Println(os.Args[0] + " shell [flags] <sub-command> [args...]")
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
  serve <falgs>               start machbase-neo server process
  shell <flags> <sub-command> run neoshell client
  gen-config                  show config template
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
