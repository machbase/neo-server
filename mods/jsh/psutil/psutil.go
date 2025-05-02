package psutil

import (
	"context"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/system")
		o := module.Get("exports").(*js.Object)
		// psutil().hostBootTime() seconds
		o.Set("hostBootTime", func(call js.FunctionCall) js.Value {
			bootTime, err := host.BootTimeWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(bootTime)
		})
		// psutil().hostUptime() seconds
		o.Set("hostUptime", func(call js.FunctionCall) js.Value {
			uptime, err := host.UptimeWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(uptime)
		})
		// psutil().hostID()
		o.Set("hostID", func(call js.FunctionCall) js.Value {
			hostID, err := host.HostIDWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(hostID)
		})
		// psutil().hostInfo()
		o.Set("hostInfo", func(call js.FunctionCall) js.Value {
			stat, err := host.InfoWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(stat)
		})
		// psutil().cpuCounts(logical) int
		o.Set("cpuCounts", func(call js.FunctionCall) js.Value {
			var logical bool
			if len(call.Arguments) > 0 {
				logical = call.Argument(0).ToBoolean()
			}
			cpuCounts, err := cpu.Counts(logical)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(cpuCounts)
		})
		// psutil().cpuPercent(intervalSec, percpu) []float64
		o.Set("cpuPercent", func(call js.FunctionCall) js.Value {
			var interval time.Duration
			if len(call.Arguments) > 0 {
				interval = time.Duration(call.Argument(0).ToInteger()) * time.Second
			}
			percpu := false
			if len(call.Arguments) > 1 {
				percpu = call.Argument(1).ToBoolean()
			}
			cpuPercents, err := cpu.PercentWithContext(ctx, interval, percpu)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(cpuPercents)
		})
		// psutil().loadAvg() Object
		o.Set("loadAvg", func(call js.FunctionCall) js.Value {
			loadAvg, err := load.AvgWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(loadAvg)
		})
		// psutil().memVirtual() Object
		o.Set("memVirtual", func(call js.FunctionCall) js.Value {
			stat, err := mem.VirtualMemoryWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(stat)
		})
		// psutil().memSwap() Object
		o.Set("memSwap", func(call js.FunctionCall) js.Value {
			stat, err := mem.SwapMemoryWithContext(ctx)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(stat)
		})
		// psutil().diskPartitions(all) []disk.PartitionStat if all is false it returns only physical devices
		o.Set("diskPartitions", func(call js.FunctionCall) js.Value {
			all := false
			if len(call.Arguments) > 0 {
				all = call.Argument(0).ToBoolean()
			}
			partitions, err := disk.PartitionsWithContext(ctx, all)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(partitions)
		})
		// psutil().diskUsage(path) disk.UsageStat
		o.Set("diskUsage", func(call js.FunctionCall) js.Value {
			if len(call.Arguments) == 0 {
				panic(rt.ToValue("usage: missing argument"))
			}
			path := call.Argument(0).String()
			usage, err := disk.UsageWithContext(ctx, path)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(usage)
		})
		// psutil().diskIOCounters() []disk.IOCountersStat
		o.Set("diskIOCounters", func(call js.FunctionCall) js.Value {
			names := make([]string, len(call.Arguments))
			for i, arg := range call.Arguments {
				rt.ExportTo(arg, &names[i])
			}
			ioCounters, err := disk.IOCountersWithContext(ctx, names...)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(ioCounters)
		})
		// psutil().netIOCounters(pernic) []net.ConnectionStat
		o.Set("netIOCounters", func(call js.FunctionCall) js.Value {
			pernic := false
			if len(call.Arguments) > 0 {
				pernic = call.Argument(0).ToBoolean()
			}
			ioCounter, err := net.IOCountersWithContext(ctx, pernic)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(ioCounter)
		})
		// psutil().netProtoCounters() []net.ProtoCountersStat
		o.Set("netProtoCounters", func(call js.FunctionCall) js.Value {
			var protos []string
			if len(call.Arguments) > 0 {
				protos = make([]string, len(call.Arguments))
				for i, arg := range call.Arguments {
					rt.ExportTo(arg, &protos[i])
				}
			}
			protoCounters, err := net.ProtoCountersWithContext(ctx, protos)
			if err != nil {
				panic(rt.ToValue(err.Error()))
			}
			return rt.ToValue(protoCounters)
		})
	}
}
