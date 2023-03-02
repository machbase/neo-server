package main

import (
	"errors"
	"fmt"
	"strings"

	shell "github.com/machbase/neo-shell"
)

type NeoCommand struct {
	Command   string
	Serve     struct{}
	Shell     shell.ShellCmd
	GenConfig struct{}
	Version   struct{}
	Help      struct {
		Command    string
		SubCommand string
	}

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
	return cli, nil
}

func parseShell(cli *NeoCommand) (*NeoCommand, error) {
	for i := 0; i < len(cli.args); i++ {
		s := cli.args[i]
		if len(s) < 2 || s[0] != '-' {
			cli.Shell.Args = append(cli.Shell.Args, s)
			continue
		}
		if strings.HasPrefix(s, "--server=") {
			cli.Shell.ServerAddr = s[9:]
		} else if strings.HasPrefix(s, "-s=") {
			cli.Shell.ServerAddr = s[3:]
		} else if (s == "--server" || s == "-s") && len(cli.args) >= i+1 && !strings.HasPrefix(cli.args[i+1], "-") {
			cli.Shell.ServerAddr = cli.args[i+1]
			i++
		}
	}
	return cli, nil
}
