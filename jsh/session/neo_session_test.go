package session

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestNormalizeShellArgs(t *testing.T) {
	testCases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "empty args",
			args: nil,
			want: nil,
		},
		{
			name: "non sql command unchanged",
			args: []string{"show", "tables"},
			want: []string{"show", "tables"},
		},
		{
			name: "select prepends sql",
			args: []string{"SELECT", "*", "FROM", "example"},
			want: []string{"sql", "SELECT", "*", "FROM", "example"},
		},
		{
			name: "exec prepends sql case insensitive",
			args: []string{"exec", "table_flush(example)"},
			want: []string{"sql", "exec", "table_flush(example)"},
		},
		{
			name: "single arg sql statement prepends sql",
			args: []string{"SELECT time, value FROM example WHERE name='my-car'"},
			want: []string{"sql", "SELECT time, value FROM example WHERE name='my-car'"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeShellArgs(tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("normalizeShellArgs() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

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

func TestConfigureLoginFails404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "wrong",
		env:      map[string]any{},
	})
	if err != ErrUserOrPasswordIncorrect {
		t.Fatalf("Configure() error = %v, want ErrUserOrPasswordIncorrect", err)
	}
}

func TestConfigureLoginFailsNon404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want error")
	}
}

func TestConfigureRpcFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "tok",
			"refreshToken": "rtok",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want error")
	}
}

func TestBuildHttpURL(t *testing.T) {
	tests := []struct {
		proto string
		host  string
		port  int
		path  string
		want  string
	}{
		{"http", "localhost", 5654, "/api/v1", "http://localhost:5654/api/v1"},
		{"https", "example.com", 0, "/api/v1", "https://example.com/api/v1"},
		{"http", "unix", 0, "/api/v1", "http://unix/api/v1"},
	}
	for _, tt := range tests {
		got := buildHttpURL(tt.proto, tt.host, tt.port, tt.path)
		if got != tt.want {
			t.Errorf("buildHttpURL(%q, %q, %d, %q) = %q, want %q", tt.proto, tt.host, tt.port, tt.path, got, tt.want)
		}
	}
}

func TestResolveUnixSocketPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantAbs bool // just check the result is absolute and not panicking
	}{
		{"unix:// absolute", "unix:///tmp/test.sock", true},
		{"absolute path", "/tmp/test.sock", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveUnixSocketPath(tt.input)
			if err != nil {
				t.Fatalf("resolveUnixSocketPath(%q) error = %v", tt.input, err)
			}
			if tt.wantAbs && (len(got) == 0 || got[0] != '/') {
				t.Errorf("resolveUnixSocketPath(%q) = %q, want absolute path", tt.input, got)
			}
		})
	}
}

func TestResolveUnixSocketPathRelative(t *testing.T) {
	// unix://./relative.sock and ../relative.sock should produce absolute paths
	tests := []string{
		"unix://./relative.sock",
		"unix://../relative.sock",
		"./relative.sock",
		"../relative.sock",
	}
	for _, input := range tests {
		got, err := resolveUnixSocketPath(input)
		if err != nil {
			t.Errorf("resolveUnixSocketPath(%q) error = %v", input, err)
			continue
		}
		if len(got) == 0 || got[0] != '/' {
			t.Errorf("resolveUnixSocketPath(%q) = %q, want absolute path", input, got)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"localhost.localdomain", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"192.168.1.1", false},
		{"example.com", false},
		{"10.0.0.1", false},
	}
	for _, tt := range tests {
		got := isLoopback(tt.host)
		if got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestGetHttpConfigAndTokens(t *testing.T) {
	// Set up a known defaultSession state
	defaultSession = Config{
		httpProto:    "http",
		httpHost:     "myhost",
		httpPort:     1234,
		User:         "admin",
		Password:     "secret",
		accessToken:  "at",
		refreshToken: "rt",
		machHost:     "machhost",
		machPort:     5656,
	}

	cfg := GetHttpConfig()
	if cfg.Protocol != "http:" || cfg.Host != "myhost" || cfg.Port != 1234 || cfg.User != "admin" || cfg.Password != "secret" {
		t.Fatalf("GetHttpConfig() = %+v", cfg)
	}

	SetHttpToken("new-at", "new-rt")
	if got := GetHttpAccessToken(); got != "new-at" {
		t.Fatalf("GetHttpAccessToken() = %q, want %q", got, "new-at")
	}
	if got := GetHttpRefreshToken(); got != "new-rt" {
		t.Fatalf("GetHttpRefreshToken() = %q, want %q", got, "new-rt")
	}

	mach := GetMachCliConfig()
	if mach.Host != "machhost" || mach.Port != 5656 || mach.User != "admin" || mach.Password != "secret" {
		t.Fatalf("GetMachCliConfig() = %+v", mach)
	}
}

func TestConfigureRpcJsonDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "tok",
			"refreshToken": "rtok",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json")) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want JSON decode error")
	}
}

func TestConfigureInvalidServer(t *testing.T) {
	err := Configure(Config{
		Server:   "not-a-valid-host:port",
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want error for invalid server address")
	}
}

func TestConfigureLoginJsonDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json")) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := Configure(Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want JSON decode error")
	}
}

func TestConfigureSortsCandidatesPreferringHttpHost(t *testing.T) {
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
				{"Service": "mach", "Address": "tcp://192.168.1.10:5656"},
				{"Service": "mach", "Address": "tcp://127.0.0.1:5657"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := Config{
		Server:   strings.TrimPrefix(srv.URL, "http://"),
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	}
	if err := Configure(cfg); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	// loopback should be preferred over non-loopback
	if defaultSession.machHost != "127.0.0.1" {
		t.Fatalf("machHost = %q, want 127.0.0.1", defaultSession.machHost)
	}
}

func TestConfigureWithUnixSocket(t *testing.T) {
	socketPath := t.TempDir() + "/test.sock"
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/web/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "unix-at",
			"refreshToken": "unix-rt",
		})
	})
	mux.HandleFunc("/web/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]string{
				{"Service": "mach", "Address": "tcp://127.0.0.1:5656"},
			},
		})
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(l) //nolint:errcheck
	defer srv.Close()

	err = Configure(Config{
		Server:   "unix://" + socketPath,
		User:     "sys",
		Password: "manager",
		env:      map[string]any{},
	})
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	if defaultSession.httpUnix != socketPath {
		t.Fatalf("httpUnix = %q, want %q", defaultSession.httpUnix, socketPath)
	}
}

func TestResolveUnixSocketPathAbsoluteNoPrefix(t *testing.T) {
	// A plain absolute path (starting with /) should be resolved correctly
	got, err := resolveUnixSocketPath("/var/run/test.sock")
	if err != nil {
		t.Fatalf("resolveUnixSocketPath error = %v", err)
	}
	if got != "/var/run/test.sock" {
		t.Fatalf("got = %q, want %q", got, "/var/run/test.sock")
	}
}

func TestResolveUnixSocketPathBareRelative(t *testing.T) {
	// A bare relative path (no ./ or unix:// prefix) should be joined with cwd
	pwd, _ := os.Getwd()
	got, err := resolveUnixSocketPath("relative.sock")
	if err != nil {
		t.Fatalf("resolveUnixSocketPath error = %v", err)
	}
	if !strings.HasPrefix(got, pwd) {
		t.Fatalf("got = %q, expected to start with %q", got, pwd)
	}
}

func TestModuleRegistersExports(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	_ = module.Set("exports", exports)

	Module(context.Background(), rt, module)

	for _, name := range []string{"getHttpConfig", "setHttpToken", "getHttpAccessToken", "getHttpRefreshToken", "getMachCliConfig"} {
		if exports.Get(name) == nil {
			t.Errorf("exports.%s is not set", name)
		}
	}
}
