package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

// TestNeoShellConfigurePropagatesCurrentDatabase reproduces a bug where `use <database>`
// set NEOSHELL_DATABASE in the shell's env, but a child process spawned via
// process.exec() (which re-enters NeoShellMain -> neoShellConfigure -> Configure())
// never read that env var into Config.Database, so GetMachCliConfig().Database
// stayed empty in the child even though the env var was present. --database is now a
// regular ExtFlag, handled the same way as --server/--user/--password/--identity-file.
func TestNeoShellConfigurePropagatesCurrentDatabase(t *testing.T) {
	prev := defaultSession
	t.Cleanup(func() {
		defaultSession = prev
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "tok",
			"refreshToken": "rtok",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	serverAddr := strings.TrimPrefix(srv.URL, "http://")
	extFlags := ExtFlags{
		{flag: "server", value: serverAddr, envKey: "NEOSHELL_HOST"},
		{flag: "user", value: "sys", envKey: "NEOSHELL_USER"},
		{flag: "password", value: "manager", envKey: "NEOSHELL_PASSWORD"},
		{flag: "identity-file", value: "", envKey: "NEOSHELL_IDENTITY_FILE"},
		{flag: "database", value: "", envKey: "NEOSHELL_DATABASE"},
	}
	conf := &engine.Config{
		Env: map[string]any{
			"NEOSHELL_DATABASE": "FACTORY_A",
		},
		Aliases: map[string]string{},
	}

	configure := neoShellConfigure(nil, nil)
	if err := configure(conf, extFlags); err != nil {
		t.Fatalf("neoShellConfigure() error = %v", err)
	}

	if defaultSession.Database != "FACTORY_A" {
		t.Fatalf("defaultSession.Database = %q, want FACTORY_A", defaultSession.Database)
	}
	if got := GetMachCliConfig().Database; got != "FACTORY_A" {
		t.Fatalf("GetMachCliConfig().Database = %q, want FACTORY_A", got)
	}
	if got, _ := conf.Env["NEOSHELL_DATABASE"].(string); got != "FACTORY_A" {
		t.Fatalf("conf.Env[NEOSHELL_DATABASE] = %q, want FACTORY_A", got)
	}
}

func TestNeoShellConfigureLeavesCurrentDatabaseEmptyWhenUnset(t *testing.T) {
	prev := defaultSession
	t.Cleanup(func() {
		defaultSession = prev
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "tok",
			"refreshToken": "rtok",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	serverAddr := strings.TrimPrefix(srv.URL, "http://")
	extFlags := ExtFlags{
		{flag: "server", value: serverAddr, envKey: "NEOSHELL_HOST"},
		{flag: "user", value: "sys", envKey: "NEOSHELL_USER"},
		{flag: "password", value: "manager", envKey: "NEOSHELL_PASSWORD"},
		{flag: "identity-file", value: "", envKey: "NEOSHELL_IDENTITY_FILE"},
		{flag: "database", value: "", envKey: "NEOSHELL_DATABASE"},
	}
	conf := &engine.Config{
		Env:     map[string]any{},
		Aliases: map[string]string{},
	}

	configure := neoShellConfigure(nil, nil)
	if err := configure(conf, extFlags); err != nil {
		t.Fatalf("neoShellConfigure() error = %v", err)
	}

	if defaultSession.Database != "" {
		t.Fatalf("defaultSession.Database = %q, want empty", defaultSession.Database)
	}
}

func TestNeoShellConfigureDatabaseFlagTakesPrecedenceOverEnv(t *testing.T) {
	prev := defaultSession
	t.Cleanup(func() {
		defaultSession = prev
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "tok",
			"refreshToken": "rtok",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	serverAddr := strings.TrimPrefix(srv.URL, "http://")
	extFlags := ExtFlags{
		{flag: "server", value: serverAddr, envKey: "NEOSHELL_HOST"},
		{flag: "user", value: "sys", envKey: "NEOSHELL_USER"},
		{flag: "password", value: "manager", envKey: "NEOSHELL_PASSWORD"},
		{flag: "identity-file", value: "", envKey: "NEOSHELL_IDENTITY_FILE"},
		{flag: "database", value: "FACTORY_B", envKey: "NEOSHELL_DATABASE"},
	}
	conf := &engine.Config{
		Env: map[string]any{
			"NEOSHELL_DATABASE": "FACTORY_A",
		},
		Aliases: map[string]string{},
	}

	configure := neoShellConfigure(nil, nil)
	if err := configure(conf, extFlags); err != nil {
		t.Fatalf("neoShellConfigure() error = %v", err)
	}

	if defaultSession.Database != "FACTORY_B" {
		t.Fatalf("defaultSession.Database = %q, want FACTORY_B (--database flag should win)", defaultSession.Database)
	}
}
