package booter

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

var DefaultFunctions = map[string]function.Function{
	"env":         GetEnvFunc,
	"envOrError":  GetEnv2Func,
	"flag":        GetFlagFunc,
	"flagOrError": GetFlag2Func,
	"arg":         GetArgFunc,
	"argOrError":  GetArg2Func,
	"arglen":      GetArgLenFunc,
	"pname":       GetPnameFunc,
	"version":     GetVersionFunc,
	"execFile":    GetExecutableFileFunc,
	"execDir":     GetExecutableDirFunc,
	"tempDir":     GetTempDirFunc,
	"userDir":     GetUserHomeDirFunc,
	"userConfDir": GetUserConfigDirFunc,
	"prefDir":     GetPrefDirFunc,
	"upper":       stdlib.UpperFunc,
	"lower":       stdlib.LowerFunc,
	"min":         stdlib.MinFunc,
	"max":         stdlib.MaxFunc,
	"strlen":      stdlib.StrlenFunc,
	"substr":      stdlib.SubstrFunc,
}

var GetPnameFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(Pname()), nil
	},
})

var GetVersionFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(VersionString()), nil
	},
})

var GetTempDirFunc = function.New(&function.Spec{
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

var GetExecutableFileFunc = function.New(&function.Spec{
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

var GetExecutableDirFunc = function.New(&function.Spec{
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

var GetUserHomeDirFunc = function.New(&function.Spec{
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

var GetUserConfigDirFunc = function.New(&function.Spec{
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

var GetPrefDirFunc = function.New(&function.Spec{
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

var GetArgLenFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return gocty.ToCtyValue(len(os.Args), cty.Number)
	},
})

var GetArgFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "argi",
			Type:             cty.Number,
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
		var i int
		err := gocty.FromCtyValue(args[0], &i)
		if err != nil {
			return cty.NilVal, err
		}
		a := make([]string, 0)
		for i, v := range os.Args {
			if i != 0 && strings.HasPrefix(v, "-") {
				continue
			}
			a = append(a, v)
		}
		if i < 0 || i >= len(a) {
			return args[1], nil
		}
		return cty.StringVal(a[i]), nil
	},
})

var GetArg2Func = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "argi",
			Type:             cty.Number,
			AllowDynamicType: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		var i int
		err := gocty.FromCtyValue(args[0], &i)
		if err != nil {
			return cty.NilVal, err
		}
		a := make([]string, 0)
		for i, v := range os.Args {
			if i != 0 && strings.HasPrefix(v, "-") {
				continue
			}
			a = append(a, v)
		}
		if i < 0 || i >= len(a) {
			return cty.NilVal, err
		}
		return cty.StringVal(a[i]), nil
	},
})

var GetEnv2Func = function.New(&function.Spec{
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

var GetEnvFunc = function.New(&function.Spec{
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

var GetFlag2Func = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "flag",
			Type:             cty.String,
			AllowDynamicType: true,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		in := args[0].AsString()
		out := ""
		for i, arg := range os.Args {
			if arg == in {
				if i < len(os.Args)-1 {
					out = os.Args[i+1]
				}
				return cty.StringVal(out), nil
			} else if strings.HasPrefix(arg, in+"=") {
				if len(arg) >= len(in)+1 {
					out = arg[len(in)+1:]
					return cty.StringVal(out), nil
				}
			}
		}
		return cty.NilVal, fmt.Errorf("required flag %s missing", in)
	},
})

var GetFlagFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name:             "flag",
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
		out := ""
		if !args[1].IsNull() {
			out = args[1].AsString()
		}
		for i, arg := range os.Args {
			if arg == in {
				if i < len(os.Args)-1 {
					out = os.Args[i+1]
				}
				break
			} else if strings.HasPrefix(arg, in+"=") {
				if len(arg) >= len(in)+1 {
					out = arg[len(in)+1:]
					break
				}
			}
		}
		return cty.StringVal(out), nil
	},
})
