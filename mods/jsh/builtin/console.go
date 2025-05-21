package builtin

import (
	"fmt"
	"strings"

	js "github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/logging"
)

func EnableConsole(vm *js.Runtime, logger LogFunc) error {
	con := vm.NewObject()
	con.Set("log", console_log(vm, logging.LevelInfo, logger))
	con.Set("debug", console_log(vm, logging.LevelDebug, logger))
	con.Set("info", console_log(vm, logging.LevelInfo, logger))
	con.Set("warn", console_log(vm, logging.LevelWarn, logger))
	con.Set("error", console_log(vm, logging.LevelError, logger))
	vm.Set("console", con)
	return nil
}

type LogFunc func(logging.Level, ...any) error

func console_log(vm *js.Runtime, level logging.Level, logger LogFunc) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		var args []any

		if len(call.Arguments) == 0 {
			args = []any{""}
		} else {
			args = make([]any, len(call.Arguments))
			for i := 0; i < len(call.Arguments); i++ {
				args[i] = valueToLogMessage(call.Arguments[i])
			}
		}
		if err := logger(level, args...); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return js.Undefined()
	}
}

func valueToLogMessage(value js.Value) any {
	val := value.Export()
	if obj, ok := val.(*js.Object); ok {
		toString, ok := js.AssertFunction(obj.Get("toString"))
		if ok {
			ret, _ := toString(obj)
			return ret
		}
	}

	if m, ok := val.(map[string]any); ok {
		f := []string{}
		for k, v := range m {
			f = append(f, fmt.Sprintf("%s:%v", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(f, ", "))
	}
	if a, ok := val.([]any); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([]float64); ok {
		f := []string{}
		for _, v := range a {
			f = append(f, fmt.Sprintf("%v", v))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	if a, ok := val.([][]float64); ok {
		f := []string{}
		for _, vv := range a {
			fv := []string{}
			for _, v := range vv {
				fv = append(fv, fmt.Sprintf("%v", v))
			}
			f = append(f, fmt.Sprintf("[%s]", strings.Join(fv, ", ")))
		}
		return fmt.Sprintf("[%s]", strings.Join(f, ", "))
	}
	return fmt.Sprintf("%v", val)
}
