package system

import (
	"context"
	_ "embed"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dop251/goja"
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
