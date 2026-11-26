package server

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func TestLoadDefinitionFixtures(t *testing.T) {
	functions := configFunctions(configEvalContext{Args: []string{
		"neo", "--logging-default-level", "WARN",
		"--logging-default-enable-source-location", "true",
		"--logging-default-prefix-width", "30",
	}})
	functions["customfunc"] = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func([]cty.Value, cty.Type) (cty.Value, error) {
			return cty.StringVal("1.2.3"), nil
		},
	})
	defs, err := loadConfigDefinitionFiles([]string{
		"./test/bootconfig/999_mod_others.hcl",
		"./test/bootconfig/000_mod_env.hcl",
		"./test/bootconfig/002_mod_bmod.hcl",
		"./test/bootconfig/001_mod_amod.hcl",
	}, &hcl.EvalContext{Functions: functions, Variables: map[string]cty.Value{}})
	require.NoError(t, err)
	require.Len(t, defs, 3)
	require.Equal(t, "github.com/booter/amod", defs[0].Id)
	require.Equal(t, 201, defs[0].Priority)
	conf := &struct {
		Version   string
		TcpConfig struct {
			ListenAddress    string
			AdvertiseAddress url.URL
			SoLinger         int
			KeepAlive        int
			NoDelay          bool
			Tls              struct {
				LoadSystemCAs    bool
				LoadPrivateCAs   bool
				CertFile         string
				KeyFile          string
				HandshakeTimeout time.Duration
			}
		}
		Timeout time.Duration
		Dur2h   time.Duration
		Dur24h  time.Duration
	}{}
	require.NoError(t, evalConfigObject("Config", conf, defs[0].Config))
	require.Equal(t, "1.2.3", conf.Version)
	require.Equal(t, "127.0.0.1:1884", conf.TcpConfig.ListenAddress)
	require.Equal(t, "mqtts://10.10.10.1:1884", conf.TcpConfig.AdvertiseAddress.String())
	require.Equal(t, 5*time.Second, conf.TcpConfig.Tls.HandshakeTimeout)
	require.Equal(t, 2*time.Hour, conf.Dur2h)
	require.Len(t, defs[1].Injects, 2)
	var logging struct {
		Filename                    string
		Append                      bool
		MaxBackups                  int
		RotateSchedule              string
		DefaultLevel                string
		DefaultPrefixWidth          int
		DefaultEnableSourceLocation bool
		Levels                      []struct{ Pattern, Level string }
	}
	require.NoError(t, evalConfigObject("Config", &logging, defs[1].Config))
	require.True(t, logging.Append)
	require.Equal(t, 3, logging.MaxBackups)
	require.Equal(t, "@midnight", logging.RotateSchedule)
	require.Equal(t, "WARN", logging.DefaultLevel)
	require.Equal(t, 30, logging.DefaultPrefixWidth)
	require.True(t, logging.DefaultEnableSourceLocation)
	require.Len(t, logging.Levels, 3)
}

func TestConfigLoaderExecutionContext(t *testing.T) {
	args := []string{"neo", "serve", "--value=from-context"}
	path := filepath.Join(t.TempDir(), "server.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
module "example" {
  config {
    FlagValue = flag("--value", "fallback")
    Argument = arg(1, "missing")
    ArgCount = arglen()
    Pname = pname()
    Version = version()
  }
}`), 0o644))
	defs, err := loadServerConfigDefinitions(
		configEvalContext{Args: args, Pname: "neo-worker", Version: "1.2.3"},
		serverBootOptions{configFile: path},
	)
	require.NoError(t, err)
	require.Len(t, defs, 1)
	var config struct {
		FlagValue string
		Argument  string
		ArgCount  int
		Pname     string
		Version   string
	}
	require.NoError(t, evalConfigObject("Config", &config, defs[0].Config))
	require.Equal(t, "from-context", config.FlagValue)
	require.Equal(t, "serve", config.Argument)
	require.Equal(t, 3, config.ArgCount)
	require.Equal(t, "neo-worker", config.Pname)
	require.Equal(t, "1.2.3", config.Version)
}

func TestConfigLoaderDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "001.conf"), []byte("define VAR { VALUE = \"configured\" }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "002.conf"), []byte("module \"example\" {\n config { Value = VAR_VALUE }\n}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.hcl"), []byte("not valid hcl {{{"), 0o644))

	defs, err := loadServerConfigDefinitions(configEvalContext{}, serverBootOptions{configDir: dir})
	require.NoError(t, err)
	require.Len(t, defs, 1)
	var config struct{ Value string }
	require.NoError(t, evalConfigObject("Config", &config, defs[0].Config))
	require.Equal(t, "configured", config.Value)
	_, err = loadServerConfigDefinitions(configEvalContext{}, serverBootOptions{configDir: filepath.Join(dir, "missing")})
	require.ErrorContains(t, err, "invalid config directory")
}

func TestConfigFlagArguments(t *testing.T) {
	functions := configFunctions(configEvalContext{Args: []string{
		"neo", "serve", "--custom", "first", "--custom=second", "--negative", "-2", "--dash", "-", "--missing", "--", "--ignored=value",
	}})
	value, err := functions["flag"].Call([]cty.Value{cty.StringVal("--custom"), cty.StringVal("fallback")})
	require.NoError(t, err)
	require.Equal(t, "first", value.AsString())
	value, err = functions["flag"].Call([]cty.Value{cty.StringVal("--negative"), cty.StringVal("fallback")})
	require.NoError(t, err)
	require.Equal(t, "-2", value.AsString())
	value, err = functions["flag"].Call([]cty.Value{cty.StringVal("--dash"), cty.StringVal("fallback")})
	require.NoError(t, err)
	require.Equal(t, "-", value.AsString())
	_, err = functions["flag"].Call([]cty.Value{cty.StringVal("--missing"), cty.StringVal("fallback")})
	require.ErrorContains(t, err, "flag --missing requires a value")
	_, err = functions["flagOrError"].Call([]cty.Value{cty.StringVal("--missing")})
	require.ErrorContains(t, err, "flag --missing requires a value")
	value, err = functions["flag"].Call([]cty.Value{cty.StringVal("--ignored"), cty.StringVal("fallback")})
	require.NoError(t, err)
	require.Equal(t, "fallback", value.AsString())
	_, err = functions["flagOrError"].Call([]cty.Value{cty.StringVal("--ignored")})
	require.ErrorContains(t, err, "required flag --ignored missing")
}

func TestLoadAndLoadFile(t *testing.T) {
	body, err := loadConfigContent([]byte("module \"example\" {\n  config {}\n}"))
	require.NoError(t, err)
	require.NotNil(t, body)
	_, err = loadConfigContent([]byte("not valid hcl {{{"))
	require.Error(t, err)

	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.hcl")
	invalidPath := filepath.Join(dir, "invalid.hcl")
	require.NoError(t, os.WriteFile(validPath, []byte("module \"example\" {\n  config {}\n}"), 0o644))
	require.NoError(t, os.WriteFile(invalidPath, []byte("not valid hcl {{{"), 0o644))
	body, err = loadConfigFiles(validPath)
	require.NoError(t, err)
	require.NotNil(t, body)
	_, err = loadConfigFiles(invalidPath)
	require.Error(t, err)
}

func TestEvalObjectCoversHelpers(t *testing.T) {
	var conf struct {
		Flag    bool
		Count   int
		Size    uint64
		Rate    float64
		Target  url.URL
		Names   []string
		Nested  struct{ Name string }
		Timeout time.Duration
	}
	err := evalConfigObject("EvalConf", &conf, cty.ObjectVal(map[string]cty.Value{
		"Flag":    cty.StringVal("yes"),
		"Count":   cty.StringVal("12"),
		"Size":    cty.StringVal("2h"),
		"Rate":    cty.StringVal("3.5"),
		"Target":  cty.StringVal("https://example.com:9443"),
		"Names":   cty.ListVal([]cty.Value{cty.StringVal("alpha"), cty.StringVal("beta")}),
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
	require.Equal(t, "nested", conf.Nested.Name)
	require.Equal(t, 1500*time.Millisecond, conf.Timeout)
}

func TestEvalObjectErrors(t *testing.T) {
	err := evalConfigObject("Empty", &struct{}{}, cty.ObjectVal(map[string]cty.Value{
		"Missing": cty.StringVal("value"),
	}))
	require.EqualError(t, err, "Missing field not found in Empty")
	err = evalConfigObject("Unsupported", &struct{ Ch chan int }{}, cty.ObjectVal(map[string]cty.Value{
		"Ch": cty.StringVal("noop"),
	}))
	require.EqualError(t, err, "unsupported reflection Unsupported.Ch type: chan")
	_, err = uint64FromCty(cty.StringVal("oops"))
	require.ErrorContains(t, err, "invalid syntax")
	_, err = float64FromCty(cty.BoolVal(true))
	require.ErrorContains(t, err, "value is not a number")
}

func TestInvalidModuleAttributes(t *testing.T) {
	for _, attribute := range []string{
		`disabled = "invalid"`,
		`disabled = null`,
		`priority = "high"`,
		`priority = 1.5`,
		`priority = 999999999999999999999999999999`,
		`name = false`,
	} {
		t.Run(attribute, func(t *testing.T) {
			_, err := loadConfigDefinitions([]byte("module \"test\" {\n"+attribute+"\n}"), nil)
			require.ErrorContains(t, err, "module test")
		})
	}
}

func TestInvalidConfigValues(t *testing.T) {
	tests := []struct {
		name  string
		value cty.Value
	}{
		{"null list", cty.ObjectVal(map[string]cty.Value{"FileDirs": cty.NullVal(cty.List(cty.String))})},
		{"wrong list type", cty.ObjectVal(map[string]cty.Value{"FileDirs": cty.StringVal("invalid")})},
		{"wrong element type", cty.ObjectVal(map[string]cty.Value{"FileDirs": cty.TupleVal([]cty.Value{cty.NumberIntVal(1)})})},
		{"unexported field", cty.ObjectVal(map[string]cty.Value{"headlessMode": cty.BoolVal(true)})},
		{"fractional integer", cty.ObjectVal(map[string]cty.Value{"MaxOpenConn": cty.NumberFloatVal(1.5)})},
		{"integer overflow", cty.ObjectVal(map[string]cty.Value{"MaxOpenConn": cty.NumberFloatVal(1e30)})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evalConfigObject("Config", newConfig("edge", false), tt.value)
			require.ErrorContains(t, err, "Config.")
		})
	}
}

func TestConfigNumericBounds(t *testing.T) {
	tests := []struct {
		name  string
		value cty.Value
	}{
		{"negative unsigned", cty.ObjectVal(map[string]cty.Value{"Unsigned": cty.NumberIntVal(-1)})},
		{"unsigned overflow", cty.ObjectVal(map[string]cty.Value{"Unsigned": cty.NumberIntVal(256)})},
		{"duration overflow", cty.ObjectVal(map[string]cty.Value{"Duration": cty.StringVal("9223372036854775807h")})},
		{"nonfinite float", cty.ObjectVal(map[string]cty.Value{"Float": cty.StringVal("NaN")})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &struct {
				Unsigned uint8
				Duration time.Duration
				Float    float64
			}{}
			require.ErrorContains(t, evalConfigObject("Config", config, tt.value), "Config.")
		})
	}
}
