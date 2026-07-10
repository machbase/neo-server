package server

import (
	"context"
	"database/sql"
	"errors"
	"expvar"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/jsh/viz"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/machbase/neo-server/v8/mods/util/metric/input"
	"github.com/machbase/neo-server/v8/spi"
)

var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	spi.StartMetrics()
	spi.AddInput(&input.Runtime{})
	spi.AddInput(&input.Netstat{})
	spi.AddInputFunc(collectSysStatz)
	spi.AddInputFunc(collectDefaultPoolStatz)
	spi.AddInputFunc(collectMqttStatz(s))
	spi.AddInputFunc(collectTqlCacheStatz)

	util.AddShutdownHook(func() { stopServerMetrics() })

	spi.SetMetricsDestTable(s.Config.StatzOut)
}

func stopServerMetrics() {
	spi.StopMetrics()
}

func collectSysStatz(g *metric.Gather) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := spi.Default().Connect(ctx, api.WithAuthKey("sys", spi.DefaultKey()))
	if err != nil {
		statzLog.Error("failed to connect to machbase: %v", err)
		return err
	}
	defer conn.Close()
	if value, err := queryRowInt64(ctx, conn, "select sum(usage) from v$sysmem"); err != nil {
		return err
	} else {
		g.Add("sys:sysmem", float64(value), metric.GaugeType(metric.UnitBytes))
	}
	if value, err := queryRowInt64(ctx, conn, "select value from v$sysstat where name=?", "APPEND_OPEN"); err != nil {
		return err
	} else {
		g.Add("sys:append:open", float64(value), metric.OdometerType(metric.UnitShort))
	}
	if value, err := queryRowInt64(ctx, conn, "select value from v$sysstat where name=?", "APPEND_CLOSE"); err != nil {
		return err
	} else {
		g.Add("sys:append:close", float64(value), metric.OdometerType(metric.UnitShort))
	}
	if value, err := queryRowInt64(ctx, conn, "select value from v$sysstat where name=?", "APPEND_DATA_SUCCESS"); err != nil {
		return err
	} else {
		g.Add("sys:append:data:success", float64(value), metric.OdometerType(metric.UnitShort))
	}
	if value, err := queryRowInt64(ctx, conn, "select value from v$sysstat where name=?", "APPEND_DATA_FAILURE"); err != nil {
		return err
	} else {
		g.Add("sys:append:data:failure", float64(value), metric.OdometerType(metric.UnitShort))
	}
	if value, err := queryRowInt64(ctx, conn, "select count(*) from v$session"); err != nil {
		return err
	} else {
		g.Add("sys:session:count", float64(value), metric.GaugeType(metric.UnitShort))
	}
	if err := addExecuteStatz(ctx, conn, g); err != nil {
		return err
	}
	if err := addRollupGapMetric(ctx, conn, g); err != nil {
		return err
	}
	return nil
}

func collectDefaultPoolStatz(g *metric.Gather) error {
	db, err := spi.DefaultPool()
	if err != nil {
		statzLog.Trace("default pool is unavailable: %v", err)
		return nil
	}
	addDefaultPoolStatz(g, db.Stats())
	return nil
}

func addDefaultPoolStatz(g *metric.Gather, stat sql.DBStats) {
	g.Add("sys:pool:max_open", float64(stat.MaxOpenConnections), metric.GaugeType(metric.UnitShort))
	g.Add("sys:pool:open", float64(stat.OpenConnections), metric.GaugeType(metric.UnitShort))
	g.Add("sys:pool:in_use", float64(stat.InUse), metric.GaugeType(metric.UnitShort))
	g.Add("sys:pool:idle", float64(stat.Idle), metric.GaugeType(metric.UnitShort))
	g.Add("sys:pool:wait_count", float64(stat.WaitCount), metric.OdometerType(metric.UnitShort))
	g.Add("sys:pool:wait_duration", float64(stat.WaitDuration.Nanoseconds()), metric.OdometerType(metric.UnitDuration))
	g.Add("sys:pool:max_idle_closed", float64(stat.MaxIdleClosed), metric.OdometerType(metric.UnitShort))
	g.Add("sys:pool:max_idletime_closed", float64(stat.MaxIdleTimeClosed), metric.OdometerType(metric.UnitShort))
	g.Add("sys:pool:max_lifetime_closed", float64(stat.MaxLifetimeClosed), metric.OdometerType(metric.UnitShort))
}

func addExecuteStatz(ctx context.Context, conn api.Conn, g *metric.Gather) error {
	var count, min, max, avg int64
	row := conn.QueryRow(ctx, "select count, min_msec, max_msec, avg_msec from v$systime where name=?", "EXECUTE")
	if err := row.Err(); err != nil {
		statzLog.Error("failed to query machbase: %v", err)
		return err
	}
	if err := row.Scan(&count, &min, &max, &avg); err != nil {
		statzLog.Error("failed to scan machbase: %v", err)
		return err
	}
	g.Add("sys:execute:count", float64(count), metric.OdometerType(metric.UnitShort))
	g.Add("sys:execute:time:min", float64(min*1000000), metric.GaugeType(metric.UnitDuration))
	g.Add("sys:execute:time:max", float64(max*1000000), metric.GaugeType(metric.UnitDuration))
	g.Add("sys:execute:time:avg", float64(avg*1000000), metric.GaugeType(metric.UnitDuration))
	return nil
}

func addRollupGapMetric(ctx context.Context, conn api.Conn, g *metric.Gather) error {
	const sqlRollupGap = `SELECT
    R.ROLLUP_TABLE NAME,
    SUM(S.TABLE_END_RID - R.END_RID) GAP,
    MAX(R.LAST_ELAPSED_MSEC) MSEC
FROM
    M$SYS_TABLES T,
    V$ROLLUP R,
    V$STORAGE_TAG_TABLES S
WHERE
    S.TABLE_END_RID <> 0
AND S.ID = T.ID
AND R.SOURCE_TABLE = T.NAME
GROUP BY NAME
ORDER BY NAME`

	var totalGap, count int64
	var maxMsec float64
	var msec float64
	var name string
	var rollups int
	rows, err := conn.Query(ctx, sqlRollupGap)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&name, &count, &msec); err != nil {
			statzLog.Error("failed to scan rollup gap: %v", err)
			continue
		}
		name = strings.ToLower(name)

		g.Add("sys:rollup:"+name+":gap", float64(count), metric.GaugeType(metric.UnitShort))
		g.Add("sys:rollup:"+name+":last_elapse", float64(msec*1000), metric.GaugeType(metric.UnitDuration))
		if msec > maxMsec {
			maxMsec = msec
		}
		totalGap += count
		rollups++
	}
	// only when there are rollup tables, add global rollup metrics
	if rollups > 0 {
		g.Add("sys:rollup_global:gap", float64(totalGap), metric.GaugeType(metric.UnitShort))
		g.Add("sys:rollup_global:last_elapse", float64(maxMsec*1000), metric.GaugeType(metric.UnitDuration))
	}
	return nil
}

func queryRowInt64(ctx context.Context, conn api.Conn, sqlText string, params ...any) (int64, error) {
	var result int64
	row := conn.QueryRow(ctx, sqlText, params...)
	if err := row.Err(); err != nil {
		statzLog.Error("failed to query machbase: %v", err)
		return 0, err
	}
	if err := row.Scan(&result); err != nil {
		statzLog.Error("failed to scan machbase: %v", err)
		return 0, err
	}
	return result, nil
}

func collectMqttStatz(s *Server) func(g *metric.Gather) error {
	return func(g *metric.Gather) error {
		if s.mqttd == nil || s.mqttd.broker == nil {
			return errors.New("MQTT broker is not initialized")
		}
		nfo := s.mqttd.broker.Info
		g.Add("mqtt:recv_bytes", float64(nfo.BytesReceived), metric.GaugeType(metric.UnitBytes))
		g.Add("mqtt:send_bytes", float64(nfo.BytesSent), metric.GaugeType(metric.UnitBytes))
		g.Add("mqtt:recv_msgs", float64(nfo.MessagesReceived), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:send_msgs", float64(nfo.MessagesSent), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:drop_msgs", float64(nfo.MessagesDropped), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:send_pkts", float64(nfo.PacketsSent), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:recv_pkts", float64(nfo.PacketsReceived), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:retained", float64(nfo.Retained), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:subscriptions", float64(nfo.Subscriptions), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:clients", float64(nfo.ClientsTotal), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:clients_connected", float64(nfo.ClientsConnected), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:clients_disconnected", float64(nfo.ClientsDisconnected), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:inflight", float64(nfo.Inflight), metric.GaugeType(metric.UnitShort))
		g.Add("mqtt:inflight_dropped", float64(nfo.InflightDropped), metric.GaugeType(metric.UnitShort))
		return nil
	}
}

func collectTqlCacheStatz(g *metric.Gather) error {
	stat := tql.StatCache()
	g.Add("tql:cache:evictions", float64(stat.Evictions), metric.GaugeType(metric.UnitShort))
	g.Add("tql:cache:insertions", float64(stat.Insertions), metric.GaugeType(metric.UnitShort))
	g.Add("tql:cache:hits", float64(stat.Hits), metric.GaugeType(metric.UnitShort))
	g.Add("tql:cache:misses", float64(stat.Misses), metric.GaugeType(metric.UnitShort))
	g.Add("tql:cache:items", float64(stat.Items), metric.GaugeType(metric.UnitShort))
	return nil
}

var maxProcessors int32
var pid int32
var ver *mods.Version

func (s *Server) getServerInfoMap() map[string]any {
	nfo := map[string]any{}
	if sinfo, err := s.getServerInfo(); err != nil {
		nfo["error"] = err.Error()
	} else {
		if v := sinfo.Version; v != nil {
			nfo["version.major"] = v.Major
			nfo["version.minor"] = v.Minor
			nfo["version.patch"] = v.Patch
			nfo["version.gitSHA"] = v.GitSHA
			nfo["version.buildTimestamp"] = v.BuildTimestamp
			nfo["version.buildCompiler"] = v.BuildCompiler
			nfo["version.engine"] = v.Engine
		}
		if r := sinfo.Runtime; r != nil {
			nfo["runtime.os"] = r.OS
			nfo["runtime.arch"] = r.Arch
			nfo["runtime.pid"] = r.Pid
			nfo["runtime.uptimeInSecond"] = r.UptimeInSecond
			nfo["runtime.processes"] = r.Processes
			nfo["runtime.goroutines"] = r.Goroutines
			if r.Mem != nil {
				for k, v := range r.Mem {
					nfo[fmt.Sprintf("runtime.mem.%s", k)] = v
				}
			}
		}
	}
	return nfo
}

// getServerInfo returns runtime and version information.
//
// params:
//
// return: server information payload
func (s *Server) getServerInfo() (*ServerInfoResponse, error) {
	if maxProcessors == 0 {
		maxProcessors = int32(runtime.GOMAXPROCS(-1))
	}
	if ver == nil {
		ver = mods.GetVersion()
	}
	if pid == 0 {
		pid = int32(os.Getpid())
	}

	rsp := &ServerInfoResponse{
		Version: &Version{
			Engine:         mach.LinkInfo(),
			Major:          int32(ver.Major),
			Minor:          int32(ver.Minor),
			Patch:          int32(ver.Patch),
			GitSHA:         ver.GitSHA,
			BuildTimestamp: mods.BuildTimestamp(),
			BuildCompiler:  mods.BuildCompiler(),
		},
		Runtime: &Runtime{
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

type ServerStatzResponse struct {
	Statz []ServerStatz `json:"statz"`
}

type ServerStatz struct {
	Name string    `json:"name"`
	Spec *viz.Spec `json:"spec"`
}

// statzViz builds visualization specifications for metric names.
//
// params:
//   - names: metric names
//
// return: visualization specifications grouped by name
func statzViz(names []string) (*ServerStatzResponse, error) {
	v := spi.Visualizer()
	ret := &ServerStatzResponse{}
	for i, name := range names {
		var metricNames []string
		var fieldNames []string
		if strings.Contains(name, "#") {
			// support metric with tag, e.g. "cpu_usage#host=server1"
			parts := strings.SplitN(name, "#", 2)
			if len(parts) != 2 {
				continue
			}
			metricNames = append(metricNames, parts[0])
			fieldNames = append(fieldNames, parts[1])
		} else {
			metricNames = append(metricNames, name)
		}
		c := metric.Chart{ID: fmt.Sprintf("chart_%d", i), MetricNames: metricNames, FieldNames: fieldNames}
		v.AddChart(c)
	}
	for i := range names {
		spec, err := v.Generate(fmt.Sprintf("chart_%d", i), 0)
		if err != nil {
			return nil, err
		}
		ret.Statz = append(ret.Statz, ServerStatz{Name: names[i], Spec: spec})
	}
	return ret, nil
}

// statzKeys lists available metric keys with optional patterns.
//
// params:
//   - pattern: wildcard filters for metric keys
//
// return: sorted metric key names
func statzKeys(pattern []string) []string {
	prefix := spi.MetricsPrefix()
	if len(prefix) > 0 {
		prefix = prefix + ":"
	}
	if len(pattern) == 0 {
		pattern = []string{prefix + "*"}
	} else {
		for i, p := range pattern {
			pattern[i] = prefix + p
		}
	}
	filter := spi.QueryStatzFilter(pattern...)
	keys := make([]string, 0)
	expvar.Do(func(kv expvar.KeyValue) {
		if ok, _ := filter(kv.Key); !ok {
			return
		}
		if !strings.HasPrefix(kv.Key, prefix) {
			return
		}
		keys = append(keys, strings.TrimPrefix(kv.Key, prefix))
	})
	slices.Sort(keys)
	return keys
}

type StatzQueryResult struct {
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	Rows    [][]any  `json:"rows"`
}

// statzQuery queries metric time-series rows.
//
// params:
//   - maxRows: maximum row count
//   - pattern: wildcard filters for metric keys
//
// return: tabular metric query result
func statzQuery(maxRows int, pattern []string) (*StatzQueryResult, error) {
	prefix := spi.MetricsPrefix()
	if len(prefix) > 0 {
		prefix = prefix + ":"
	}
	for i, p := range pattern {
		pattern[i] = prefix + strings.ToLower(p)
	}

	statz := spi.QueryStatzRows(1*time.Minute, maxRows, spi.QueryStatzFilter(pattern...))
	if statz.Err != nil {
		return nil, statz.Err
	}

	ret := &StatzQueryResult{
		Columns: make([]string, len(statz.Cols)+1),
		Types:   make([]string, len(statz.Cols)+1),
		Rows:    make([][]any, len(statz.Rows)),
	}
	ret.Columns[0] = "time"
	ret.Types[0] = "datetime"
	for i, c := range statz.Cols {
		ret.Columns[i+1] = strings.TrimPrefix(c.Name, prefix)
		if statz.ValueTypes[i] == "i" {
			ret.Types[i+1] = "int64"
		} else {
			ret.Types[i+1] = c.Type.String()
		}
	}
	for i, r := range statz.Rows {
		ret.Rows[i] = make([]any, 0, len(r.Values)+1)
		ret.Rows[i] = append(ret.Rows[i], r.Timestamp.Format(time.RFC3339))
		ret.Rows[i] = append(ret.Rows[i], r.Values...)
	}
	return ret, nil
}
