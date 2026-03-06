package system

import (
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
)

//go:embed child_process.js
var child_process_js []byte

//go:embed events.js
var events_js []byte

//go:embed fs.js
var fs_js []byte

//go:embed path.js
var path_js []byte

//go:embed process.js
var process_js []byte

//go:embed string_decoder.js
var string_decoder_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"child_process.js":  child_process_js,
		"events.js":         events_js,
		"fs.js":             fs_js,
		"path.js":           path_js,
		"process.js":        process_js,
		"string_decoder.js": string_decoder_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/system")
	o := module.Get("exports").(*goja.Object)
	// l = new m.Log("name")
	o.Set("Log", new_log(rt))
	// m.free_os_memory()
	o.Set("free_os_memory", free_os_memory)
	// m.gc()
	o.Set("gc", gc)
	// m.now()
	o.Set("now", now(rt))
	// m.parseTime(value, "ms") e.g. s, ms, us, ns
	// m.parseTime(value, "2006-01-02 15:04:05")
	// m.parseTime(value, "RFC3339")
	// m.parseTime(value, "2006-01-02 15:04:05", "Asia/Shanghai")
	o.Set("parseTime", parseTime(rt))
	// t = m.time(sec, nanoSec)
	o.Set("time", timeTime(rt))
	// m.location("Asia/Shanghai")
	// m.location("UTC")
	o.Set("location", timeLocation(rt))
	// m.statz("1m", ...keys)
	o.Set("statz", statz(rt))
}

func new_log(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
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

func (log *jshLog) Logger(lvl logging.Level) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = fmt.Sprintf("%v", a.Export())
		}
		log.l.Logf(lvl, "%s", strings.Join(args, " "))
		return goja.Undefined()
	}
}

func free_os_memory() goja.Value {
	debug.FreeOSMemory()
	return goja.Undefined()
}

func gc() goja.Value {
	runtime.GC()
	return goja.Undefined()
}

func now(rt *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return rt.ToValue(time.Now())
	}
}

func timeTime(rt *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 2 {
			var sec, nanoSec int64
			rt.ExportTo(call.Arguments[0], &sec)
			rt.ExportTo(call.Arguments[0], &nanoSec)
			return rt.ToValue(time.Unix(int64(sec), 0)).(*goja.Object)
		} else if len(call.Arguments) == 1 {
			var sec int64
			rt.ExportTo(call.Arguments[0], &sec)
			return rt.ToValue(time.Unix(int64(sec), 0)).(*goja.Object)
		} else {
			return rt.ToValue(time.Unix(0, 0)).(*goja.Object)
		}
	}
}

func timeLocation(rt *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
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

func parseTime(rt *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
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

func statz(rt *goja.Runtime) func(samplingInterval string, keyFilters ...string) goja.Value {
	return func(samplingInterval string, keyFilters ...string) goja.Value {
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
