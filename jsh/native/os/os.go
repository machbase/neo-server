package os

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"
	"unsafe"

	"github.com/dop251/goja"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	m := module.Get("exports").(*goja.Object)

	m.Set("arch", func() string { return arch() })
	m.Set("cpus", func() interface{} { return cpus() })
	m.Set("endianness", func() string { return endianness() })
	m.Set("freemem", func() uint64 { return freemem() })
	m.Set("homedir", func() string { return homedir() })
	m.Set("hostname", func() string { return hostname() })
	m.Set("loadavg", func() interface{} { return loadavg() })
	m.Set("platform", func() string { return platform() })
	m.Set("release", func() string { return release() })
	m.Set("tmpdir", func() string { return tmpdir() })
	m.Set("totalmem", func() uint64 { return totalmem() })
	m.Set("type", func() string { return ostype() })
	m.Set("uptime", func() int64 { return uptime() })
	m.Set("userInfo", func() interface{} { return userInfo(map[string]interface{}{}) })
	m.Set("networkInterfaces", func() interface{} { return networkInterfaces() })
	m.Set("EOL", eol())

	m.Set("bootTime", func() (uint64, error) { return host.BootTime() })
	m.Set("hostInfo", func(call goja.FunctionCall) interface{} { return hostInfo() })
	m.Set("cpuCounts", func(logical bool) (int, error) { return cpu.Counts(logical) })
	m.Set("cpuPercent", func(intervalSec int, perCPU bool) ([]float64, error) {
		return cpu.Percent(time.Duration(intervalSec)*time.Second, perCPU)
	})
	m.Set("diskPartitions", func(all bool) ([]disk.PartitionStat, error) {
		return disk.Partitions(all)
	})
	m.Set("diskUsage", func(path string) (*disk.UsageStat, error) {
		return disk.Usage(path)
	})
	m.Set("diskIOCounters", func(names ...string) (map[string]disk.IOCountersStat, error) {
		return disk.IOCounters(names...)
	})
	m.Set("netProtoCounters", func(proto []string) ([]net.ProtoCountersStat, error) {
		return net.ProtoCounters(proto)
	})

	constants := rt.NewObject()
	signals := rt.NewObject()
	signals.Set("SIGHUP", 1)
	signals.Set("SIGINT", 2)
	signals.Set("SIGQUIT", 3)
	signals.Set("SIGILL", 4)
	signals.Set("SIGTRAP", 5)
	signals.Set("SIGABRT", 6)
	signals.Set("SIGBUS", 7)
	signals.Set("SIGFPE", 8)
	signals.Set("SIGKILL", 9)
	signals.Set("SIGUSR1", 10)
	signals.Set("SIGSEGV", 11)
	signals.Set("SIGUSR2", 12)
	signals.Set("SIGPIPE", 13)
	signals.Set("SIGALRM", 14)
	signals.Set("SIGTERM", 15)
	constants.Set("signals", signals)

	priority := rt.NewObject()
	priority.Set("PRIORITY_LOW", 19)
	priority.Set("PRIORITY_BELOW_NORMAL", 10)
	priority.Set("PRIORITY_NORMAL", 0)
	priority.Set("PRIORITY_ABOVE_NORMAL", -7)
	priority.Set("PRIORITY_HIGH", -14)
	priority.Set("PRIORITY_HIGHEST", -20)
	constants.Set("priority", priority)

	m.Set("constants", constants)
}

func arch() string {
	return runtime.GOARCH
}

func cpus() interface{} {
	info, err := cpu.Info()
	if err != nil {
		return []interface{}{}
	}

	times, err := cpu.Times(true)
	if err != nil {
		times = []cpu.TimesStat{}
	}

	result := make([]map[string]interface{}, 0, len(info))
	for i, cpuInfo := range info {
		cpuMap := map[string]interface{}{
			"cpu":        cpuInfo.CPU,
			"vendor":     cpuInfo.VendorID,
			"family":     cpuInfo.Family,
			"model":      cpuInfo.Model,
			"stepping":   cpuInfo.Stepping,
			"physicalId": cpuInfo.PhysicalID,
			"coreId":     cpuInfo.CoreID,
			"cores":      cpuInfo.Cores,
			"modelName":  cpuInfo.ModelName,
			"speed":      cpuInfo.Mhz,
		}

		if i < len(times) {
			cpuMap["times"] = map[string]interface{}{
				"user": times[i].User * 1000,
				"nice": times[i].Nice * 1000,
				"sys":  times[i].System * 1000,
				"idle": times[i].Idle * 1000,
				"irq":  times[i].Irq * 1000,
			}
		} else {
			cpuMap["times"] = map[string]interface{}{
				"user": 0,
				"nice": 0,
				"sys":  0,
				"idle": 0,
				"irq":  0,
			}
		}

		result = append(result, cpuMap)
	}

	return result
}

func endianness() string {
	var i int32 = 0x01020304
	u := (*[4]byte)(unsafe.Pointer(&i))
	if u[0] == 0x01 {
		return "BE"
	}
	return "LE"
}

func freemem() uint64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return v.Available
}

func homedir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

func loadavg() interface{} {
	avg, err := load.Avg()
	if err != nil {
		return []float64{0, 0, 0}
	}
	return []float64{avg.Load1, avg.Load5, avg.Load15}
}

func platform() string {
	return runtime.GOOS
}

func release() string {
	info, err := host.Info()
	if err != nil {
		return ""
	}
	return info.KernelVersion
}

func tmpdir() string {
	return os.TempDir()
}

func totalmem() uint64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return v.Total
}

func ostype() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows_NT"
	default:
		return runtime.GOOS
	}
}

func uptime() int64 {
	info, err := host.Info()
	if err != nil {
		return 0
	}
	return int64(info.Uptime)
}

func userInfo(options map[string]interface{}) interface{} {
	// {encoding: 'utf8'} default.
	// If encoding is set to 'buffer', the username, shell, and homedir values will be Buffer instances.
	_ = options // currently unused,

	currentUser, err := user.Current()
	if err != nil {
		return map[string]interface{}{
			"uid":      -1,
			"gid":      -1,
			"username": "",
			"homedir":  "",
			"shell":    "",
		}
	}

	result := map[string]interface{}{
		"username": currentUser.Username,
		"homedir":  currentUser.HomeDir,
	}

	if currentUser.Uid != "" {
		var uid int
		fmt.Sscanf(currentUser.Uid, "%d", &uid)
		result["uid"] = uid
	} else {
		result["uid"] = -1
	}

	if currentUser.Gid != "" {
		var gid int
		fmt.Sscanf(currentUser.Gid, "%d", &gid)
		result["gid"] = gid
	} else {
		result["gid"] = -1
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = os.Getenv("ComSpec")
			if shell == "" {
				shell = "cmd.exe"
			}
		} else {
			shell = "/bin/sh"
		}
	}
	result["shell"] = shell

	return result
}

func hostInfo() interface{} {
	n, err := host.Info()
	if err != nil {
		return map[string]interface{}{}
	}

	result := map[string]interface{}{
		"hostname":             n.Hostname,
		"uptime":               n.Uptime,
		"bootTime":             n.BootTime,
		"procs":                n.Procs,                // number of processes
		"os":                   n.OS,                   // ex: freebsd, linux
		"platform":             n.Platform,             // ex: ubuntu
		"platformFamily":       n.PlatformFamily,       // ex: debian, rhel
		"platformVersion":      n.PlatformVersion,      // version of the complete OS
		"kernelVersion":        n.KernelVersion,        // version of the OS kernel (if available)
		"kernelArch":           n.KernelArch,           // native cpu architecture queried at runtime, as returned by `uname -m` or empty string in case of error
		"virtualizationSystem": n.VirtualizationSystem, //
		"virtualizationRole":   n.VirtualizationRole,   // guest or host
		"hostId":               n.HostID,               // ex: uuid
	}
	return result
}

func networkInterfaces() interface{} {
	interfaces, err := net.Interfaces()
	if err != nil {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{})

	for _, iface := range interfaces {
		addrs := make([]map[string]interface{}, 0)

		for _, addr := range iface.Addrs {
			addrInfo := map[string]interface{}{
				"address": addr.Addr,
			}

			if len(addr.Addr) > 0 {
				if containsColon(addr.Addr) {
					addrInfo["family"] = "IPv6"
					addrInfo["internal"] = false
				} else {
					addrInfo["family"] = "IPv4"
					addrInfo["internal"] = addr.Addr == "127.0.0.1" || addr.Addr == "::1"
				}
			}

			addrs = append(addrs, addrInfo)
		}

		if len(addrs) > 0 {
			result[iface.Name] = addrs
		}
	}

	return result
}

func eol() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
