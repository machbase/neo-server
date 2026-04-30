package system

import (
	"context"
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
)

//go:embed system.js
var system_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"system.js": system_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/system")
	o := module.Get("exports").(*goja.Object)
	// m.free_os_memory()
	o.Set("free_os_memory", free_os_memory)
	// m.gc()
	o.Set("gc", gc)
	// m.now()
	o.Set("now", now)
	// m.location("Asia/Shanghai")
	// m.location("UTC")
	o.Set("timeLocation", timeLocation)
	// m.statz("1m", ...keys)
	o.Set("statz", statz)
}

func free_os_memory() goja.Value {
	debug.FreeOSMemory()
	return goja.Undefined()
}

func gc() goja.Value {
	runtime.GC()
	return goja.Undefined()
}

func timeLocation(name string) (*time.Location, error) {
	if name == "" {
		return time.Local, nil
	} else if strings.EqualFold(name, "UTC") {
		return time.UTC, nil
	} else if strings.EqualFold(name, "Local") {
		return time.Local, nil
	}
	tz, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	return tz, nil
}

func now() time.Time {
	return time.Now()
}

func statz(samplingInterval string, keyFilters ...string) ([]map[string]any, error) {
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
		return nil, stat.Err
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
			if i >= len(keyFilters) {
				break
			}
			m[strings.ReplaceAll(keyFilters[i], ":", "_")] = v
		}
		ret = append(ret, m)
	}
	return ret, nil
}
