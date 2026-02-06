package log

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dop251/goja"
)

var defaultWriter io.Writer = io.Discard

func SetConsole(vm *goja.Runtime, w io.Writer) *goja.Object {
	defaultWriter = w

	con := vm.NewObject()
	con.Set("log", makeConsoleLog(slog.LevelInfo))
	con.Set("debug", makeConsoleLog(slog.LevelDebug))
	con.Set("info", makeConsoleLog(slog.LevelInfo))
	con.Set("warn", makeConsoleLog(slog.LevelWarn))
	con.Set("error", makeConsoleLog(slog.LevelError))
	con.Set("println", doPrintln)
	con.Set("print", doPrint)
	con.Set("printf", doPrintf)
	con.Set("writer", w) // expose writer for advanced usage, it used in pretty box.SetOutput()
	return con
}

func Println(args ...interface{}) {
	fmt.Fprintln(defaultWriter, args...)
}

func Print(args ...interface{}) {
	fmt.Fprint(defaultWriter, args...)
}

func Printf(format string, args ...interface{}) {
	fmt.Fprintf(defaultWriter, format, args...)
}

func Log(level slog.Level, args ...interface{}) {
	strLevel := level.String()
	strLevel = strLevel + strings.Repeat(" ", 5-len(strLevel))
	fmt.Fprintln(defaultWriter, strLevel, fmt.Sprint(args...))
}

func doPrint(call goja.FunctionCall) goja.Value {
	Print(argsValues(call)...)
	return goja.Undefined()
}

func doPrintln(call goja.FunctionCall) goja.Value {
	Println(argsValues(call)...)
	return goja.Undefined()
}

func doPrintf(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return goja.Undefined()
	}
	format := call.Arguments[0].String()
	args := make([]interface{}, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		args[i-1] = valueToPrintable(call.Arguments[i])
	}
	Printf(format, args...)
	return goja.Undefined()
}

func makeConsoleLog(level slog.Level) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		Log(level, argsValues(call)...)
		return goja.Undefined()
	}
}

func argsValues(call goja.FunctionCall) []interface{} {
	args := make([]interface{}, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = valueToPrintable(arg)
	}
	return args
}

func valueToPrintable(value goja.Value) any {
	val := value.Export()
	return anyToPrintable(val)
}

func anyToPrintable(val any) any {
	if val == nil {
		return "null"
	}
	switch val := val.(type) {
	default:
		return fmt.Sprintf("%v(%T)", val, val)
	case string:
		return val
	case *string:
		return *val
	case []string:
		return fmt.Sprintf("[%s]", strings.Join(val, ", "))
	case bool:
		return val
	case int:
		return val
	case *int:
		return *val
	case int32:
		return val
	case *int32:
		return *val
	case int64:
		return val
	case *int64:
		return *val
	case float64:
		return val
	case *float64:
		return *val
	case time.Time:
		return val.Local().Format(time.DateTime)
	case *time.Time:
		return val.Local().Format(time.DateTime)
	case time.Duration:
		return val.String()
	case *url.URL:
		return val.String()
	case []*url.URL:
		f := []string{}
		for _, u := range val {
			f = append(f, u.String())
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	case *[]*url.URL:
		return anyToPrintable(*val)
	case *goja.Object:
		toString, ok := goja.AssertFunction(val.Get("toString"))
		if ok {
			ret, _ := toString(val)
			return ret
		} else {
			return val.String()
		}
	case map[string]any:
		keys := []string{}
		for k := range val {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		f := []string{}
		for _, k := range keys {
			v := val[k]
			f = append(f, fmt.Sprintf("%s:%v", k, anyToPrintable(v)))
		}
		return fmt.Sprintf("{%s}", strings.Join(f, ", "))
	case []byte:
		return string(val)
	case []any:
		f := []string{}
		for _, v := range val {
			f = append(f, fmt.Sprintf("%v", anyToPrintable(v)))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	case []float64:
		f := []string{}
		for _, v := range val {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	case [][]float64:
		f := []string{}
		for _, vv := range val {
			fv := []string{}
			for _, v := range vv {
				fv = append(fv, fmt.Sprintf("%v", v))
			}
			f = append(f, fmt.Sprintf("[%s]", strings.Join(fv, ", ")))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
}
