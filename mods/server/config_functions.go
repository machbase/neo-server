package server

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"github.com/zclconf/go-cty/cty/gocty"
)

func configFunctions(context configEvalContext) map[string]function.Function {
	functions := map[string]function.Function{
		"env":         configEnvFunc,
		"envOrError":  configEnvRequiredFunc,
		"execFile":    configExecutableFileFunc,
		"execDir":     configExecutableDirFunc,
		"tempDir":     configTempDirFunc,
		"userDir":     configUserHomeDirFunc,
		"userConfDir": configUserConfigDirFunc,
		"prefDir":     configPrefDirFunc,
		"upper":       stdlib.UpperFunc,
		"lower":       stdlib.LowerFunc,
		"min":         stdlib.MinFunc,
		"max":         stdlib.MaxFunc,
		"strlen":      stdlib.StrlenFunc,
		"substr":      stdlib.SubstrFunc,
	}

	args := append([]string(nil), context.Args...)
	arguments := context.Arguments
	if arguments == nil {
		arguments = scanServerArguments(args)
	}
	functions["flag"] = flagFunction(arguments, false)
	functions["flagOrError"] = flagFunction(arguments, true)
	functions["arg"] = argFunction(args, false)
	functions["argOrError"] = argFunction(args, true)
	functions["arglen"] = function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.Number),
		Impl: func([]cty.Value, cty.Type) (cty.Value, error) {
			return cty.NumberIntVal(int64(len(args))), nil
		},
	})
	functions["pname"] = stringValueFunction(context.Pname)
	functions["version"] = stringValueFunction(context.Version)
	return functions
}

func stringValueFunction(value string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func([]cty.Value, cty.Type) (cty.Value, error) {
			return cty.StringVal(value), nil
		},
	})
}

func flagFunction(args []serverArgument, required bool) function.Function {
	params := []function.Parameter{
		{Name: "flag", Type: cty.String, AllowDynamicType: true},
	}
	if !required {
		params = append(params, function.Parameter{Name: "default", Type: cty.String, AllowNull: true})
	}
	return function.New(&function.Spec{
		Params: params,
		Type:   function.StaticReturnType(cty.String),
		Impl: func(values []cty.Value, _ cty.Type) (cty.Value, error) {
			name := values[0].AsString()
			result := ""
			if !required && !values[1].IsNull() {
				result = values[1].AsString()
			}
			for _, arg := range args {
				if arg.name == name {
					if arg.hasValue {
						return cty.StringVal(arg.value), nil
					}
					return cty.NilVal, fmt.Errorf("flag %s requires a value", name)
				}
			}
			if required {
				return cty.NilVal, fmt.Errorf("required flag %s missing", name)
			}
			return cty.StringVal(result), nil
		},
	})
}

func argFunction(args []string, required bool) function.Function {
	params := []function.Parameter{
		{Name: "argi", Type: cty.Number, AllowDynamicType: true},
	}
	if !required {
		params = append(params, function.Parameter{Name: "default", Type: cty.String, AllowNull: true})
	}
	return function.New(&function.Spec{
		Params: params,
		Type:   function.StaticReturnType(cty.String),
		Impl: func(values []cty.Value, _ cty.Type) (cty.Value, error) {
			var index int
			if err := gocty.FromCtyValue(values[0], &index); err != nil {
				return cty.NilVal, err
			}
			positionals := make([]string, 0, len(args))
			for i, arg := range args {
				if i != 0 && strings.HasPrefix(arg, "-") {
					continue
				}
				positionals = append(positionals, arg)
			}
			if index < 0 || index >= len(positionals) {
				if required {
					return cty.NilVal, fmt.Errorf("required argument %d missing", index)
				}
				return values[1], nil
			}
			return cty.StringVal(positionals[index]), nil
		},
	})
}

var configTempDirFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		dirPath := "/tmp"
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			dirPath = "/tmp"
		} else if runtime.GOOS == "windows" {
			dirPath = util.GetTempDirPath()
		} else {
			if d, err := filepath.Abs(os.TempDir()); err != nil {
				return cty.NilVal, err
			} else {
				dirPath = d
			}
		}
		return cty.StringVal(dirPath), nil
	},
})

var configExecutableFileFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		exePath, err := os.Executable()
		if err != nil {
			return cty.NilVal, err
		}
		filePath, err := filepath.Abs(exePath)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(filePath), nil
	},
})

var configExecutableDirFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		exePath, err := os.Executable()
		if err != nil {
			return cty.NilVal, err
		}
		exeDirPath := filepath.Dir(exePath)
		dirPath, err := filepath.Abs(exeDirPath)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(dirPath), nil
	},
})

var configUserHomeDirFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		homePath, err := os.UserHomeDir()
		if err != nil {
			return cty.NilVal, err
		}
		dirPath, err := filepath.Abs(homePath)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(dirPath), nil
	},
})

var configUserConfigDirFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		confPath, err := os.UserConfigDir()
		if err != nil {
			return cty.NilVal, err
		}
		dirPath, err := filepath.Abs(confPath)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(dirPath), nil
	},
})

var configPrefDirFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "str",
			Type:             cty.String,
			AllowDynamicType: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		in := args[0].AsString()
		homePath, err := os.UserHomeDir()
		if err != nil {
			return cty.NilVal, err
		}
		dirPath, err := filepath.Abs(homePath)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(filepath.Join(dirPath, ".config", in)), nil
	},
})

var configEnvRequiredFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "env",
			Type:             cty.String,
			AllowDynamicType: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		in := args[0].AsString()
		out, ok := os.LookupEnv(in)
		if !ok {
			return cty.NilVal, fmt.Errorf("required env variable %s missing", in)
		}
		return cty.StringVal(out), nil
	},
})

var configEnvFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "env",
			Type:             cty.String,
			AllowDynamicType: true,
		},
		{
			Name:      "default",
			Type:      cty.String,
			AllowNull: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		in := args[0].AsString()
		def := ""
		if !args[1].IsNull() {
			def = args[1].AsString()
		}
		out, ok := os.LookupEnv(in)
		if !ok {
			out = def
		}
		return cty.StringVal(out), nil
	},
})
