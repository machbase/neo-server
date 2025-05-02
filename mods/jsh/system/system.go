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
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/system")
		o := module.Get("exports").(*js.Object)
		// m.free_os_memory()
		o.Set("free_os_memory", free_os_memory)
		// m.gc()
		o.Set("gc", gc)
		// m.now()
		o.Set("now", now(ctx, rt))
		// m.parseTime(value)
		o.Set("parseTime", parseTime(ctx, rt))
		// m.statz("1m", ...keys)
		o.Set("statz", statz(ctx, rt))
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

func parseTime(_ context.Context, rt *js.Runtime) func(value js.Value) js.Value {
	return func(value js.Value) js.Value {
		if t, ok := value.Export().(time.Time); ok {
			return rt.ToValue(t)
		}
		if t, ok := value.Export().(string); ok {
			if t, err := time.Parse(time.RFC3339, t); err == nil {
				return rt.ToValue(t)
			}
			if t, err := time.Parse(time.RFC3339Nano, t); err == nil {
				return rt.ToValue(t)
			}
		}
		if t, ok := value.Export().(int64); ok {
			return rt.ToValue(time.Unix(0, t*int64(time.Millisecond)))
		}
		if t, ok := value.Export().(float64); ok {
			return rt.ToValue(time.Unix(0, int64(t*float64(time.Millisecond))))
		}
		panic(rt.ToValue(fmt.Sprintf("parseTime: invalid time value %q", value.ExportType())))
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
