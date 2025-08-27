package server

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/OutOfBedlam/metric"
	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/jemalloc"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	api.StartMetrics()
	api.AddMetricsFunc(collectSysStatz)
	api.AddMetricsFunc(collectMachSvrStatz)
	api.AddMetricsFunc(collectMqttStatz(s))
	api.AddMetricsFunc(collectRuntime)
	api.AddMetricsFunc(collectPsStatz)

	util.AddShutdownHook(func() { stopServerMetrics() })

	api.SetMetricsDestTable(s.Config.StatzOut)
}

func stopServerMetrics() {
	api.StopMetrics()
}

func collectRuntime() (metric.Measurement, error) {
	ms := runtime.MemStats{}
	runtime.ReadMemStats(&ms)

	m := metric.Measurement{Name: "runtime"}
	m.AddField(
		metric.Field{Name: "goroutines", Value: float64(runtime.NumGoroutine()), Type: metric.GaugeType(metric.UnitShort)},
		metric.Field{Name: "heap_inuse", Value: float64(ms.HeapInuse), Type: metric.GaugeType(metric.UnitShort)},
		metric.Field{Name: "cgo_call", Value: float64(runtime.NumCgoCall()), Type: metric.GaugeType(metric.UnitShort)},
	)
	return m, nil
}

func collectPsStatz() (metric.Measurement, error) {
	m := metric.Measurement{Name: "ps"}

	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return m, fmt.Errorf("failed to collect CPU percent: %w", err)
	}
	m.Fields = append(m.Fields, metric.Field{
		Name:  "cpu_percent",
		Value: cpuPercent[0],
		Type:  metric.MeterType(metric.UnitPercent),
	})

	memStat, err := mem.VirtualMemory()
	if err != nil {
		return m, fmt.Errorf("failed to collect memory percent: %w", err)
	}
	m.Fields = append(m.Fields, metric.Field{
		Name:  "mem_percent",
		Value: memStat.UsedPercent,
		Type:  metric.MeterType(metric.UnitPercent),
	})
	return m, nil
}

func collectSysStatz() (metric.Measurement, error) {
	m := metric.Measurement{Name: "sys"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := api.Default().Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		statzLog.Error("failed to connect to machbase: %v", err)
		return m, err
	}
	defer conn.Close()
	row := conn.QueryRow(ctx, "select sum(usage) from v$sysmem")
	if err = row.Err(); err != nil {
		statzLog.Error("failed to query machbase: %v", err)
		return m, err
	}
	var usageTotal int64
	if err = row.Scan(&usageTotal); err != nil {
		statzLog.Error("failed to scan machbase: %v", err)
		return m, err
	}
	m.AddField(metric.Field{Name: "sysmem", Value: float64(usageTotal), Type: metric.GaugeType(metric.UnitBytes)})

	if jemalloc.Enabled {
		stat := &jemalloc.Stat{}
		jemalloc.HeapStat(stat)
		m.AddField(metric.Field{Name: "jemalloc_active", Value: float64(stat.Active), Type: metric.GaugeType(metric.UnitBytes)})
	}

	return m, nil
}

func collectMachSvrStatz() (metric.Measurement, error) {
	m := metric.Measurement{Name: "machsvr"}
	nfo := mach.Stat()
	m.AddField(metric.Field{Name: "conn", Value: float64(nfo.EngConn), Type: metric.GaugeType(metric.UnitShort)})
	m.AddField(metric.Field{Name: "stmt", Value: float64(nfo.EngStmt), Type: metric.GaugeType(metric.UnitShort)})
	m.AddField(metric.Field{Name: "append", Value: float64(nfo.EngAppend), Type: metric.GaugeType(metric.UnitShort)})
	return m, nil
}

func collectMqttStatz(s *Server) func() (metric.Measurement, error) {
	return func() (metric.Measurement, error) {
		m := metric.Measurement{Name: "mqtt"}
		if s.mqttd == nil || s.mqttd.broker == nil {
			return m, errors.New("MQTT broker is not initialized")
		}
		nfo := s.mqttd.broker.Info
		m.AddField(
			metric.Field{Name: "recv_bytes", Value: float64(nfo.BytesReceived), Type: metric.GaugeType(metric.UnitBytes)},
			metric.Field{Name: "send_bytes", Value: float64(nfo.BytesSent), Type: metric.GaugeType(metric.UnitBytes)},
			metric.Field{Name: "recv_msgs", Value: float64(nfo.MessagesReceived), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "send_msgs", Value: float64(nfo.MessagesSent), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "drop_msgs", Value: float64(nfo.MessagesDropped), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "send_pkts", Value: float64(nfo.PacketsSent), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "recv_pkts", Value: float64(nfo.PacketsReceived), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "retained", Value: float64(nfo.Retained), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "subscriptions", Value: float64(nfo.Subscriptions), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "clients", Value: float64(nfo.ClientsTotal), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "clients_connected", Value: float64(nfo.ClientsConnected), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "clients_disconnected", Value: float64(nfo.ClientsDisconnected), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "inflight", Value: float64(nfo.Inflight), Type: metric.GaugeType(metric.UnitShort)},
			metric.Field{Name: "inflight_dropped", Value: float64(nfo.InflightDropped), Type: metric.GaugeType(metric.UnitShort)},
		)
		return m, nil
	}
}
