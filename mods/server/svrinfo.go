package server

import (
	"os"
	"runtime"
	"time"

	"github.com/machbase/neo-server/api/machsvr"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/mods"
)

var maxProcessors int32
var pid int32
var ver *mods.Version

func (s *svr) getServerInfo() (*mgmt.ServerInfoResponse, error) {
	if maxProcessors == 0 {
		maxProcessors = int32(runtime.GOMAXPROCS(-1))
	}
	if ver == nil {
		ver = mods.GetVersion()
	}
	if pid == 0 {
		pid = int32(os.Getpid())
	}

	rsp := &mgmt.ServerInfoResponse{
		Version: &mgmt.Version{
			Engine:         machsvr.LinkInfo(),
			Major:          int32(ver.Major),
			Minor:          int32(ver.Minor),
			Patch:          int32(ver.Patch),
			GitSHA:         ver.GitSHA,
			BuildTimestamp: mods.BuildTimestamp(),
			BuildCompiler:  mods.BuildCompiler(),
		},
		Runtime: &mgmt.Runtime{
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			Pid:            pid,
			UptimeInSecond: int64(time.Since(s.startupTime).Seconds()),
			Processes:      maxProcessors,
			Goroutines:     int32(runtime.NumGoroutine()),
		},
	}

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	rsp.Runtime.Mem = map[string]uint64{
		"sys":               mem.Sys,
		"alloc":             mem.Alloc,
		"total_alloc":       mem.TotalAlloc,
		"lookups":           mem.Lookups,
		"mallocs":           mem.Mallocs,
		"frees":             mem.Frees,
		"lives":             mem.Mallocs - mem.Frees,
		"heap_alloc":        mem.HeapAlloc,
		"heap_sys":          mem.HeapSys,
		"heap_idle":         mem.HeapIdle,
		"heap_in_use":       mem.HeapInuse,
		"heap_released":     mem.HeapReleased,
		"heap_objects":      mem.HeapObjects,
		"stack_in_use":      mem.StackInuse,
		"stack_sys":         mem.StackSys,
		"mspan_in_use":      mem.MSpanInuse,
		"mspan_sys":         mem.MSpanSys,
		"mcache_in_use":     mem.MCacheInuse,
		"mcache_sys":        mem.MCacheSys,
		"buck_hash_sys":     mem.BuckHashSys,
		"gc_sys":            mem.GCSys,
		"other_sys":         mem.OtherSys,
		"gc_next":           mem.NextGC,
		"gc_last":           mem.LastGC,
		"gc_pause_total_ns": mem.PauseTotalNs,
	}
	return rsp, nil
}

type SessionWatcher interface {
	ListWatcher(cb func(*machsvr.ConnState) bool)
}

var _ SessionWatcher = &machsvr.Database{}

func (s *svr) ServerSessions(reqStatz, reqSessions bool) (statz *mgmt.Statz, sessions []*mgmt.Session, err error) {
	if reqStatz {
		if st := machsvr.StatzSnapshot(); st != nil {
			statz = &mgmt.Statz{
				Conns:          st.Conns,
				ConnsInUse:     st.ConnsInUse,
				Stmts:          st.Stmts,
				StmtsInUse:     st.StmtsInUse,
				Appenders:      st.Appenders,
				AppendersInUse: st.AppendersInUse,
				RawConns:       st.RawConns,
			}
		}
	}
	if reqSessions {
		sessions = []*mgmt.Session{}
		s.db.ListWatcher(func(st *machsvr.ConnState) bool {
			sessions = append(sessions, &mgmt.Session{
				Id:            st.Id,
				CreTime:       st.CreatedTime.UnixNano(),
				LatestSqlTime: st.LatestTime.UnixNano(),
				LatestSql:     st.LatestSql,
			})
			return true
		})
	}
	return
}

func (s *svr) MqttInfo() map[string]any {
	if s.mqttd == nil {
		return nil
	}
	return s.mqttd.Statz()
}
