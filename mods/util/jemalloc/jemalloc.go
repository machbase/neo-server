//go:build linux && amd64 && debug
// +build linux,amd64,debug

package jemalloc

/*
// This cgo directive is what actually causes jemalloc to be linked in to the
// final Go executable
#cgo pkg-config: jemalloc
#include <jemalloc/jemalloc.h>
void _refresh_jemalloc_stats() {
	// You just need to pass something not-null into the "epoch" mallctl.
	size_t random_something = 1;
	mallctl("epoch", NULL, NULL, &random_something, sizeof(random_something));
}
size_t _get_jemalloc_active() {
	size_t stat, stat_size;
	stat = 0;
	stat_size = sizeof(stat);
	mallctl("stats.active", &stat, &stat_size, NULL, 0);
	return stat;
}
size_t _get_jemalloc_resident() {
	size_t stat, stat_size;
	stat = 0;
	stat_size = sizeof(stat);
	mallctl("stats.resident", &stat, &stat_size, NULL, 0);
	return stat;
}
*/
import "C"

import (
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

func init() {
	var refreshLock sync.Mutex

	metricActive := metric.NewExpVarIntGauge("go:jemalloc_active", api.MetricTimeFrames...)
	metricResident := metric.NewExpVarIntGauge("go:jemalloc_resident", api.MetricTimeFrames...)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshLock.Lock()
			C._refresh_jemalloc_stats()
			if st := C._get_jemalloc_active(); st > 0 {
				metricActive.Add(int64(st))
			}
			if st := C._get_jemalloc_resident(); st > 0 {
				metricResident.Add(int64(st))
			}
			refreshLock.Unlock()
		}
	}()
}
