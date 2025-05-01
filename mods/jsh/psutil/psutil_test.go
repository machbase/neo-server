package psutil_test

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

type TestCase struct {
	Name      string
	Script    string
	UseRegex  bool
	Expect    []string
	ExpectLog []string
}

func runTest(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/psutil"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}
	lines := bytes.Split(w.Bytes(), []byte{'\n'})
	for i, line := range lines {
		if i >= len(tc.Expect) {
			break
		}
		if tc.UseRegex {
			re, err := regexp.Compile(tc.Expect[i])
			if err != nil {
				t.Fatalf("Error compiling regex: %s", err)
			}
			if !re.Match(line) {
				t.Errorf("Expected regex %q, got %q", tc.Expect[i], line)
			}
		} else {
			if !bytes.Equal(line, []byte(tc.Expect[i])) {
				t.Errorf("Expected %q, got %q", tc.Expect[i], line)
			}
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestPSUtil_cpu(t *testing.T) {
	tests := []TestCase{
		{
			Name: "psutil-cpu",
			Script: `
				const {println} = require("@jsh/process");
				const psutil = require("@jsh/psutil")
				try {
					println("cpu count logical: " + psutil.cpuCounts(true));
					println("cpu count physical: " + psutil.cpuCounts(false));
					println("cpu percent: " + psutil.cpuPercent(0, false));
				} catch (e) {
				 	println(e.toString());
				}
			`,
			UseRegex: true,
			Expect: []string{
				"cpu count logical: [0-9]+",
				"cpu count physical: [0-9]+",
				"cpu percent: [0-9]+([.][0-9]+)?",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestPSUtil_load(t *testing.T) {
	tests := []TestCase{
		{
			Name: "psutil-load",
			Script: `
				const {println} = require("@jsh/process");
				const psutil = require("@jsh/psutil")
				try {
					println("load1: " + psutil.loadAvg().load1);
					println("load5: " + psutil.loadAvg().load5);
					println("load15: " + psutil.loadAvg().load15);
					println(psutil.loadAvg());
				} catch (e) {
				 	println(e.toString());
				}
			`,
			UseRegex: true,
			Expect: []string{
				"load1: [0-9]+([.][0-9]+)?",
				"load5: [0-9]+([.][0-9]+)?",
				"load15: [0-9]+([.][0-9]+)?",
				`{"load1":[0-9]+([.][0-9]+)?,"load5":[0-9]+([.][0-9]+)?,"load15":[0-9]+([.][0-9]+)?`,
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestPSUtil_mem(t *testing.T) {
	tests := []TestCase{
		{
			Name: "psutil-mem",
			Script: `
				const {println} = require("@jsh/process");
				const psutil = require("@jsh/psutil")
				try {
					println("virtual total: " + psutil.memVirtual().total);
					println("virtual available: " + psutil.memVirtual().available);
					println("virtual usedPercent: " + psutil.memVirtual().usedPercent);
					println("swap total: " + psutil.memSwap().total);
					println("swap free: " + psutil.memSwap().free);
				} catch (e) {
				 	println(e.toString());
				}
			`,
			UseRegex: true,
			Expect: []string{
				"virtual total: [0-9]+",
				"virtual available: [0-9]+",
				"virtual usedPercent: [0-9]+([.][0-9]+)?",
				"swap total: [0-9]+",
				"swap free: [0-9]+",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestPSUtil_disk(t *testing.T) {
	tests := []TestCase{
		{
			Name: "psutil-disk",
			Script: `
				const {println} = require("@jsh/process");
				const psutil = require("@jsh/psutil")
				try {
					println("disk partitions: " + psutil.diskPartitions().length);
					println("disk usage: " + psutil.diskUsage("/").usedPercent);
				} catch (e) {
				 	println(e.toString());
				}
			`,
			UseRegex: true,
			Expect: []string{
				"disk partitions: [0-9]+",
				"disk usage: [0-9]+([.][0-9]+)?",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestPSUtil_net(t *testing.T) {
	tests := []TestCase{
		{
			Name: "psutil-net",
			Script: `
				const {println} = require("@jsh/process");
				const psutil = require("@jsh/psutil")
				try {
					stat = psutil.netIOCounters()
					println("name:", stat[0].name);
					println("bytesSent:", stat[0].bytesSent);
					println("bytesRecv:", stat[0].bytesRecv);
					println("packetsSent:", stat[0].packetsSent);
					println("packetsRecv:", stat[0].packetsRecv);
					println("errin:", stat[0].errin);
					println("errout:", stat[0].errout);
					println("dropin:", stat[0].dropin);
					println("dropout:", stat[0].dropout);
					println("fifoin:", stat[0].fifoin);
					println("fifoout:", stat[0].fifoout);
				} catch (e) {
				 	println(e.toString());
				}
			`,
			UseRegex: true,
			Expect: []string{
				"name: all",
				"bytesSent: [0-9]+",
				"bytesRecv: [0-9]+",
				"packetsSent: [0-9]+",
				"packetsRecv: [0-9]+",
				"errin: [0-9]+",
				"errout: [0-9]+",
				"dropin: [0-9]+",
				"dropout: [0-9]+",
				"fifoin: [0-9]+",
				"fifoout: [0-9]+",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}
