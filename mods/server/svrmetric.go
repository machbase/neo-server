package server

import (
	"context"
	"errors"

	"github.com/OutOfBedlam/metric"
	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/jemalloc"
)

var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	api.StartMetrics(s.Config.StatzOut)
	api.AddMetricsFunc(collectSysStatz)
	api.AddMetricsFunc(collectMachSvrStatz)
	api.AddMetricsFunc(collectMqttStatz(s))

	util.AddShutdownHook(func() { stopServerMetrics() })
}

func stopServerMetrics() {
	api.StopMetrics()
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
	m.AddField(metric.Field{Name: "sysmem", Value: float64(usageTotal), Unit: metric.UnitBytes, Type: metric.FieldTypeGauge})

	if jemalloc.Enabled {
		stat := &jemalloc.Stat{}
		jemalloc.HeapStat(stat)
		m.AddField(metric.Field{Name: "jemalloc_active", Value: float64(stat.Active), Unit: metric.UnitBytes, Type: metric.FieldTypeGauge})
	}

	return m, nil
}

func collectMachSvrStatz() (metric.Measurement, error) {
	m := metric.Measurement{Name: "machsvr"}
	nfo := mach.Stat()
	m.AddField(metric.Field{Name: "conn", Value: float64(nfo.EngConn), Unit: metric.UnitShort, Type: metric.FieldTypeGauge})
	m.AddField(metric.Field{Name: "stmt", Value: float64(nfo.EngStmt), Unit: metric.UnitShort, Type: metric.FieldTypeGauge})
	m.AddField(metric.Field{Name: "append", Value: float64(nfo.EngAppend), Unit: metric.UnitShort, Type: metric.FieldTypeGauge})
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
			metric.Field{Name: "recv_bytes", Value: float64(nfo.BytesReceived), Unit: metric.UnitBytes, Type: metric.FieldTypeGauge},
			metric.Field{Name: "send_bytes", Value: float64(nfo.BytesSent), Unit: metric.UnitBytes, Type: metric.FieldTypeGauge},
			metric.Field{Name: "recv_msgs", Value: float64(nfo.MessagesReceived), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "send_msgs", Value: float64(nfo.MessagesSent), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "drop_msgs", Value: float64(nfo.MessagesDropped), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "send_pkts", Value: float64(nfo.PacketsSent), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "recv_pkts", Value: float64(nfo.PacketsReceived), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "retained", Value: float64(nfo.Retained), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "subscriptions", Value: float64(nfo.Subscriptions), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "clients", Value: float64(nfo.ClientsTotal), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "clients_connected", Value: float64(nfo.ClientsConnected), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "clients_disconnected", Value: float64(nfo.ClientsDisconnected), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "inflight", Value: float64(nfo.Inflight), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
			metric.Field{Name: "inflight_dropped", Value: float64(nfo.InflightDropped), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		)
		return m, nil
	}
}
