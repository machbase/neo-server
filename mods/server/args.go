package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/util"
)

func Main(args []string) int {
	cli, err := ParseCommand(args)
	if err != nil {
		if cli != nil {
			doHelp(cli.Command)
		} else {
			doHelp("")
		}
		fmt.Println("ERR", err.Error())
		return 1
	}
	if len(args) > 1 && args[1] == "serve" {
		return doServe(cli.Serve.Preset, false, false)
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(DefaultFallbackConfig))
	case "version":
		fmt.Println(mods.GenBanner())
	case "help":
		doHelp(cli.Help.Command)
	case "serve":
		return doServe(cli.Serve.Preset, false, false)
	case "serve-headless":
		return doServe(cli.Serve.Preset, true, false)
	case "restore":
		return doRestore(&cli.Restore)
	case "service":
		doService(&cli.Service)
	case "help <command> <sub-command>":
	}
	return 0
}

func doServe(preset string, headless bool, doNotExit bool) int {
	PreferredPreset = preset
	Headless = headless

	booter.SetConfigFileSuffix(".conf")
	booter.SetFallbackConfig(DefaultFallbackConfig)
	booter.SetFallbackPname(DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + machsvr.LinkInfo())
	booter.Startup()
	booter.WaitSignal()
	if doNotExit {
		// If process is running as an Windows Service, it should not call os.Exit()
		// before send the notification report to the service manager.
		// Otherwise Windows service control panel reports "Error 1067, the process terminated unexpectedly"
		booter.Shutdown()
	} else {
		// The other cases, when process is running in foreground or other OS except Windows.
		// it can shutdown and exit.
		booter.ShutdownAndExit(0)
	}
	return 0
}

func doRestore(r *RestoreCmd) int {
	if err := Restore(r.DataDir, r.BackupDir); err != nil {
		fmt.Println("ERR", err.Error())
		return -1
	}
	return 0
}

type NeoCommand struct {
	Command string
	Serve   struct {
		Preset string
	}
	GenConfig struct{}
	Version   struct{}
	Help      struct {
		Command    string
		SubCommand string
	}
	Restore RestoreCmd
	Service Service

	args []string
}

func ParseCommand(args []string) (*NeoCommand, error) {
	if len(args) <= 1 {
		return nil, errors.New("missing required command")
	}

	cli := &NeoCommand{}

	hasHelpFlag := false
	idxHelpFlag := -1
	if args[0] != "help" {
		for i, s := range args[1:] {
			if s == "--help" || s == "-h" {
				hasHelpFlag = true
				idxHelpFlag = i + 1
				break
			}
		}
	}
	if hasHelpFlag {
		cli.Command = "help"
		if idxHelpFlag == 2 {
			cli.Help.Command = args[1]
		} else if idxHelpFlag >= 3 {
			cli.Help.Command = args[1]
			cli.Help.SubCommand = args[2]
		}
		return cli, nil
	} else {
		cli.Command = args[1]
		cli.args = args[2:]
	}

	switch cli.Command {
	case "serve", "serve-headless":
		return parseServe(cli)
	case "restore":
		return parseRestore(cli)
	case "help":
		return parseHelp(cli)
	case "gen-config":
		return parseGenConfig(cli)
	case "version":
		return parseVersion(cli)
	case "service":
		if runtime.GOOS == "windows" {
			return parseService(cli)
		} else {
			return nil, fmt.Errorf("command 'service' is only available on Windows")
		}
	default:
		return nil, fmt.Errorf("unknown command '%s'", cli.Command)
	}
}

func parseVersion(cli *NeoCommand) (*NeoCommand, error) {
	return cli, nil
}

func parseGenConfig(cli *NeoCommand) (*NeoCommand, error) {
	return cli, nil
}

func parseHelp(cli *NeoCommand) (*NeoCommand, error) {
	if len(cli.args) >= 1 {
		cli.Help.Command = cli.args[0]
	}
	if len(cli.args) >= 2 {
		cli.Help.SubCommand = cli.args[1]
	}
	return cli, nil
}

func parseServe(cli *NeoCommand) (*NeoCommand, error) {
	for i := 0; i < len(cli.args); i++ {
		s := cli.args[i]
		if strings.HasPrefix(s, "--preset=") {
			cli.Serve.Preset = s[9:]
		} else if s == "--preset" && len(cli.args) >= i+1 && !strings.HasPrefix(cli.args[i+1], "-") {
			cli.Serve.Preset = cli.args[i+1]
			i++
		}
	}
	return cli, nil
}

type RestoreCmd struct {
	Help      bool   `kong:"-"`
	DataDir   string `name:"data"`
	BackupDir string `arg:"" name:"path"`
}

func parseRestore(cli *NeoCommand) (*NeoCommand, error) {
	parser, err := kong.New(&cli.Restore, kong.HelpOptions{Compact: true})
	if err != nil {
		return nil, err
	}
	_, err = parser.Parse(cli.args)
	if err != nil {
		return nil, err
	}
	if cli.Restore.DataDir == "" {
		if ep, err := os.Executable(); err != nil {
			return nil, err
		} else {
			cli.Restore.DataDir = filepath.Join(filepath.Dir(ep), "machbase_home")
		}
	}
	return cli, nil
}

type Service struct {
	Args []string `arg:"" optional:"" name:"ARGS" passthrough:""`
}

func parseService(cli *NeoCommand) (*NeoCommand, error) {
	cli.Service.Args = cli.args
	return cli, nil
}

func doHelp(command string) error {
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
		fmt.Println("  timeformat:")
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
		fmt.Println(helpShellText)
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

const helpShellText = `Usage of shell:
  -C string
        command to execute
  -S string
        configured file to start from
  -e value
        environment variable (format: name=value)
  -password string
        password (default: manager)
  -server string
        machbase-neo host
  -user string
        user name (default: sys)
  -v value
        volume to mount (format: /mountpoint=source)`
