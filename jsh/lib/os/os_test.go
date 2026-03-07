package os_test

import (
	"runtime"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestOSBasicFunctions(t *testing.T) {
	osType := func() string {
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
	}()

	tests := []test_engine.TestCase{
		{
			Name: "arch",
			Script: `
				const os = require('os');
				const arch = os.arch();
				console.println('arch:', arch);
				console.println('is string:', typeof arch === 'string');
			`,
			Output: []string{
				"arch: " + runtime.GOARCH,
				"is string: true",
			},
		},
		{
			Name: "platform",
			Script: `
				const os = require('os');
				const platform = os.platform();
				console.println('platform:', platform);
				console.println('is string:', typeof platform === 'string');
			`,
			Output: []string{
				"platform: " + runtime.GOOS,
				"is string: true",
			},
		},
		{
			Name: "type",
			Script: `
				const os = require('os');
				const type = os.type();
				console.println('type:', type);
				console.println('is string:', typeof type === 'string');
			`,
			Output: []string{
				"type: " + osType,
				"is string: true",
			},
		},
		{
			Name: "release",
			Script: `
				const os = require('os');
				const release = os.release();
				console.println('has release:', release.length > 0);
				console.println('is string:', typeof release === 'string');
			`,
			Output: []string{
				"has release: true",
				"is string: true",
			},
		},
		{
			Name: "version",
			Script: `
				const os = require('os');
				const version = os.version();
				console.println('has version:', version.length > 0);
				console.println('is string:', typeof version === 'string');
			`,
			Output: []string{
				"has version: true",
				"is string: true",
			},
		},
		{
			Name: "hostname",
			Script: `
				const os = require('os');
				const hostname = os.hostname();
				console.println('has hostname:', hostname.length > 0);
				console.println('is string:', typeof hostname === 'string');
			`,
			Output: []string{
				"has hostname: true",
				"is string: true",
			},
		},
		{
			Name: "homedir",
			Script: `
				const os = require('os');
				const homedir = os.homedir();
				console.println('has homedir:', homedir.length > 0);
				console.println('is string:', typeof homedir === 'string');
			`,
			Output: []string{
				"has homedir: true",
				"is string: true",
			},
		},
		{
			Name: "tmpdir",
			Script: `
				const os = require('os');
				const tmpdir = os.tmpdir();
				console.println('has tmpdir:', tmpdir.length > 0);
				console.println('is string:', typeof tmpdir === 'string');
			`,
			Output: []string{
				"has tmpdir: true",
				"is string: true",
			},
		},
		{
			Name: "endianness",
			Script: `
				const os = require('os');
				const endian = os.endianness();
				console.println('is valid:', endian === 'BE' || endian === 'LE');
				console.println('is string:', typeof endian === 'string');
			`,
			Output: []string{
				"is valid: true",
				"is string: true",
			},
		},
		{
			Name: "memory",
			Script: `
				const os = require('os');
				const total = os.totalmem();
				const free = os.freemem();
				console.println('has total:', total > 0);
				console.println('has free:', free >= 0);
				console.println('free <= total:', free <= total);
			`,
			Output: []string{
				"has total: true",
				"has free: true",
				"free <= total: true",
			},
		},
		{
			Name: "uptime",
			Script: `
				const os = require('os');
				const uptime = os.uptime();
				console.println('has uptime:', uptime >= 0);
				console.println('is number:', typeof uptime === 'number');
			`,
			Output: []string{
				"has uptime: true",
				"is number: true",
			},
		},
		{
			Name: "bootTime",
			Script: `
				const os = require('os');
				const bootTime = os.bootTime();
				console.println('has bootTime:', bootTime >= 0);
				console.println('is number:', typeof bootTime === 'number');
			`,
			Output: []string{
				"has bootTime: true",
				"is number: true",
			},
		},
		{
			Name: "cpus",
			Script: `
				const os = require('os');
				const cpus = os.cpus();
				console.println('is array:', Array.isArray(cpus));
				console.println('has cpus:', cpus.length > 0);
				if (cpus.length > 0) {
					console.println('has model:', typeof cpus[0].model === 'string');
					console.println('has speed:', typeof cpus[0].speed === 'number');
					console.println('has times:', typeof cpus[0].times === 'object');
				}
			`,
			Output: []string{
				"is array: true",
				"has cpus: true",
				"has model: true",
				"has speed: true",
				"has times: true",
			},
		},
		{
			Name: "cpuCounts",
			Script: `
				const os = require('os');
				console.println("cpu logical:", os.cpuCounts(true) > 0);
				console.println("cpu physical:", os.cpuCounts(false) > 0);
			`,
			Output: []string{
				"cpu logical: true",
				"cpu physical: true",
			},
		},
		{
			Name: "cpuPercent",
			Script: `
				const os = require('os');
				const percent = os.cpuPercent(0, true)
				console.println('percent is array:', Array.isArray(percent));
				console.println('cpu[0] is number:', typeof percent[0] === 'number');
			`,
			Output: []string{
				"percent is array: true",
				"cpu[0] is number: true",
			},
		},
		{
			Name: "loadavg",
			Script: `
				const os = require('os');
				const loadavg = os.loadavg();
				console.println('is array:', Array.isArray(loadavg));
				console.println('has 3 elements:', loadavg.length === 3);
				console.println('all numbers:', loadavg.every(v => typeof v === 'number'));
			`,
			Output: []string{
				"is array: true",
				"has 3 elements: true",
				"all numbers: true",
			},
		},
		{
			Name: "hostInfo",
			Script: `
				const os = require('os');
				const info = os.hostInfo();
				console.println('is object:', typeof info === 'object');
				console.println('has hostname:', typeof info.hostname === 'string');
				console.println('has uptime:', typeof info.uptime === 'number');
				console.println('has bootTime:', typeof info.bootTime === 'number');
				console.println('has procs:', typeof info.procs === 'number');
				console.println('has os:', typeof info.os === 'string');
				console.println('has platform:', typeof info.platform === 'string');
				console.println('has platformFamily:', typeof info.platformFamily === 'string');
				console.println('has platformVersion:', typeof info.platformVersion === 'string');
				console.println('has kernelVersion:', typeof info.kernelVersion === 'string');
				console.println('has kernelArch:', typeof info.kernelArch === 'string');
				console.println('has hostId:', typeof info.hostId === 'string');
			`,
			Output: []string{
				"is object: true",
				"has hostname: true",
				"has uptime: true",
				"has bootTime: true",
				"has procs: true",
				"has os: true",
				"has platform: true",
				"has platformFamily: true",
				"has platformVersion: true",
				"has kernelVersion: true",
				"has kernelArch: true",
				"has hostId: true",
			},
		},
		{
			Name: "userInfo",
			Script: `
				const os = require('os');
				const user = os.userInfo();
				console.println('is object:', typeof user === 'object');
				console.println('has username:', typeof user.username === 'string');
				console.println('has homedir:', typeof user.homedir === 'string');
				console.println('has shell:', typeof user.shell === 'string');
			`,
			Output: []string{
				"is object: true",
				"has username: true",
				"has homedir: true",
				"has shell: true",
			},
		},
		{
			Name: "diskPartitions",
			Script: `
				const os = require('os');
				const parts = os.diskPartitions();
				console.println('is array:', Array.isArray(parts));
				console.println('has partitions:', parts.length >= 0);
				if (parts.length > 0) {
					console.println('has device:', typeof parts[0].device === 'string');
					console.println('has mountpoint:', typeof parts[0].mountpoint === 'string');
					console.println('has fstype:', typeof parts[0].fstype === 'string');
					console.println('has opts:', Array.isArray(parts[0].opts));
				}
			`,
			Output: []string{
				"is array: true",
				"has partitions: true",
				"has device: true",
				"has mountpoint: true",
				"has fstype: true",
				"has opts: true",
			},
		},
		{
			Name: "diskUsage",
			Script: `
				const os = require('os');
				const parts = os.diskPartitions();
				if (parts.length > 0) {
					const usage = os.diskUsage(parts[0].mountpoint);
					console.println('is object:', typeof usage === 'object');
					console.println('has total:', typeof usage.total === 'number');
					console.println('has used:', typeof usage.used === 'number');
					console.println('has free:', typeof usage.free === 'number');
					console.println('has usedPercent:', typeof usage.usedPercent === 'number');
				} else {
					console.println('no partitions available for diskUsage test');
				}
			`,
			Output: []string{
				"is object: true",
				"has total: true",
				"has used: true",
				"has free: true",
				"has usedPercent: true",
			},
		},
		{
			Name: "networkInterfaces",
			Script: `
				const os = require('os');
				const interfaces = os.networkInterfaces();
				console.println('is object:', typeof interfaces === 'object');
				const keys = Object.keys(interfaces);
				console.println('has interfaces:', keys.length >= 0);
				if (keys.length > 0) {
					const firstIface = interfaces[keys[0]];
					console.println('interfaces is array:', Array.isArray(firstIface));
					if (firstIface.length > 0) {
						console.println('has address:', typeof firstIface[0].address === 'string');
						console.println('has family:', typeof firstIface[0].family === 'string');
					}
				}
			`,
			Output: []string{
				"is object: true",
				"has interfaces: true",
				"interfaces is array: true",
				"has address: true",
				"has family: true",
			},
		},
		{
			Name: "EOL",
			Script: `
				const os = require('os');
				const eol = os.EOL;
				console.println('is string:', typeof eol === 'string');
				console.println('is valid:', eol === '\n' || eol === '\r\n');
			`,
			Output: []string{
				"is string: true",
				"is valid: true",
			},
		},
		{
			Name: "constants",
			Script: `
				const os = require('os');
				const c = os.constants;
				console.println('has constants:', typeof c === 'object');
				console.println('has signals:', typeof c.signals === 'object');
				console.println('has priority:', typeof c.priority === 'object');
				console.println('has SIGINT:', typeof c.signals.SIGINT === 'number');
				console.println('has PRIORITY_NORMAL:', typeof c.priority.PRIORITY_NORMAL === 'number');
			`,
			Output: []string{
				"has constants: true",
				"has signals: true",
				"has priority: true",
				"has SIGINT: true",
				"has PRIORITY_NORMAL: true",
			},
		},
		{
			Name: "destructuring",
			Script: `
				const { arch, platform, type, hostname, EOL, constants } = require('os');
				console.println('arch:', typeof arch === 'function');
				console.println('platform:', typeof platform === 'function');
				console.println('type:', typeof type === 'function');
				console.println('hostname:', typeof hostname === 'function');
				console.println('EOL:', typeof EOL === 'string');
				console.println('constants:', typeof constants === 'object');
			`,
			Output: []string{
				"arch: true",
				"platform: true",
				"type: true",
				"hostname: true",
				"EOL: true",
				"constants: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
