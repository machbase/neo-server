package server

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

func newShellTestServer(t *testing.T) *Server {
	t.Helper()
	models := model.NewService(model.WithConfigDirPath(t.TempDir()))
	require.NoError(t, models.Start())

	return &Server{
		log:    logging.GetLog("svrshells-test"),
		models: models,
		Config: Config{Http: HttpConfig{Listeners: []string{}}},
	}
}

func withReservedShellCommands(t *testing.T, svr *Server, shellCmd string, jshCmd string) {
	t.Helper()
	shellDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_SHELL)
	require.NoError(t, err)
	jshDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_JSH)
	require.NoError(t, err)
	prevShell := shellDef.Command
	prevJsh := jshDef.Command
	svr.models.ShellProvider().SetDefaultShellCommand(shellCmd)
	svr.models.ShellProvider().SetDefaultJshCommand(jshCmd)
	t.Cleanup(func() {
		svr.models.ShellProvider().SetDefaultShellCommand(prevShell)
		svr.models.ShellProvider().SetDefaultJshCommand(prevJsh)
	})
}

func TestShellAddressCandidates(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		loopback  bool
		anyIface  bool
	}{
		{name: "localhost", candidate: "localhost:5655", loopback: true, anyIface: false},
		{name: "ipv4_loopback", candidate: "127.0.0.1:5655", loopback: true, anyIface: false},
		{name: "ipv6_loopback", candidate: "[::1]:5655", loopback: true, anyIface: false},
		{name: "ipv4_any", candidate: "0.0.0.0:5655", loopback: false, anyIface: true},
		{name: "ipv6_any", candidate: "[::]:5655", loopback: false, anyIface: true},
		{name: "remote", candidate: "10.0.0.10:5655", loopback: false, anyIface: false},
		{name: "invalid", candidate: "not-an-addr", loopback: false, anyIface: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.loopback, isLoopbackCandidate(tt.candidate))
			require.Equal(t, tt.anyIface, isAnyIfaceCandidate(tt.candidate))
		})
	}
}

func TestInitShellProvider(t *testing.T) {
	t.Run("prefers_loopback_listener", func(t *testing.T) {
		svr := newShellTestServer(t)
		svr.Http.Listeners = []string{"http://ignored", "tcp://10.0.0.10:5655", "tcp://127.0.0.1:7777"}
		withReservedShellCommands(t, svr, "", "")

		err := svr.initShellProvider()
		require.NoError(t, err)

		shellDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_SHELL)
		require.NoError(t, err)
		require.Contains(t, shellDef.Command, "shell")
		require.Contains(t, shellDef.Command, "-server 127.0.0.1:7777")

		jshDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_JSH)
		require.NoError(t, err)
		require.Contains(t, jshDef.Command, "jsh")
		require.NotContains(t, jshDef.Command, " -server ")
	})

	t.Run("rewrites_any_interface_to_loopback", func(t *testing.T) {
		svr := newShellTestServer(t)
		svr.Http.Listeners = []string{"tcp://0.0.0.0:5655"}
		withReservedShellCommands(t, svr, "", "")

		err := svr.initShellProvider()
		require.NoError(t, err)

		shellDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_SHELL)
		require.NoError(t, err)
		require.Contains(t, shellDef.Command, "-server 127.0.0.1:5655")
	})

	t.Run("falls_back_to_first_tcp_candidate", func(t *testing.T) {
		svr := newShellTestServer(t)
		svr.Http.Listeners = []string{"tcp://10.10.1.20:5655", "tcp://10.10.1.21:5656"}
		withReservedShellCommands(t, svr, "", "")

		err := svr.initShellProvider()
		require.NoError(t, err)

		shellDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_SHELL)
		require.NoError(t, err)
		require.Contains(t, shellDef.Command, "-server 10.10.1.20:5655")
	})

	t.Run("keeps_existing_when_no_tcp_listener", func(t *testing.T) {
		svr := newShellTestServer(t)
		svr.Http.Listeners = []string{"unix:///tmp/neo.sock"}
		withReservedShellCommands(t, svr, "keep-shell", "keep-jsh")

		err := svr.initShellProvider()
		require.NoError(t, err)

		shellDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_SHELL)
		require.NoError(t, err)
		require.Equal(t, "keep-shell", shellDef.Command)

		jshDef, err := svr.models.ShellProvider().GetShell(model.SHELLID_JSH)
		require.NoError(t, err)
		require.Equal(t, "keep-jsh", jshDef.Command)
	})
}

func TestProvideShellForSsh(t *testing.T) {
	t.Run("missing_shell_returns_nil", func(t *testing.T) {
		svr := newShellTestServer(t)
		require.Nil(t, svr.provideShellForSsh("sys", "missing"))
	})

	t.Run("empty_command_returns_nil", func(t *testing.T) {
		svr := newShellTestServer(t)
		withReservedShellCommands(t, svr, "", "")
		require.Nil(t, svr.provideShellForSsh("sys", model.SHELLID_SHELL))
	})

	t.Run("parses_reserved_shell_command", func(t *testing.T) {
		svr := newShellTestServer(t)
		withReservedShellCommands(t, svr, `"/bin/sh" -l -c "echo ok"`, "")

		shell := svr.provideShellForSsh("sys", strings.ToLower(model.SHELLID_SHELL))
		require.NotNil(t, shell)
		require.Equal(t, "/bin/sh", shell.Cmd)
		require.Equal(t, []string{"-l", "-c", "echo ok"}, shell.Args)
		require.NotEmpty(t, shell.Envs)
	})

	t.Run("command_without_args_keeps_empty_args", func(t *testing.T) {
		svr := newShellTestServer(t)
		withReservedShellCommands(t, svr, "/bin/sh", "")

		shell := svr.provideShellForSsh("sys", model.SHELLID_SHELL)
		require.NotNil(t, shell)
		require.Equal(t, "/bin/sh", shell.Cmd)
		require.Empty(t, shell.Args)
	})
}
