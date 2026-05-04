package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

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

func TestParseRestore(t *testing.T) {
	t.Run("explicit data dir", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data", "/tmp/data", "/tmp/backup"}}

		parsed, err := parseRestore(cli)

		require.NoError(t, err)
		require.Same(t, cli, parsed)
		require.Equal(t, "/tmp/data", parsed.Restore.DataDir)
		require.Equal(t, "/tmp/backup", parsed.Restore.BackupDir)
	})

	t.Run("default data dir uses executable directory", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"/tmp/backup"}}

		parsed, err := parseRestore(cli)

		require.NoError(t, err)
		require.Equal(t, "/tmp/backup", parsed.Restore.BackupDir)

		ep, err := os.Executable()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(filepath.Dir(ep), "machbase_home"), parsed.Restore.DataDir)
	})

	t.Run("missing backup dir returns error", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data", "/tmp/data"}}

		parsed, err := parseRestore(cli)

		require.Error(t, err)
		require.Nil(t, parsed)
	})
}

func TestParseService(t *testing.T) {
	cli := &NeoCommand{args: []string{"install", "neo", "--force"}}

	parsed, err := parseService(cli)

	require.NoError(t, err)
	require.Same(t, cli, parsed)
	require.Equal(t, []string{"install", "neo", "--force"}, parsed.Service.Args)
}

func TestParseCommandService(t *testing.T) {
	cli, err := ParseCommand([]string{"test", "service", "install"})

	if runtime.GOOS == "windows" {
		require.NoError(t, err)
		require.NotNil(t, cli)
		require.Equal(t, "service", cli.Command)
		require.Equal(t, []string{"install"}, cli.Service.Args)
		return
	}

	require.Error(t, err)
	require.Nil(t, cli)
	require.Contains(t, err.Error(), "only available on Windows")
}

func TestDoHelp(t *testing.T) {
	err := doHelp("serve")
	require.Nil(t, err)

	err = doHelp("shell")
	require.Nil(t, err)

	err = doHelp("timeformat")
	require.Nil(t, err)

	err = doHelp("tz")
	require.Nil(t, err)
}
