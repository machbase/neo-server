package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestHasFileFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "not specified", args: []string{"neo", "serve"}, want: false},
		{name: "separate value", args: []string{"neo", "serve", "--file", "/data=/srv/data"}, want: true},
		{name: "equals value", args: []string{"neo", "serve", "--file=/data=/srv/data"}, want: true},
		{name: "after terminator", args: []string{"neo", "serve", "--", "--file=/data=/srv/data"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			options, err := parseServerBootOptions(tc.args)
			require.NoError(t, err)
			found := false
			for _, arg := range options.arguments {
				found = found || arg.name == "--file"
			}
			require.Equal(t, tc.want, found)
		})
	}
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

func TestParseServerBootOptions(t *testing.T) {
	options, err := parseServerBootOptions([]string{
		"neo", "serve",
		"--config-dir", "/etc/neo",
		"--config", "/etc/neo/server.conf",
		"--pname", "neo-test",
		"--pid", "/tmp/neo.pid",
		"--bootlog", "/tmp/neo.log",
		"--daemon",
		"--gen-config",
	})
	require.NoError(t, err)
	options.arguments = nil
	require.Equal(t, serverBootOptions{
		configDir:      "/etc/neo",
		configFile:     "/etc/neo/server.conf",
		pname:          "neo-test",
		pidFile:        "/tmp/neo.pid",
		bootlogFile:    "/tmp/neo.log",
		daemon:         true,
		generateConfig: true,
	}, options)

	options, err = parseServerBootOptions([]string{"neo", "serve", "--pname="})
	require.NoError(t, err)
	require.Equal(t, DefaultFallbackPname, options.pname)
}

func TestParseServerBootOptionsForms(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want serverBootOptions
	}{
		{
			name: "short and equals",
			args: []string{"-c", "'server.conf'", "-d", "--pid=neo.pid", "--bootlog=neo.log"},
			want: serverBootOptions{pname: DefaultFallbackPname, configFile: "server.conf", daemon: true, pidFile: "neo.pid", bootlogFile: "neo.log"},
		},
		{
			name: "long option takes precedence",
			args: []string{"-c", "short.conf", "--config=long.conf", "--daemon=false"},
			want: serverBootOptions{pname: DefaultFallbackPname, configFile: "long.conf"},
		},
		{
			name: "unknown config flags and terminator",
			args: []string{"--data", "data-dir", "--http-port=5654", "--pname", "named", "--", "--daemon"},
			want: serverBootOptions{pname: "named"},
		},
		{
			name: "headless and repeated options",
			args: []string{"--pid=old", "--pid", "new", "--gen-config"},
			want: serverBootOptions{pname: DefaultFallbackPname, pidFile: "new", generateConfig: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"neo", "serve-headless"}, tt.args...)
			got, err := parseServerBootOptions(args)
			require.NoError(t, err)
			got.arguments = nil
			require.Equal(t, tt.want, got)
		})
	}

	_, err := parseServerBootOptions([]string{"neo", "serve", "---bad"})
	require.ErrorContains(t, err, "bad flag syntax")
	_, err = parseServerBootOptions([]string{"neo", "serve", "--daemon=invalid"})
	require.ErrorContains(t, err, "invalid value")
}

func TestServerBootOptionsMissingValues(t *testing.T) {
	for _, name := range []string{"--config", "-c", "--config-dir", "--pid", "--bootlog", "--preset", "--file"} {
		for _, suffix := range [][]string{nil, {"--daemon"}} {
			args := append([]string{"neo", "serve", name}, suffix...)
			t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
				_, err := ParseCommand(args)
				require.ErrorContains(t, err, "requires a value")
			})
		}
		_, err := ParseCommand([]string{"neo", "serve", name + "="})
		require.ErrorContains(t, err, "requires a value")
	}
	require.NotZero(t, Main([]string{"neo", "serve", "--config"}))
	_, err := ParseCommand([]string{"neo", "serve", "--config", "--config=valid"})
	require.ErrorContains(t, err, "--config requires a value")
	_, err = ParseCommand([]string{"neo", "serve", "--daemon=invalid", "--daemon=true"})
	require.ErrorContains(t, err, "invalid value")
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

	t.Run("explicit data dir with equals", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data=/tmp/data", "/tmp/backup"}}

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

	t.Run("missing data value returns error", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data", "/tmp/backup", "--data"}}

		parsed, err := parseRestore(cli)

		require.Error(t, err)
		require.Nil(t, parsed)
		require.Contains(t, err.Error(), "requires a value")
	})

	t.Run("unknown flag returns error", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data", "/tmp/data", "--verbose", "/tmp/backup"}}

		parsed, err := parseRestore(cli)

		require.Error(t, err)
		require.Nil(t, parsed)
		require.Contains(t, err.Error(), "unknown flag")
	})

	t.Run("too many positional args returns error", func(t *testing.T) {
		cli := &NeoCommand{args: []string{"--data", "/tmp/data", "/tmp/backup1", "/tmp/backup2"}}

		parsed, err := parseRestore(cli)

		require.Error(t, err)
		require.Nil(t, parsed)
		require.Contains(t, err.Error(), "too many restore arguments")
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
	err := doHelp("serve", []string{"neo"})
	require.Nil(t, err)

	err = doHelp("shell", []string{"neo"})
	require.Nil(t, err)

	err = doHelp("timeformat", []string{"neo"})
	require.Nil(t, err)

	err = doHelp("tz", []string{"neo"})
	require.Nil(t, err)
}

func TestMainReturnsErrorOnUnknownCommand(t *testing.T) {
	rc := Main([]string{"neo", "unknown-command"})
	require.Equal(t, 1, rc)
}

func TestMainReturnsZeroOnSimpleCommands(t *testing.T) {
	tests := [][]string{
		{"neo", "gen-config"},
		{"neo", "version"},
		{"neo", "help"},
		{"neo", "help", "tz"},
	}

	for _, args := range tests {
		rc := Main(args)
		require.Equal(t, 0, rc, "args=%v", args)
	}
}

func TestMainReturnsErrorWithoutStoppingExistingServer(t *testing.T) {
	existingServer := lastStartedServer

	configPath := filepath.Join(t.TempDir(), "invalid.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("not valid hcl {{{"), 0o644))

	rc := Main([]string{"neo", "serve", "--config", configPath})
	require.Equal(t, 1, rc)
	if existingServer != nil {
		require.Same(t, existingServer, lastStartedServer)
	}
}

func TestForegroundStartupFailureCleansPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "server.pid")
	configPath := filepath.Join(dir, "invalid.conf")
	logPath := filepath.Join(dir, "boot.log")
	require.NoError(t, os.WriteFile(configPath, []byte("not valid hcl {{{"), 0o644))

	require.Equal(t, 1, Main([]string{"neo", "serve", "--config", configPath, "--pid", pidPath, "--bootlog", logPath}))
	_, err := os.Lstat(pidPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(logPath)
	require.NoError(t, err)

	require.Equal(t, 1, Main([]string{"neo", "serve", "--config", configPath, "--pid", pidPath, "--bootlog", filepath.Join(dir, "missing", "boot.log")}))
	_, err = os.Lstat(pidPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, os.WriteFile(pidPath, []byte("other process"), 0o644))
	require.Equal(t, 1, Main([]string{"neo", "serve", "--config", configPath, "--pid", pidPath}))
	content, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	require.Equal(t, "other process", string(content))
}
