package server

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"

	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/jemalloc"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	api.StartMetrics()
	api.AddMetricsFunc(collectSysStatz)
	api.AddMetricsFunc(collectMachSvrStatz)
	api.AddMetricsFunc(collectMqttStatz(s))
	api.AddMetricsInput(&RuntimeInput{})
	api.AddMetricsFunc(collectPsStatz)
	api.AddMetricsFunc(collectNetstatz)

	util.AddShutdownHook(func() { stopServerMetrics() })

	api.SetMetricsDestTable(s.Config.StatzOut)
}

func stopServerMetrics() {
	api.StopMetrics()
}

type RuntimeInput struct {
}

func (ri *RuntimeInput) Init() error { return nil }
func (ri *RuntimeInput) Gather(g *metric.Gather) error {
	ms := runtime.MemStats{}
	runtime.ReadMemStats(&ms)
	g.Add("runtime:goroutines", float64(runtime.NumGoroutine()), metric.GaugeType(metric.UnitShort))
	g.Add("runtime:heap_inuse", float64(ms.HeapInuse), metric.GaugeType(metric.UnitBytes))
	g.Add("runtime:cgo_call", float64(runtime.NumCgoCall()), metric.OdometerType(metric.UnitShort))
	return nil
}

func collectPsStatz(g *metric.Gather) error {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return fmt.Errorf("failed to collect CPU percent: %w", err)
	}
	g.Add("ps:cpu_percent", cpuPercent[0], metric.GaugeType(metric.UnitPercent))

	memStat, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("failed to collect memory percent: %w", err)
	}
	g.Add("ps:mem_percent", memStat.UsedPercent, metric.GaugeType(metric.UnitPercent))
	return nil
}

func collectNetstatz(g *metric.Gather) error {
	stat, err := net.Connections("all")
	if err != nil {
		return fmt.Errorf("failed to collect netstat: %w", err)
	}

	counts := make(map[string]int)
	for _, cs := range stat {
		if cs.Type == syscall.SOCK_DGRAM {
			continue
		}
		c, ok := counts[cs.Status]
		if !ok {
			counts[cs.Status] = 0
		}
		counts[cs.Status] = c + 1
	}

	gaugeType := metric.GaugeType(metric.UnitShort)
	for kind, name := range netStatzList {
		value, ok := counts[kind]
		if !ok {
			value = 0
		}
		val := float64(value)
		g.Add("netstat:"+name, val, gaugeType)
	}
	return nil
}

var netStatzList = map[string]string{
	"ESTABLISHED": "tcp_established",
	// "SYN_SENT":    "tcp_syn_sent",
	// "SYN_RECV":    "tcp_syn_recv",
	"FIN_WAIT1": "tcp_fin_wait1",
	"FIN_WAIT2": "tcp_fin_wait2",
	"TIME_WAIT": "tcp_time_wait",
	// "CLOSE":       "tcp_close",
	"CLOSE_WAIT": "tcp_close_wait",
	// "LAST_ACK":    "tcp_last_ack",
	"LISTEN": "tcp_listen",
	// "CLOSING": "tcp_closing",
	// "NONE":    "tcp_none",
	// "UDP":     "udp_socket",
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
	if jemalloc.Enabled {
		stat := &jemalloc.Stat{}
		jemalloc.HeapStat(stat)
		g.Add("sys:jemalloc_active", float64(stat.Active), metric.GaugeType(metric.UnitBytes))
	}
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
