package os

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestOSModule(t *testing.T) {
	rt := goja.New()

	// Create module object
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)

	// Initialize os module
	Module(rt, module)

	// Test that all expected functions are exported
	exportsObj := module.Get("exports").(*goja.Object)

	testCases := []string{
		"arch",
		"cpus",
		"endianness",
		"freemem",
		"homedir",
		"hostname",
		"loadavg",
		"networkInterfaces",
		"platform",
		"release",
		"tmpdir",
		"totalmem",
		"type",
		"uptime",
		"userInfo",
		"constants",
		"EOL",
	}

	for _, name := range testCases {
		if exportsObj.Get(name) == nil || goja.IsUndefined(exportsObj.Get(name)) {
			t.Errorf("Expected %s to be exported", name)
		}
	}
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab()},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/os", Module)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

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

	tests := []TestCase{
		{
			name: "arch",
			script: `
				const os = require('os');
				const arch = os.arch();
				console.println('arch:', arch);
				console.println('is string:', typeof arch === 'string');
			`,
			output: []string{
				"arch: " + runtime.GOARCH,
				"is string: true",
			},
		},
		{
			name: "platform",
			script: `
				const os = require('os');
				const platform = os.platform();
				console.println('platform:', platform);
				console.println('is string:', typeof platform === 'string');
			`,
			output: []string{
				"platform: " + runtime.GOOS,
				"is string: true",
			},
		},
		{
			name: "type",
			script: `
				const os = require('os');
				const type = os.type();
				console.println('type:', type);
				console.println('is string:', typeof type === 'string');
			`,
			output: []string{
				"type: " + osType,
				"is string: true",
			},
		},
		{
			name: "hostname",
			script: `
				const os = require('os');
				const hostname = os.hostname();
				console.println('has hostname:', hostname.length > 0);
				console.println('is string:', typeof hostname === 'string');
			`,
			output: []string{
				"has hostname: true",
				"is string: true",
			},
		},
		{
			name: "homedir",
			script: `
				const os = require('os');
				const homedir = os.homedir();
				console.println('has homedir:', homedir.length > 0);
				console.println('is string:', typeof homedir === 'string');
			`,
			output: []string{
				"has homedir: true",
				"is string: true",
			},
		},
		{
			name: "tmpdir",
			script: `
				const os = require('os');
				const tmpdir = os.tmpdir();
				console.println('has tmpdir:', tmpdir.length > 0);
				console.println('is string:', typeof tmpdir === 'string');
			`,
			output: []string{
				"has tmpdir: true",
				"is string: true",
			},
		},
		{
			name: "endianness",
			script: `
				const os = require('os');
				const endian = os.endianness();
				console.println('is valid:', endian === 'BE' || endian === 'LE');
				console.println('is string:', typeof endian === 'string');
			`,
			output: []string{
				"is valid: true",
				"is string: true",
			},
		},
		{
			name: "memory",
			script: `
				const os = require('os');
				const total = os.totalmem();
				const free = os.freemem();
				console.println('has total:', total > 0);
				console.println('has free:', free >= 0);
				console.println('free <= total:', free <= total);
			`,
			output: []string{
				"has total: true",
				"has free: true",
				"free <= total: true",
			},
		},
		{
			name: "uptime",
			script: `
				const os = require('os');
				const uptime = os.uptime();
				console.println('has uptime:', uptime >= 0);
				console.println('is number:', typeof uptime === 'number');
			`,
			output: []string{
				"has uptime: true",
				"is number: true",
			},
		},
		{
			name: "bootTime",
			script: `
				const os = require('os');
				const bootTime = os.bootTime();
				console.println('has bootTime:', bootTime >= 0);
				console.println('is number:', typeof bootTime === 'number');
			`,
			output: []string{
				"has bootTime: true",
				"is number: true",
			},
		},
		{
			name: "cpus",
			script: `
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
			output: []string{
				"is array: true",
				"has cpus: true",
				"has model: true",
				"has speed: true",
				"has times: true",
			},
		},
		{
			name: "cpuCounts",
			script: `
				const os = require('os');
				console.println("cpu logical:", os.cpuCounts(true) > 0);
				console.println("cpu physical:", os.cpuCounts(false) > 0);
			`,
			output: []string{
				"cpu logical: true",
				"cpu physical: true",
			},
		},
		{
			name: "cpuPercent",
			script: `
				const os = require('os');
				const percent = os.cpuPercent(0, true)
				console.println('percent is array:', Array.isArray(percent));
				console.println('cpu[0] is number:', typeof percent[0] === 'number');
			`,
			output: []string{
				"percent is array: true",
				"cpu[0] is number: true",
			},
		},
		{
			name: "loadavg",
			script: `
				const os = require('os');
				const loadavg = os.loadavg();
				console.println('is array:', Array.isArray(loadavg));
				console.println('has 3 elements:', loadavg.length === 3);
				console.println('all numbers:', loadavg.every(v => typeof v === 'number'));
			`,
			output: []string{
				"is array: true",
				"has 3 elements: true",
				"all numbers: true",
			},
		},
		{
			name: "hostInfo",
			script: `
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
			output: []string{
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
			name: "userInfo",
			script: `
				const os = require('os');
				const user = os.userInfo();
				console.println('is object:', typeof user === 'object');
				console.println('has username:', typeof user.username === 'string');
				console.println('has homedir:', typeof user.homedir === 'string');
				console.println('has shell:', typeof user.shell === 'string');
			`,
			output: []string{
				"is object: true",
				"has username: true",
				"has homedir: true",
				"has shell: true",
			},
		},
		{
			name: "diskPartitions",
			script: `
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
			output: []string{
				"is array: true",
				"has partitions: true",
				"has device: true",
				"has mountpoint: true",
				"has fstype: true",
				"has opts: true",
			},
		},
		{
			name: "diskUsage",
			script: `
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
			output: []string{
				"is object: true",
				"has total: true",
				"has used: true",
				"has free: true",
				"has usedPercent: true",
			},
		},
		{
			name: "networkInterfaces",
			script: `
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
			output: []string{
				"is object: true",
				"has interfaces: true",
				"interfaces is array: true",
				"has address: true",
				"has family: true",
			},
		},
		{
			name: "EOL",
			script: `
				const os = require('os');
				const eol = os.EOL;
				console.println('is string:', typeof eol === 'string');
				console.println('is valid:', eol === '\n' || eol === '\r\n');
			`,
			output: []string{
				"is string: true",
				"is valid: true",
			},
		},
		{
			name: "constants",
			script: `
				const os = require('os');
				const c = os.constants;
				console.println('has constants:', typeof c === 'object');
				console.println('has signals:', typeof c.signals === 'object');
				console.println('has priority:', typeof c.priority === 'object');
				console.println('has SIGINT:', typeof c.signals.SIGINT === 'number');
				console.println('has PRIORITY_NORMAL:', typeof c.priority.PRIORITY_NORMAL === 'number');
			`,
			output: []string{
				"has constants: true",
				"has signals: true",
				"has priority: true",
				"has SIGINT: true",
				"has PRIORITY_NORMAL: true",
			},
		},
		{
			name: "destructuring",
			script: `
				const { arch, platform, type, hostname, EOL, constants } = require('os');
				console.println('arch:', typeof arch === 'function');
				console.println('platform:', typeof platform === 'function');
				console.println('type:', typeof type === 'function');
				console.println('hostname:', typeof hostname === 'function');
				console.println('EOL:', typeof EOL === 'string');
				console.println('constants:', typeof constants === 'object');
			`,
			output: []string{
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
		RunTest(t, tc)
	}
}
