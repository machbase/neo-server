package booter

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"testing"
	"time"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/stretchr/testify/require"
)

var AmodId = "github.com/booter/amod"
var BmodId = "github.com/booter/bmod"

var customFunc function.Function

func TestMain(m *testing.M) {
	Args = []string{
		"--logging-default-level", "WARN",
		"--logging-default-enable-source-location", "true",
		"--logging-default-prefix-width", "30",
	}

	Register(AmodId,
		func() *AmodConf {
			return new(AmodConf)
		},
		func(conf *AmodConf) (Boot, error) {
			instance := &Amod{
				conf: conf,
			}
			return instance, nil
		})
	Register(BmodId,
		func() *BmodConf {
			return new(BmodConf)
		},
		func(conf *BmodConf) (Boot, error) {
			instance := &Bmod{
				conf: *conf,
			}
			return instance, nil
		})
	customFunc = function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal("1.2.3"), nil
		},
	})

	m.Run()
}

func TestParser(t *testing.T) {
	DefaultFunctions["customfunc"] = customFunc
	defs, err := LoadDefinitionFiles(
		[]string{
			"./test/999_mod_others.hcl",
			"./test/000_mod_env.hcl",
			"./test/002_mod_bmod.hcl",
			"./test/001_mod_amod.hcl",
		},
		nil,
	)
	delete(DefaultFunctions, "customfunc")

	require.Nil(t, err)
	require.Equal(t, 3, len(defs))
}

func TestParserDir(t *testing.T) {
	builder := NewBuilder()
	builder.SetFunction("customfunc", customFunc)

	bt, err := builder.BuildWithDir("./test")
	require.Nil(t, err)
	require.NotNil(t, bt)
}

func TestBoot(t *testing.T) {
	builder := NewBuilder()
	builder.SetFunction("customfunc", customFunc)

	b, err := builder.BuildWithDir("./test")
	require.Nil(t, err)

	err = b.Startup()
	require.Nil(t, err)

	def := b.GetDefinition(AmodId)
	require.NotNil(t, def)
	require.Equal(t, 201, def.Priority)
	require.Equal(t, false, def.Disabled)
	aconf := b.GetConfig(AmodId).(*AmodConf)
	require.Equal(t, "127.0.0.1:1884", aconf.TcpConfig.ListenAddress)
	require.Equal(t, "mqtts://10.10.10.1:1884", aconf.TcpConfig.AdvertiseAddress.String())
	require.Equal(t, true, aconf.TcpConfig.Tls.LoadPrivateCAs)
	require.Equal(t, "./test/test_server_cert.pem", aconf.TcpConfig.Tls.CertFile)
	require.Equal(t, "./test/test_server_key.pem", aconf.TcpConfig.Tls.KeyFile)
	require.Equal(t, 5*time.Second, aconf.TcpConfig.Tls.HandshakeTimeout)
	require.Equal(t, "1.2.3", aconf.Version)
	require.Equal(t, 2*time.Hour, aconf.Dur2h)
	require.Equal(t, 24*time.Hour, aconf.Dur24h)
	// check if injection works
	amod := b.GetInstance(AmodId).(*Amod)
	require.NotNil(t, amod)
	require.NotNil(t, amod.Bmod)

	def = b.GetDefinition(BmodId)
	require.NotNil(t, def)
	require.Equal(t, 202, def.Priority)
	require.Equal(t, false, def.Disabled)
	bconf := b.GetConfig(BmodId).(*BmodConf)
	home := "."
	if str := os.Getenv("HOME"); str != "" {
		home = str
	}
	require.Equal(t, fmt.Sprintf("%s/./tmp/cmqd00.log", home), bconf.Filename)
	require.Equal(t, true, bconf.Append)
	require.Equal(t, "@midnight", bconf.RotateSchedule)
	require.Equal(t, 3, bconf.MaxBackups)
	require.Equal(t, 3, len(bconf.Levels))
	require.Equal(t, 30, bconf.DefaultPrefixWidth)
	require.Equal(t, "WARN", bconf.DefaultLevel)
	require.Equal(t, true, bconf.DefaultEnableSourceLocation)

	b.Shutdown()
}

type AmodConf struct {
	Version   string
	TcpConfig TcpConfig
	Timeout   time.Duration
	Dur2h     time.Duration
	Dur24h    time.Duration
}

type TcpConfig struct {
	ListenAddress    string
	AdvertiseAddress url.URL
	SoLinger         int
	KeepAlive        int
	NoDelay          bool
	Tls              TlsConfig
}

type TlsConfig struct {
	LoadSystemCAs    bool
	LoadPrivateCAs   bool
	CertFile         string
	KeyFile          string
	HandshakeTimeout time.Duration
}

type Amod struct {
	conf             *AmodConf
	Bmod             *Bmod
	OtherNameForBmod BmodInterface
}

func (am *Amod) Start() error {
	fmt.Println("amod start")
	if am.Bmod != am.OtherNameForBmod {
		return errors.New("amod.Bmod and amod.OtherNameForBmod has different references")
	}
	am.OtherNameForBmod.DoWork()
	return nil
}

func (am *Amod) Stop() {
	fmt.Println("amod stop")
}

type BmodConf struct {
	Filename                    string
	Append                      bool
	MaxBackups                  int
	RotateSchedule              string
	DefaultLevel                string
	DefaultPrefixWidth          int
	DefaultEnableSourceLocation bool
	Levels                      []LevelConf
}

type LevelConf struct {
	Pattern string
	Level   string
}

type BmodInterface interface {
	DoWork()
}

type Bmod struct {
	conf BmodConf
}

func (bm *Bmod) Start() error {
	fmt.Println("bmod start")
	return nil
}

func (bm *Bmod) Stop() {
	fmt.Println("bmod stop")
}

func (bm *Bmod) DoWork() {
	fmt.Println("bmod work...")
}

type hookTestConf struct{}

type hookTestBoot struct {
	name   string
	events *[]string
}

func (hb *hookTestBoot) Start() error {
	*hb.events = append(*hb.events, "start-"+hb.name)
	return nil
}

func (hb *hookTestBoot) Stop() {
	*hb.events = append(*hb.events, "stop-"+hb.name)
}

type variableTestConf struct {
	Version string
}

type variableTestBoot struct{}

func (vb *variableTestBoot) Start() error { return nil }

func (vb *variableTestBoot) Stop() {}

type evalNestedConf struct {
	Name string
}

type evalComplexConf struct {
	Flag    bool
	Count   int
	Size    uint64
	Rate    float64
	Target  url.URL
	Names   []string
	Options map[string]bool
	Nested  evalNestedConf
	Timeout time.Duration
}

type unsupportedEvalConf struct {
	Ch chan int
}

func TestBuilderBuildWithContent(t *testing.T) {
	builder := NewBuilder()
	builder.SetFunction("customfunc", customFunc)

	envContent, err := os.ReadFile("./test/000_mod_env.hcl")
	require.NoError(t, err)
	amodContent, err := os.ReadFile("./test/001_mod_amod.hcl")
	require.NoError(t, err)
	bmodContent, err := os.ReadFile("./test/002_mod_bmod.hcl")
	require.NoError(t, err)

	content := append([]byte{}, envContent...)
	content = append(content, '\n')
	content = append(content, amodContent...)
	content = append(content, '\n')
	content = append(content, bmodContent...)

	b, err := builder.BuildWithContent(content)
	require.NoError(t, err)
	require.NotNil(t, b)

	err = b.Startup()
	require.NoError(t, err)
	t.Cleanup(func() { b.Shutdown() })

	aconf := b.GetConfig(AmodId).(*AmodConf)
	require.Equal(t, "1.2.3", aconf.Version)
	require.Equal(t, "127.0.0.1:1884", aconf.TcpConfig.ListenAddress)

	bconf := b.GetConfig(BmodId).(*BmodConf)
	require.Equal(t, 3, bconf.MaxBackups)
	require.Equal(t, "WARN", bconf.DefaultLevel)
}

func TestBuilderHooksOrder(t *testing.T) {
	moduleOneID := "github.com/booter/hookmod1"
	moduleTwoID := "github.com/booter/hookmod2"
	events := []string{}

	Register(moduleOneID,
		func() *hookTestConf { return &hookTestConf{} },
		func(conf *hookTestConf) (Boot, error) {
			return &hookTestBoot{name: "one", events: &events}, nil
		})
	Register(moduleTwoID,
		func() *hookTestConf { return &hookTestConf{} },
		func(conf *hookTestConf) (Boot, error) {
			return &hookTestBoot{name: "two", events: &events}, nil
		})
	t.Cleanup(func() {
		UnregisterBootFactory(moduleOneID)
		UnregisterBootFactory(moduleTwoID)
	})

	builder := NewBuilder()
	builder.AddStartupHook(func() { events = append(events, "startup-hook") })
	builder.AddShutdownHook(func() { events = append(events, "shutdown-hook") })

	content := []byte(fmt.Sprintf(`
module %q {
  name = "hook-one"
  config {}
}

module %q {
  name = "hook-two"
  config {}
}
`, moduleOneID, moduleTwoID))

	b, err := builder.BuildWithContent(content)
	require.NoError(t, err)

	require.NoError(t, b.Startup())
	b.Shutdown()

	require.Equal(t,
		[]string{"startup-hook", "start-one", "start-two", "shutdown-hook", "stop-two", "stop-one"},
		events,
	)
}

func TestBuilderSetVariable(t *testing.T) {
	moduleID := "github.com/booter/variablemod"
	Register(moduleID,
		func() *variableTestConf { return &variableTestConf{} },
		func(conf *variableTestConf) (Boot, error) {
			return &variableTestBoot{}, nil
		})
	t.Cleanup(func() { UnregisterBootFactory(moduleID) })

	builder := NewBuilder()
	require.NoError(t, builder.SetVariable("CUSTOM_VERSION", "9.9.9"))

	content := []byte(fmt.Sprintf(`
module %q {
  name = "variablemod"
  config {
    Version = CUSTOM_VERSION
  }
}
`, moduleID))

	b, err := builder.BuildWithContent(content)
	require.NoError(t, err)
	require.NoError(t, b.Startup())
	t.Cleanup(func() { b.Shutdown() })

	conf := b.GetConfig(moduleID).(*variableTestConf)
	require.Equal(t, "9.9.9", conf.Version)
}

func TestBuilderSetVariableErrors(t *testing.T) {
	builder := NewBuilder()
	require.EqualError(t, builder.SetVariable("", "value"), "can not define with empty name")
	require.EqualError(t, builder.SetVariable("BAD", []string{"x"}), "can not define BAD with value type []string")
}

func TestBuilderSetConfigFileSuffix(t *testing.T) {
	moduleID := "github.com/booter/suffixmod"
	Register(moduleID,
		func() *variableTestConf { return &variableTestConf{} },
		func(conf *variableTestConf) (Boot, error) {
			return &variableTestBoot{}, nil
		})
	t.Cleanup(func() { UnregisterBootFactory(moduleID) })

	builder := NewBuilder()
	builder.SetConfigFileSuffix(".cfg")

	configDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "module.cfg"), []byte(fmt.Sprintf(`
module %q {
  name = "suffixmod"
  config {
    Version = "2.0.0"
  }
}
`, moduleID)), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "ignored.hcl"), []byte("not valid hcl {{{"), 0644))

	b, err := builder.BuildWithDir(configDir)
	require.NoError(t, err)
	require.NoError(t, b.Startup())
	t.Cleanup(func() { b.Shutdown() })

	conf := b.GetConfig(moduleID).(*variableTestConf)
	require.Equal(t, "2.0.0", conf.Version)
}

func TestBootAddShutdownHook(t *testing.T) {
	b, err := NewWithDefinitions(nil)
	require.NoError(t, err)

	called := false
	b.AddShutdownHook(func() { called = true })
	b.Shutdown()

	require.True(t, called)
}

func TestBootShutdownAndExit(t *testing.T) {
	if os.Getenv("BOOTER_HELPER_PROCESS") == "1" {
		b, err := NewWithDefinitions(nil)
		if err != nil {
			os.Exit(91)
		}
		b.ShutdownAndExit(7)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestBootShutdownAndExit")
	cmd.Env = append(os.Environ(), "BOOTER_HELPER_PROCESS=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 7, exitErr.ExitCode())
}

func TestLoadAndLoadFile(t *testing.T) {
	body, err := Load([]byte("module \"example\" {\n  config {}\n}"))
	require.NoError(t, err)
	require.NotNil(t, body)

	_, err = Load([]byte("not valid hcl {{{"))
	require.Error(t, err)

	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.hcl")
	invalidPath := filepath.Join(tmpDir, "invalid.hcl")
	require.NoError(t, os.WriteFile(validPath, []byte("module \"example\" {\n  config {}\n}"), 0644))
	require.NoError(t, os.WriteFile(invalidPath, []byte("not valid hcl {{{"), 0644))

	body, err = LoadFile(validPath)
	require.NoError(t, err)
	require.NotNil(t, body)

	_, err = LoadFile(invalidPath)
	require.Error(t, err)
}

func TestEvalObjectCoversHelpers(t *testing.T) {
	conf := &evalComplexConf{}
	err := EvalObject("EvalConf", conf, cty.ObjectVal(map[string]cty.Value{
		"Flag":    cty.StringVal("yes"),
		"Count":   cty.StringVal("12"),
		"Size":    cty.StringVal("2h"),
		"Rate":    cty.StringVal("3.5"),
		"Target":  cty.StringVal("https://example.com:9443"),
		"Names":   cty.ListVal([]cty.Value{cty.StringVal("alpha"), cty.StringVal("beta")}),
		"Options": cty.ObjectVal(map[string]cty.Value{"enabled": cty.StringVal("true")}),
		"Nested":  cty.ObjectVal(map[string]cty.Value{"Name": cty.StringVal("nested")}),
		"Timeout": cty.StringVal("1500ms"),
	}))
	require.NoError(t, err)

	require.True(t, conf.Flag)
	require.Equal(t, 12, conf.Count)
	require.Equal(t, uint64(2*time.Hour), conf.Size)
	require.Equal(t, 3.5, conf.Rate)
	require.Equal(t, "https://example.com:9443", conf.Target.String())
	require.Equal(t, []string{"alpha", "beta"}, conf.Names)
	require.Equal(t, map[string]bool{"enabled": true}, conf.Options)
	require.Equal(t, "nested", conf.Nested.Name)
	require.Equal(t, 1500*time.Millisecond, conf.Timeout)
}

func TestEvalObjectErrors(t *testing.T) {
	err := EvalObject("Empty", &struct{}{}, cty.ObjectVal(map[string]cty.Value{
		"Missing": cty.StringVal("value"),
	}))
	require.EqualError(t, err, "Missing field not found in Empty")

	err = EvalObject("Unsupported", &unsupportedEvalConf{}, cty.ObjectVal(map[string]cty.Value{
		"Ch": cty.StringVal("noop"),
	}))
	require.EqualError(t, err, "unsupported reflection Unsupported.Ch type: chan")

	_, err = IntFromCty(cty.BoolVal(true))
	require.ErrorContains(t, err, "value is not a number")

	_, err = Uint64FromCty(cty.StringVal("oops"))
	require.ErrorContains(t, err, "invalid syntax")

	_, err = Float64FromCty(cty.BoolVal(true))
	require.ErrorContains(t, err, "value is not a number")
}

func TestBootWaitAndNotifySignalInternal(t *testing.T) {
	bt := &boot{}
	done := make(chan struct{})

	go func() {
		bt.WaitSignal()
		close(done)
	}()

	require.Eventually(t, func() bool {
		return bt.quitChan != nil
	}, time.Second, 10*time.Millisecond)

	go bt.NotifySignal()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	signal.Stop(bt.quitChan)
}
