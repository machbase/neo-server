package server

import (
	"context"
	"errors"

	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	api.StartMetrics()
	api.AddMetricsFunc(collectSysStatz)
	api.AddMetricsFunc(collectMachSvrStatz)
	api.AddMetricsFunc(collectMqttStatz(s))

	util.AddShutdownHook(func() { stopServerMetrics() })

	api.SetMetricsDestTable(s.Config.StatzOut)
}

func stopServerMetrics() {
	api.StopMetrics()
}

func collectSysStatz(g *metric.Gather) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := api.Default().Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		statzLog.Error("failed to connect to machbase: %v", err)
		return err
	}
	defer conn.Close()
	row := conn.QueryRow(ctx, "select sum(usage) from v$sysmem")
	if err = row.Err(); err != nil {
		statzLog.Error("failed to query machbase: %v", err)
		return err
	}
	var usageTotal int64
	if err = row.Scan(&usageTotal); err != nil {
		statzLog.Error("failed to scan machbase: %v", err)
		return err
	}

	g.Add("sys:sysmem", float64(usageTotal), metric.GaugeType(metric.UnitBytes))
	return nil
}

func collectMachSvrStatz(g *metric.Gather) error {
	nfo := mach.Stat()
	g.Add("machsvr:conn", float64(nfo.EngConn), metric.GaugeType(metric.UnitShort))
	g.Add("machsvr:stmt", float64(nfo.EngStmt), metric.GaugeType(metric.UnitShort))
	g.Add("machsvr:append", float64(nfo.EngAppend), metric.GaugeType(metric.UnitShort))
	return nil
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
