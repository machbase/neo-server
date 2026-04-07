package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigureHandlesUnixServicePorts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access-token",
			"refreshToken": "refresh-token",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "unix:///tmp/mach.sock"},
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
				{"Service": "servicectl", "Address": "tcp://127.0.0.1:9999"},
				{"Service": "servicectl", "Address": "unix:///tmp/servicectl.sock"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	serverAddr := strings.TrimPrefix(srv.URL, "http://")
	cfg := Config{
		Server:   serverAddr,
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	}
	if err := Configure(cfg); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if got := defaultSession.env["SERVICE_CONTROLLER"]; got != "unix:///tmp/servicectl.sock" {
		t.Fatalf("SERVICE_CONTROLLER = %v", got)
	}
	if got := GetMachCliConfig(); got.Host != "127.0.0.1" || got.Port != 5656 {
		t.Fatalf("GetMachCliConfig() = %+v", got)
	}
}

func TestConfigureFailsWithoutTcpMachPort(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access-token",
			"refreshToken": "refresh-token",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "unix:///tmp/mach.sock"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
	})
	if err == nil {
		t.Fatal("Configure() error = nil")
	}
	if !strings.Contains(err.Error(), "tcp://") {
		t.Fatalf("Configure() error = %v", err)
	}
}

func TestConfigureInitializesEnvAndFallsBackToTcpServiceController(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access-token",
			"refreshToken": "refresh-token",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
				{"Service": "servicectl", "Address": "tcp://127.0.0.1:9999"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
	})
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if defaultSession.env == nil {
		t.Fatal("defaultSession.env = nil")
	}
	if got := defaultSession.env["SERVICE_CONTROLLER"]; got != "tcp://127.0.0.1:9999" {
		t.Fatalf("SERVICE_CONTROLLER = %v", got)
	}
}
