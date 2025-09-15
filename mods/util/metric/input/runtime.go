package input

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type Runtime struct {
}

func (ri *Runtime) Gather(g *metric.Gather) error {
	ms := runtime.MemStats{}
	runtime.ReadMemStats(&ms)
	g.Add("runtime:goroutines", float64(runtime.NumGoroutine()), metric.GaugeType(metric.UnitShort))
	g.Add("runtime:heap_inuse", float64(ms.HeapInuse), metric.GaugeType(metric.UnitBytes))
	g.Add("runtime:cgo_call", float64(runtime.NumCgoCall()), metric.OdometerType(metric.UnitShort))

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

type Netstat struct {
}

func (ni *Netstat) Gather(g *metric.Gather) error {
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
