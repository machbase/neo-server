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
