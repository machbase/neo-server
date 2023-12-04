package args

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	shell "github.com/machbase/neo-server/mods/shellV2"
)

type NeoCommand struct {
	Command string
	Serve   struct {
		Preset string
	}
	Shell     shell.ShellCmd
	GenConfig struct{}
	Version   struct{}
	Help      struct {
		Command    string
		SubCommand string
	}

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
	case "serve":
		return parseServe(cli)
	case "shell":
		return parseShell(cli)
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

func parseShell(cli *NeoCommand) (*NeoCommand, error) {
	for i := 0; i < len(cli.args); i++ {
		s := cli.args[i]
		if strings.HasPrefix(s, "--server=") {
			cli.Shell.ServerAddr = s[9:]
		} else if strings.HasPrefix(s, "-s=") {
			cli.Shell.ServerAddr = s[3:]
		} else if (s == "--server" || s == "-s") && len(cli.args) >= i+1 && !strings.HasPrefix(cli.args[i+1], "-") {
			cli.Shell.ServerAddr = cli.args[i+1]
			i++
		} else if strings.HasPrefix(s, "--user=") {
			cli.Shell.User = s[7:]
		} else if s == "--user" && len(cli.args) >= i+1 && !strings.HasPrefix(cli.args[i+1], "-") {
			cli.Shell.User = cli.args[i+1]
			i++
		} else if strings.HasPrefix(s, "--password=") {
			cli.Shell.Password = s[11:]
		} else if s == "--password" && len(cli.args) >= i+1 && !strings.HasPrefix(cli.args[i+1], "-") {
			cli.Shell.Password = cli.args[i+1]
			i++
		} else {
			// other flags and args should be passed to neoshell
			cli.Shell.Args = append(cli.Shell.Args, s)
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
