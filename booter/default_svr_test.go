package booter

import (
	"testing"
)

func TestDefaultBooterConfiguration(t *testing.T) {
	previousBooter := defaultBooter
	previousBuilder := defaultBuilder
	previousConfig := conf
	previousFallbackConfig := fallbackConfigContent
	previousFallbackPname := fallbackPname
	previousArgs := Args
	t.Cleanup(func() {
		defaultBooter = previousBooter
		defaultBuilder = previousBuilder
		conf = previousConfig
		fallbackConfigContent = previousFallbackConfig
		fallbackPname = previousFallbackPname
		Args = previousArgs
	})

	defaultBooter = nil
	defaultBuilder = NewBuilder()
	conf = testBooterConfig()
	if Pname() != "" || VersionString() != "" {
		t.Fatalf("unexpected initial config: pname=%q version=%q", Pname(), VersionString())
	}
	SetVersionString("v1.2.3")
	SetFallbackConfig([]byte("module {}"))
	SetFallbackPname("fallback")
	SetConfigFileSuffix(".hcl")
	if VersionString() != "v1.2.3" || string(fallbackConfigContent) != "module {}" || fallbackPname != "fallback" {
		t.Fatalf("unexpected configured globals: version=%q config=%q pname=%q", VersionString(), fallbackConfigContent, fallbackPname)
	}

	startupCalled := false
	shutdownCalled := false
	AddStartupHook(func() { startupCalled = true })
	AddShutdownHook(func() { shutdownCalled = true })
	if startupCalled || shutdownCalled {
		t.Fatal("hooks executed while being registered")
	}

	if GetDefinition("missing") != nil || GetInstance("missing") != nil || GetConfig("missing") != nil {
		t.Fatal("nil default booter returned an object")
	}
	ShutdownAndExit(1)
	WaitSignal()
	NotifySignal()

	SetFlag(PnameFlag, "process-name", "n", "neo")
	flag := conf.flags[PnameFlag]
	if flag.Long != "process-name" || flag.Short != "n" || flag.Default != "neo" {
		t.Fatalf("SetFlag() produced %+v", flag)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("SetFlag() did not panic for an invalid flag type")
		}
	}()
	SetFlag(numofFlags, "invalid", "", "")
}

func TestDefaultBooterAccessorsWithBooter(t *testing.T) {
	previousBooter := defaultBooter
	previousConfig := conf
	t.Cleanup(func() {
		defaultBooter = previousBooter
		conf = previousConfig
	})

	instance := &boot{}
	defaultBooter = instance
	conf = testBooterConfig()
	conf.Pname = "neo"
	conf.versionString = "v2.0.0"
	if Pname() != "neo" || VersionString() != "v2.0.0" {
		t.Fatalf("unexpected accessors: pname=%q version=%q", Pname(), VersionString())
	}
	if GetDefinition("missing") != nil || GetInstance("missing") != nil || GetConfig("missing") != nil {
		t.Fatal("empty booter returned an object")
	}
	called := false
	AddShutdownHook(func() { called = true })
	if len(instance.shutdownHooks) != 1 || called {
		t.Fatal("shutdown hook was not registered on the running booter")
	}
	Shutdown()
}

func TestParseFlagsAndUsage(t *testing.T) {
	previousConfig := conf
	previousFallbackConfig := fallbackConfigContent
	previousFallbackPname := fallbackPname
	previousArgs := Args
	t.Cleanup(func() {
		conf = previousConfig
		fallbackConfigContent = previousFallbackConfig
		fallbackPname = previousFallbackPname
		Args = previousArgs
	})

	conf = testBooterConfig()
	conf.flags[ConfigDirFlag] = BootFlag{Long: "config-dir", Default: "default-dir"}
	conf.flags[PnameFlag] = BootFlag{Long: "pname"}
	conf.flags[DaemonFlag] = BootFlag{Long: "daemon", Short: "d", Default: "false"}
	conf.flags[HelpFlag] = BootFlag{Long: "help", Short: "h"}
	fallbackConfigContent = []byte("module {}")
	fallbackPname = "fallback-name"
	Args = []string{"neo", "--config-dir", "configured-dir", "--pname", "configured-name", "--daemon"}

	parseflags()
	if conf.ConfDir != "configured-dir" || conf.Pname != "configured-name" || !conf.Daemon {
		t.Fatalf("unexpected parsed config: %+v", conf)
	}
	usage()

	conf = testBooterConfig()
	conf.flags[PnameFlag] = BootFlag{Long: "pname"}
	conf.flags[DaemonFlag] = BootFlag{Long: "daemon", Default: "false"}
	conf.flags[HelpFlag] = BootFlag{Long: "help"}
	Args = []string{"neo"}
	parseflags()
	if conf.Pname != "fallback-name" {
		t.Fatalf("fallback pname was %q", conf.Pname)
	}
}

func TestConfigAccessorsWithNilConfig(t *testing.T) {
	previousConfig := conf
	t.Cleanup(func() { conf = previousConfig })
	conf = nil
	if Pname() != "" || VersionString() != "" {
		t.Fatal("nil config returned non-empty values")
	}
	SetVersionString("ignored")
}

func testBooterConfig() *Config {
	return &Config{flags: map[BootFlagType]BootFlag{
		ConfigDirFlag:  {Long: "config-dir"},
		ConfigFileFlag: {Long: "config", Short: "c"},
		GenConfigFlag:  {Long: "gen-config"},
		PnameFlag:      {Long: "pname"},
		PidFlag:        {Long: "pid"},
		BootlogFlag:    {Long: "bootlog"},
		DaemonFlag:     {Long: "daemon", Short: "d"},
		HelpFlag:       {Long: "help", Short: "h"},
	}}
}
