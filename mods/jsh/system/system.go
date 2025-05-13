package system

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/system")
		o := module.Get("exports").(*js.Object)
		// l = new m.Log("name")
		o.Set("Log", new_log(ctx, rt))
		// m.free_os_memory()
		o.Set("free_os_memory", free_os_memory)
		// m.gc()
		o.Set("gc", gc)
		// m.now()
		o.Set("now", now(ctx, rt))
		// m.parseTime(value, "ms") e.g. s, ms, us, ns
		// m.parseTime(value, "2006-01-02 15:04:05")
		// m.parseTime(value, "RFC3339")
		// m.parseTime(value, "2006-01-02 15:04:05", "Asia/Shanghai")
		o.Set("parseTime", parseTime(ctx, rt))
		// t = m.time(sec, nanoSec)
		o.Set("time", timeTime(ctx, rt))
		// m.location("Asia/Shanghai")
		// m.location("UTC")
		o.Set("location", timeLocation(ctx, rt))
		// m.statz("1m", ...keys)
		o.Set("statz", statz(ctx, rt))
	}
}

func new_log(_ context.Context, rt *js.Runtime) func(call js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("missing arguments"))
		}
		var name string
		if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
			panic(rt.ToValue(fmt.Sprintf("log: invalid argument %s", err.Error())))
		}
		l := &jshLog{l: logging.GetLog(name)}
		ret := rt.NewObject()
		ret.Set("trace", l.Logger(logging.LevelTrace))
		ret.Set("debug", l.Logger(logging.LevelDebug))
		ret.Set("info", l.Logger(logging.LevelInfo))
		ret.Set("warn", l.Logger(logging.LevelWarn))
		ret.Set("error", l.Logger(logging.LevelError))
		return ret
	}
}

type jshLog struct {
	l logging.Log
}

func (log *jshLog) Logger(lvl logging.Level) func(call js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		args := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = fmt.Sprintf("%v", a.Export())
		}
		log.l.Logf(lvl, "%s", strings.Join(args, " "))
		return js.Undefined()
	}
}

func free_os_memory() js.Value {
	debug.FreeOSMemory()
	return js.Undefined()
}

func gc() js.Value {
	runtime.GC()
	return js.Undefined()
}

func now(_ context.Context, rt *js.Runtime) func(call js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		return rt.ToValue(time.Now())
	}
}

func timeTime(_ context.Context, rt *js.Runtime) func(call js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) >= 2 {
			var sec, nanoSec int64
			rt.ExportTo(call.Arguments[0], &sec)
			rt.ExportTo(call.Arguments[0], &nanoSec)
			return rt.ToValue(time.Unix(int64(sec), 0)).(*js.Object)
		} else if len(call.Arguments) == 1 {
			var sec int64
			rt.ExportTo(call.Arguments[0], &sec)
			return rt.ToValue(time.Unix(int64(sec), 0)).(*js.Object)
		} else {
			return rt.ToValue(time.Unix(0, 0)).(*js.Object)
		}
	}
}

func timeLocation(_ context.Context, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		name := "UTC"
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
				panic(rt.ToValue(fmt.Sprintf("location: invalid argument %s", err.Error())))
			}
		}
		loc, err := time.LoadLocation(name)
		if err != nil {
			panic(rt.ToValue(fmt.Sprintf("location: %s", err.Error())))
		}
		return rt.ToValue(loc)
	}
}

func parseTime(_ context.Context, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("parseTime: missing argument"))
		}
		value := call.Arguments[0]
		if t, ok := value.Export().(time.Time); ok {
			return rt.ToValue(t)
		}
		var format string
		if len(call.Arguments) > 1 {
			if err := rt.ExportTo(call.Arguments[1], &format); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
		}

		if format == "s" {
			var val int64
			if err := rt.ExportTo(value, &val); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
			return rt.ToValue(time.Unix(val, 0))
		} else if format == "ms" {
			var val int64
			if err := rt.ExportTo(value, &val); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
			return rt.ToValue(time.Unix(0, val*int64(time.Millisecond)))
		} else if format == "us" {
			var val int64
			if err := rt.ExportTo(value, &val); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
			return rt.ToValue(time.Unix(0, val*int64(time.Microsecond)))
		} else if format == "ns" {
			var val int64
			if err := rt.ExportTo(value, &val); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
			return rt.ToValue(time.Unix(0, val))
		} else {
			var val string
			if err := rt.ExportTo(value, &val); err != nil {
				panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
			}
			var location *time.Location = time.Local
			if len(call.Arguments) > 2 {
				if err := rt.ExportTo(call.Arguments[2], &location); err != nil {
					panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", err.Error())))
				}
			}
			format = util.GetTimeformat(format)
			if t, err := time.ParseInLocation(format, val, location); err == nil {
				return rt.ToValue(t)
			}
			if t, err := time.ParseInLocation(format, val, time.Local); err == nil {
				return rt.ToValue(t)
			}
		}
		panic(rt.ToValue(fmt.Sprintf("parseTime: invalid argument %s", value.String())))
	}
}

func statz(_ context.Context, rt *js.Runtime) func(samplingInterval string, keyFilters ...string) js.Value {
	return func(samplingInterval string, keyFilters ...string) js.Value {
		var interval = api.MetricShortTerm
		switch strings.ToLower(samplingInterval) {
		case "short":
			interval = api.MetricShortTerm
		case "mid":
			interval = api.MetricMidTerm
		case "long":
			interval = api.MetricLongTerm
		default:
			if dur, err := time.ParseDuration(samplingInterval); err == nil {
				interval = dur
			}
		}
		stat := api.QueryStatz(interval, api.QueryStatzFilter(keyFilters))
		if stat.Err != nil {
			panic(rt.ToValue(stat.Err.Error()))
		}
		ret := []map[string]any{}
		for _, row := range stat.Rows {
			m := map[string]any{
				"time":   row.Timestamp,
				"values": row.Values,
				"toString": func() string {
					return fmt.Sprintf("%s %s", row.Timestamp, row.Values)
				},
			}
			for i, v := range row.Values {
				m[strings.ReplaceAll(keyFilters[i], ":", "_")] = v
			}
			ret = append(ret, m)
		}
		return rt.ToValue(ret)
	}
}
