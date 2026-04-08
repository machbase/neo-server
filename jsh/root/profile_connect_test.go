package root_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentProfileConnectMergesOverrideObject(t *testing.T) {
	workDir := t.TempDir()

	script := strings.Join([]string{
		"const machcli = require('machcli');",
		"let captured = null;",
		"machcli.Client = function(cfg) {",
		"    captured = cfg;",
		"    this.connect = function() { return { close: function() {} }; };",
		"    this.close = function() {};",
		"};",
		"const agent = require('repl/profiles/agent');",
		"agent.db.connect({ password: 'test', user: 'demo' });",
		"console.println(JSON.stringify(captured));",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent profile script failed: %v\n%s", err, output)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &cfg); err != nil {
		t.Fatalf("parse config JSON: %v\n%s", err, output)
	}
	if cfg["password"] != "test" {
		t.Fatalf("password = %v, want test", cfg["password"])
	}
	if cfg["user"] != "demo" {
		t.Fatalf("user = %v, want demo", cfg["user"])
	}
	if cfg["host"] != "127.0.0.1" {
		t.Fatalf("host = %v, want 127.0.0.1", cfg["host"])
	}
}

func TestUserProfileConnectMergesOverrideObjectOnActiveConfigPath(t *testing.T) {
	workDir := t.TempDir()

	script := strings.Join([]string{
		"const fs = require('fs');",
		"const machcli = require('machcli');",
		"let captured = [];",
		"machcli.Client = function(cfg) {",
		"    captured.push(cfg);",
		"    this.connect = function() { return { close: function() {} }; };",
		"    this.close = function() {};",
		"};",
		"fs.writeFileSync('/work/db.json', JSON.stringify({ host: '10.0.0.9', port: 6001, user: 'sys', password: 'manager' }));",
		"const user = require('repl/profiles/user');",
		"user.db.connect('/work/db.json');",
		"user.db.disconnect();",
		"user.db.connect({ password: 'test' });",
		"console.println(JSON.stringify(captured[1]));",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("user profile script failed: %v\n%s", err, output)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &cfg); err != nil {
		t.Fatalf("parse config JSON: %v\n%s", err, output)
	}
	if cfg["host"] != "10.0.0.9" {
		t.Fatalf("host = %v, want 10.0.0.9", cfg["host"])
	}
	if cfg["port"] != float64(6001) {
		t.Fatalf("port = %v, want 6001", cfg["port"])
	}
	if cfg["password"] != "test" {
		t.Fatalf("password = %v, want test", cfg["password"])
	}
}

func TestAgentProfileVizLinesReturnsRenderEnvelope(t *testing.T) {
	workDir := t.TempDir()

	script := strings.Join([]string{
		"const advn = require('vizspec');",
		"const agent = require('repl/profiles/agent');",
		"const spec = advn.createSpec({",
		"  series: [{",
		"    id: 'cpu',",
		"    representation: { kind: 'raw-point', fields: ['x', 'y'] },",
		"    data: [[0, 1], [1, 3], [2, 2], [3, 4]]",
		"  }]",
		"});",
		"const env = agent.viz.lines(spec, { width: 12, height: 3 });",
		"console.println(JSON.stringify(env));",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent viz lines script failed: %v\n%s", err, output)
	}

	var env map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &env); err != nil {
		t.Fatalf("parse render envelope JSON: %v\n%s", err, output)
	}
	if env["__agentRender"] != true {
		t.Fatalf("__agentRender = %v, want true", env["__agentRender"])
	}
	if env["schema"] != "agent-render/v1" {
		t.Fatalf("schema = %v, want agent-render/v1", env["schema"])
	}
	if env["renderer"] != "viz.tui" {
		t.Fatalf("renderer = %v, want viz.tui", env["renderer"])
	}
	if env["mode"] != "lines" {
		t.Fatalf("mode = %v, want lines", env["mode"])
	}
	lines, ok := env["lines"].([]any)
	if !ok {
		t.Fatalf("lines type = %T, want []any", env["lines"])
	}
	if len(lines) == 0 {
		t.Fatalf("lines length = 0, want > 0")
	}
}

func TestAgentProfileVizBlocksReturnsRenderEnvelope(t *testing.T) {
	workDir := t.TempDir()

	script := strings.Join([]string{
		"const advn = require('vizspec');",
		"const agent = require('repl/profiles/agent');",
		"const spec = advn.createSpec({",
		"  series: [{",
		"    id: 'cpu',",
		"    representation: { kind: 'raw-point', fields: ['x', 'y'] },",
		"    data: [[0, 1], [1, 3], [2, 2], [3, 4]]",
		"  }]",
		"});",
		"const env = agent.viz.blocks(spec, { width: 20, rows: 4, compact: true });",
		"console.println(JSON.stringify(env));",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent viz blocks script failed: %v\n%s", err, output)
	}

	var env map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &env); err != nil {
		t.Fatalf("parse render envelope JSON: %v\n%s", err, output)
	}
	if env["mode"] != "blocks" {
		t.Fatalf("mode = %v, want blocks", env["mode"])
	}
	blocks, ok := env["blocks"].([]any)
	if !ok {
		t.Fatalf("blocks type = %T, want []any", env["blocks"])
	}
	if len(blocks) == 0 {
		t.Fatalf("blocks length = 0, want > 0")
	}
}

func TestAgentProfileVizFromRowsExplicitY(t *testing.T) {
	workDir := t.TempDir()

	// Simulates the common pattern: query result rows → agent.viz.fromRows()
	script := strings.Join([]string{
		"const agent = require('repl/profiles/agent');",
		"const rows = [",
		"  { TIME: 0, LAT: 43.75, LON: 11.29 },",
		"  { TIME: 1, LAT: 43.76, LON: 11.30 },",
		"  { TIME: 2, LAT: 43.77, LON: 11.31 },",
		"  { TIME: 3, LAT: 43.78, LON: 11.32 },",
		"];",
		"const env = agent.viz.fromRows(rows, { x: 'TIME', y: ['LAT', 'LON'], width: 20, height: 3 });",
		"console.println(JSON.stringify(env));",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent viz fromRows script failed: %v\n%s", err, output)
	}

	var env map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &env); err != nil {
		t.Fatalf("parse render envelope JSON: %v\n%s", err, output)
	}
	if env["__agentRender"] != true {
		t.Fatalf("__agentRender = %v, want true", env["__agentRender"])
	}
	if env["mode"] != "lines" {
		t.Fatalf("mode = %v, want lines", env["mode"])
	}
	lines, ok := env["lines"].([]any)
	if !ok {
		t.Fatalf("lines type = %T, want []any", env["lines"])
	}
	if len(lines) == 0 {
		t.Fatalf("lines length = 0, want > 0")
	}
	meta, ok := env["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta type = %T, want map[string]any", env["meta"])
	}
	// 2 y-fields → 2 series
	if seriesCount, _ := meta["seriesCount"].(int64); seriesCount != 2 {
		if seriesCountF, _ := meta["seriesCount"].(float64); int(seriesCountF) != 2 {
			t.Fatalf("meta.seriesCount = %v, want 2", meta["seriesCount"])
		}
	}
}

func TestAgentProfileVizFromRowsAutoDetectY(t *testing.T) {
	workDir := t.TempDir()

	// When y is omitted, numeric fields other than x should be auto-detected.
	script := strings.Join([]string{
		"const agent = require('repl/profiles/agent');",
		"const rows = [",
		"  { TIME: 0, VALUE: 1.1 },",
		"  { TIME: 1, VALUE: 2.2 },",
		"  { TIME: 2, VALUE: 3.3 },",
		"  { TIME: 3, VALUE: 4.4 },",
		"];",
		"const env = agent.viz.fromRows(rows, { x: 'TIME', width: 20, height: 3 });",
		"console.println(env.__agentRender);",
		"console.println(env.mode);",
	}, "\n")

	output, err := runScript(workDir, nil, script)
	if err != nil {
		t.Fatalf("agent viz fromRows auto-y script failed: %v\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 output lines, got: %s", output)
	}
	if lines[0] != "true" {
		t.Fatalf("__agentRender = %v, want true", lines[0])
	}
	if lines[1] != "lines" {
		t.Fatalf("mode = %v, want lines", lines[1])
	}
}
