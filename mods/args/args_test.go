package args_test

import (
	"testing"

	. "github.com/machbase/neo-server/v8/mods/args"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	args := []string{
		"test",
	}
	cli, err := ParseCommand(args)
	require.NotNil(t, err)
	require.Nil(t, cli)

	args = []string{
		"test", "",
	}
	cli, err = ParseCommand(args)
	require.NotNil(t, err)
	require.Nil(t, cli)

}

func TestParseServe(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "serve",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "serve", cli.Command)

	args = []string{
		"test", "serve", "--preset", "fog",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "serve", cli.Command)
	require.Equal(t, "fog", cli.Serve.Preset)

	args = []string{
		"test", "serve", "--preset=edge",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "serve", cli.Command)
	require.Equal(t, "edge", cli.Serve.Preset)
}

func TestParseShell(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "shell",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 0, len(cli.Shell.Args))
	require.Equal(t, "", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "--server=tcp://127.0.0.1:5655",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 0, len(cli.Shell.Args))
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "-s=tcp://127.0.0.1:5655",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 0, len(cli.Shell.Args))
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "--server", "tcp://127.0.0.1:5655",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 0, len(cli.Shell.Args))
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "-s", "tcp://127.0.0.1:5655",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 0, len(cli.Shell.Args))
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "-s", "tcp://127.0.0.1:5655", "select * from table",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 1, len(cli.Shell.Args))
	require.Equal(t, "select * from table", cli.Shell.Args[0])
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)

	args = []string{
		"test", "shell", "select * from table", "-s", "tcp://127.0.0.1:5655",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "shell", cli.Command)
	require.Equal(t, 1, len(cli.Shell.Args))
	require.Equal(t, "select * from table", cli.Shell.Args[0])
	require.Equal(t, "tcp://127.0.0.1:5655", cli.Shell.ServerAddr)
}

func TestParseHelp(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "help",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "help", "command_h1",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command_h1", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "help", "command_h1", "subcommand_h1",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command_h1", cli.Help.Command)
	require.Equal(t, "subcommand_h1", cli.Help.SubCommand)

	args = []string{
		"test", "--help",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "-h",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "help", "command",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "command_s", "--help",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command_s", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "command_s", "-h",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command_s", cli.Help.Command)
	require.Equal(t, "", cli.Help.SubCommand)

	args = []string{
		"test", "help", "command1", "subcommand1",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command1", cli.Help.Command)
	require.Equal(t, "subcommand1", cli.Help.SubCommand)

	args = []string{
		"test", "command2", "subcommand2", "-h",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "help", cli.Command)
	require.Equal(t, "command2", cli.Help.Command)
	require.Equal(t, "subcommand2", cli.Help.SubCommand)
}

func TestParseHelpTz(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "help", "tz",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
}

func TestParseVersion(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "version",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "version", cli.Command)
}

func TestParseGenConfig(t *testing.T) {
	var args []string
	var cli *NeoCommand
	var err error

	args = []string{
		"test", "gen-config",
	}
	cli, err = ParseCommand(args)
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, "gen-config", cli.Command)
}
