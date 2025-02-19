package server

import (
	"context"
	"expvar"
	"runtime"
	"sync"
	"time"

	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/mochi-mqtt/server/v2/system"
)

var (
	metricGoHeapInUse = metric.NewExpVarIntGauge("go:heap_inuse", api.MetricTimeFrames...)
	metricGoRoutines  = metric.NewExpVarIntGauge("go:goroutine", api.MetricTimeFrames...)
	metricCGoCall     = metric.NewExpVarIntGauge("go:cgo_call", api.MetricTimeFrames...)

	metricMqttRecvBytes           = metric.NewExpVarIntCounter("machbase:mqtt:recv_bytes", api.MetricTimeFrames...)
	metricMqttSendBytes           = metric.NewExpVarIntCounter("machbase:mqtt:send_bytes", api.MetricTimeFrames...)
	metricMqttRecvMsgs            = metric.NewExpVarIntCounter("machbase:mqtt:recv_msgs", api.MetricTimeFrames...)
	metricMqttSendMsgs            = metric.NewExpVarIntCounter("machbase:mqtt:send_msgs", api.MetricTimeFrames...)
	metricMqttDropMsgs            = metric.NewExpVarIntCounter("machbase:mqtt:drop_msgs", api.MetricTimeFrames...)
	metricMqttSendPkts            = metric.NewExpVarIntCounter("machbase:mqtt:send_pkts", api.MetricTimeFrames...)
	metricMqttRecvPkts            = metric.NewExpVarIntCounter("machbase:mqtt:recv_pkts", api.MetricTimeFrames...)
	metricMqttRetained            = metric.NewExpVarIntCounter("machbase:mqtt:retained", api.MetricTimeFrames...)
	metricMqttSubscriptions       = metric.NewExpVarIntCounter("machbase:mqtt:subscriptions", api.MetricTimeFrames...)
	metricMqttClients             = metric.NewExpVarIntCounter("machbase:mqtt:clients", api.MetricTimeFrames...)
	metricMqttClientsConnected    = metric.NewExpVarIntCounter("machbase:mqtt:clients_connected", api.MetricTimeFrames...)
	metricMqttClientsDisconnected = metric.NewExpVarIntCounter("machbase:mqtt:clients_disconnected", api.MetricTimeFrames...)
	metricMqttInflight            = metric.NewExpVarIntCounter("machbase:mqtt:inflight", api.MetricTimeFrames...)
	metricMqttInflightDropped     = metric.NewExpVarIntCounter("machbase:mqtt:inflight_dropped", api.MetricTimeFrames...)

	metricMachConn   = metric.NewExpVarIntGauge("machbase:machsvr:conn", api.MetricTimeFrames...)
	metricMachStmt   = metric.NewExpVarIntGauge("machbase:machsvr:stmt", api.MetricTimeFrames...)
	metricMachAppend = metric.NewExpVarIntGauge("machbase:machsvr:append", api.MetricTimeFrames...)
)

var svrMqttInfo system.Info
var svrCloseStatz chan struct{}
var svrWgStatz sync.WaitGroup
var statzLog = logging.GetLog("server-statz")

func startServerMetrics(s *Server) {
	svrCloseStatz = make(chan struct{})
	svrWgStatz.Add(1)
	go func() {
		defer svrWgStatz.Done()
		for {
			select {
			case <-time.Tick(time.Second):
				doMqttStatz(s)
				doSysMemStatz(s)
				doMachSvrStatz(s)
			case <-svrCloseStatz:
				return
			}
		}
	}()

}

func stopServerMetrics() {
	close(svrCloseStatz)
	svrWgStatz.Wait()
}

func doMqttStatz(s *Server) {
	if s.mqttd == nil || s.mqttd.broker == nil {
		return
	}
	nfo := s.mqttd.broker.Info
	metricMqttRecvBytes.Add(nfo.BytesReceived - svrMqttInfo.BytesReceived)
	metricMqttSendBytes.Add(nfo.BytesSent - svrMqttInfo.BytesSent)
	metricMqttRecvMsgs.Add(nfo.MessagesReceived - svrMqttInfo.MessagesReceived)
	metricMqttSendMsgs.Add(nfo.MessagesSent - svrMqttInfo.MessagesSent)
	metricMqttDropMsgs.Add(nfo.MessagesDropped - svrMqttInfo.MessagesDropped)
	metricMqttSendPkts.Add(nfo.PacketsSent - svrMqttInfo.PacketsSent)
	metricMqttRecvPkts.Add(nfo.PacketsReceived - svrMqttInfo.PacketsReceived)
	metricMqttRetained.Add(nfo.Retained - svrMqttInfo.Retained)
	metricMqttSubscriptions.Add(nfo.Subscriptions - svrMqttInfo.Subscriptions)
	metricMqttInflight.Add(nfo.Inflight - svrMqttInfo.Inflight)
	metricMqttInflightDropped.Add(nfo.InflightDropped - svrMqttInfo.InflightDropped)
	metricMqttClients.Add(nfo.ClientsTotal - svrMqttInfo.ClientsTotal)
	metricMqttClientsConnected.Add(nfo.ClientsConnected - svrMqttInfo.ClientsConnected)
	metricMqttClientsDisconnected.Add(nfo.ClientsDisconnected - svrMqttInfo.ClientsDisconnected)
	metricGoHeapInUse.Add(int64(nfo.MemoryAlloc))
	metricGoRoutines.Add(int64(nfo.Threads))
	metricCGoCall.Add(runtime.NumCgoCall())
	svrMqttInfo = *nfo
}

func doMachSvrStatz(_ *Server) {
	nfo := mach.Stat()
	metricMachConn.Add(nfo.EngConn)
	metricMachStmt.Add(nfo.EngStmt)
	metricMachAppend.Add(nfo.EngAppend)
}

func doSysMemStatz(s *Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		statzLog.Error("failed to connect to machbase: %v", err)
		return
	}
	defer conn.Close()
	rows, err := conn.Query(ctx, "select name, usage from v$sysmem")
	if err != nil {
		statzLog.Error("failed to query machbase: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var usage int64
		if err := rows.Scan(&name, &usage); err != nil {
			statzLog.Error("failed to scan machbase: %v", err)
			return
		}
		var key = "machbase:sysmem:" + name
		var value *metric.ExpVarMetric[int64]
		if met := expvar.Get(key); met != nil {
			if g, ok := met.(*metric.ExpVarMetric[int64]); ok {
				value = g
			} else {
				statzLog.Error("failed to get metric: %s %T", key, g)
				return
			}
		} else {
			value = metric.NewExpVarIntGauge(key, api.MetricTimeFrames...)
		}
		value.Add(usage)
	}
}
