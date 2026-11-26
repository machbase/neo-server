package server

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectServerModules(t *testing.T) {
	serverDefinition := &configModuleDefinition{Id: serverModuleID, Priority: 2}
	loggingDefinition := &configModuleDefinition{Id: loggingModuleID, Priority: 1}

	loggingModule, serverModule, err := selectServerModules([]*configModuleDefinition{
		{Id: "optional/unavailable", Disabled: true},
		serverDefinition,
		loggingDefinition,
	})
	require.NoError(t, err)
	require.Same(t, loggingDefinition, loggingModule)
	require.Same(t, serverDefinition, serverModule)

	_, _, err = selectServerModules([]*configModuleDefinition{{Id: "unsupported/module"}})
	require.ErrorContains(t, err, "unsupported module")

	_, _, err = selectServerModules([]*configModuleDefinition{
		{Id: serverModuleID, Priority: 1, Injects: []configInjection{{Target: loggingModuleID, FieldName: "server"}}},
	})
	require.ErrorContains(t, err, "unsupported dependency injection")

	_, _, err = selectServerModules([]*configModuleDefinition{{Id: loggingModuleID}})
	require.ErrorContains(t, err, "must define module "+serverModuleID)
}

func TestValidateServerModuleOrder(t *testing.T) {
	loggingDefinition := &configModuleDefinition{Id: loggingModuleID, Priority: 2}
	serverDefinition := &configModuleDefinition{Id: serverModuleID, Priority: 1}

	require.ErrorContains(t, validateModuleOrder(loggingDefinition, serverDefinition), "must have a lower priority")
	require.NoError(t, validateModuleOrder(nil, serverDefinition))
	loggingDefinition.Disabled = true
	require.NoError(t, validateModuleOrder(loggingDefinition, serverDefinition))
}

func TestServerExecutionShutdownRequest(t *testing.T) {
	options := serverBootOptions{pname: "neo"}
	execution := newServerExecution(nil, "edge", true, options, nil, nil)
	waiting := make(chan struct{})
	go func() {
		execution.waitSignal()
		close(waiting)
	}()

	execution.requestShutdown()
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("shutdown request did not release waitSignal")
	}

	execution.shutdown()
	execution.shutdown()
	select {
	case <-execution.shutdownDone:
	default:
		t.Fatal("shutdown completion was not signaled")
	}
}

func TestServerExecutionUsesRunOptions(t *testing.T) {
	config := newConfig("edge", true)
	require.Equal(t, PresetEdge, config.MachbasePreset)
	require.True(t, config.headlessMode)
}

func TestServerStartFailureCanRetry(t *testing.T) {
	if os.Getenv("GO_WANT_DO_RESTORE_HELPER") != "1" {
		t.Skip("requires isolated server lifecycle")
	}
	for range 2 {
		config := newConfig("edge", true)
		config.DataDir = "machbase://127.0.0.1:5656"
		server, err := NewServer(config)
		require.NoError(t, err)
		execution := newServerExecution(nil, "edge", true, serverBootOptions{}, log.New(io.Discard, "", 0), nil)
		execution.server = server
		require.ErrorContains(t, execution.start(), "headless mode is not possible")
		execution.shutdown()
		execution.shutdown()
	}
}

func TestLoadServerConfigurationUsesRunContext(t *testing.T) {
	dataDir := t.TempDir()
	fileDir := t.TempDir()
	prefDir := t.TempDir()
	args := []string{
		"neo", "serve",
		"--data", dataDir,
		"--file", fileDir,
		"--pref", prefDir,
		"--log-level", "DEBUG",
		"--http-port", "9876",
	}

	loggingConfig, serverConfig, err := loadServerConfiguration(
		args,
		"edge",
		true,
		serverBootOptions{pname: "neo-test"},
		func() {},
	)
	require.NoError(t, err)
	require.Equal(t, "DEBUG", loggingConfig.DefaultLevel)
	require.Equal(t, 16, loggingConfig.DefaultPrefixWidth)
	require.Equal(t, dataDir, serverConfig.DataDir)
	require.Equal(t, prefDir, serverConfig.PrefDir)
	require.Equal(t, []string{"/=" + fileDir}, serverConfig.FileDirs)
	require.True(t, serverConfig.fileFlagSet)
	require.Equal(t, PresetEdge, serverConfig.MachbasePreset)
	require.True(t, serverConfig.headlessMode)
	require.NotNil(t, serverConfig.notifyShutdown)
	require.Contains(t, serverConfig.Http.Listeners, "tcp://127.0.0.1:9876")
}

func TestLoadServerConfigurationFlagParsing(t *testing.T) {
	for _, tt := range []struct {
		name        string
		args        []string
		wantFile    string
		wantFileSet bool
	}{
		{"first value wins", []string{"neo", "serve", "--file=/first", "--file=/second"}, "/=/first", true},
		{"terminator", []string{"neo", "serve", "--", "--file=/ignored"}, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			options, err := parseServerBootOptions(tt.args)
			require.NoError(t, err)
			_, config, err := loadServerConfiguration(tt.args, "", false, options, nil)
			require.NoError(t, err)
			require.Equal(t, tt.wantFileSet, config.fileFlagSet)
			if tt.wantFile != "" {
				require.Equal(t, []string{tt.wantFile}, config.FileDirs)
			} else {
				require.NotContains(t, config.FileDirs[0], "ignored")
			}
		})
	}
}

func TestLoadServerConfigurationRejectsMissingFlagValue(t *testing.T) {
	for _, args := range [][]string{
		{"neo", "serve", "--log-level"},
		{"neo", "serve", "--log-level", "--data", "data"},
		{"neo", "serve", "--log-level", "--log-level=DEBUG"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			options, err := parseServerBootOptions(args)
			require.NoError(t, err)
			_, _, err = loadServerConfiguration(args, "", false, options, nil)
			require.ErrorContains(t, err, "flag --log-level requires a value")
		})
	}
}

func TestLoadServerConfigurationSources(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "server.conf")
	content := []byte("module \"machbase.com/neo-server\" {\n config { DataDir = \"from-file\" }\n}\n")
	require.NoError(t, os.WriteFile(configFile, content, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.hcl"), []byte("invalid {{{"), 0o644))

	for _, options := range []serverBootOptions{
		{configFile: configFile, configDir: filepath.Join(dir, "missing")},
		{configDir: dir},
	} {
		_, config, err := loadServerConfiguration(nil, "edge", false, options, nil)
		require.NoError(t, err)
		require.Equal(t, "from-file", config.DataDir)
	}

	_, _, err := loadServerConfiguration(nil, "edge", false, serverBootOptions{configFile: filepath.Join(dir, "missing")}, nil)
	require.Error(t, err)
}

func TestLoadServerConfigurationIgnoresDisabledUnsupportedModule(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "server.conf")
	content := []byte(`
module "optional/unsupported" {
  disabled = true
  config { Value = flagOrError("--not-needed") }
}
module "machbase.com/neo-server" {}
`)
	require.NoError(t, os.WriteFile(configFile, content, 0o644))
	_, config, err := loadServerConfiguration(nil, "edge", false, serverBootOptions{configFile: configFile}, nil)
	require.NoError(t, err)
	require.NotNil(t, config)
}

func TestLoadServerConfigurationRejectsInvalidModules(t *testing.T) {
	tests := []struct {
		name    string
		content string
		err     string
	}{
		{"duplicate", `module "machbase.com/neo-server" {}` + "\n" + `module "machbase.com/neo-server" {}`, "duplicate module"},
		{"disabled server", `module "machbase.com/neo-server" { disabled = true }`, "server must be enabled"},
		{"invalid config", "module \"machbase.com/neo-server\" {\n config { MaxOpenConn = null }\n}", "Config.MaxOpenConn"},
		{"wrong order", "module \"machbase.com/neo-server\" { priority = 1 }\nmodule \"machbase.com/neo-logging\" { priority = 2 }", "must have a lower priority"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "server.conf")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.content), 0o644))
			_, _, err := loadServerConfiguration(nil, "edge", false, serverBootOptions{configFile: configFile}, nil)
			require.ErrorContains(t, err, tt.err)
		})
	}
}

func TestServerExecutionLoadsConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "server.conf")
	require.NoError(t, os.WriteFile(configFile, []byte(`module "machbase.com/neo-server" {}`), 0o644))
	execution := newServerExecution(nil, "edge", true, serverBootOptions{configFile: configFile}, log.New(io.Discard, "", 0), nil)
	defer execution.shutdown()
	require.NoError(t, execution.loadConfig())
	require.NotNil(t, execution.server)
	require.True(t, execution.server.headlessMode)
}
