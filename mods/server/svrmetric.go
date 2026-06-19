package server

import (
	"context"
	"errors"
	"strings"

	"github.com/machbase/neo-client/api"
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
