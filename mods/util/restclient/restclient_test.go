package restclient

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedFunc func(*testing.T, *RestResult)
	}{
		{
			name: "do_get",
			content: `
				GET http://{{ .HostPort }}/api/data HTTP/1.1
				User-Agent: TestClient
			`,
			expectedFunc: func(t *testing.T, rr *RestResult) {
				require.NoError(t, rr.Err)
				data := rr.String()
				require.Contains(t, data, "HTTP/1.1 200 OK")
				require.Contains(t, data, "text/plain; charset=utf-8")
				require.Contains(t, data, "Content-Length: 11")
				require.Contains(t, data, "hello world")
			},
		},
		{
			name: "do_get_query_params",
			content: `
				GET http://{{ .HostPort }}/api/hello
				    ?name=World
					&greeting=Say Hello
				User-Agent: TestClient
			`,
			expectedFunc: func(t *testing.T, rr *RestResult) {
				require.NoError(t, rr.Err)
				data := rr.String()
				require.Contains(t, data, "HTTP/1.1 200 OK")
				require.Contains(t, data, "text/plain; charset=utf-8")
				require.Contains(t, data, "Content-Length: 15")
				require.Contains(t, data, "Say Hello World")
			},
		},
		{
			name: "do_get_sql",
			content: `
				GET http://{{ .HostPort }}/api/echo
				    ?q=SELECT * FROM users where name = 'John'
					&format=json
			`,
			expectedFunc: func(t *testing.T, rr *RestResult) {
				require.NoError(t, rr.Err)
				body := rr.Body.String()
				require.JSONEq(t,
					`{"q": "SELECT * FROM users where name = 'John'", "format": "json"}`,
					body, body)
			},
		},
		{
			name: "do_post_sql",
			content: `
				POST http://{{ .HostPort }}/api/echo
				Content-Type: application/json

				{"q": "SELECT * FROM users where name = 'John'", "format": "json"}
			`,
			expectedFunc: func(t *testing.T, rr *RestResult) {
				require.NoError(t, rr.Err)
				require.Contains(t, rr.String(), "X-Debug: 12345")
				body := rr.Body.String()
				require.JSONEq(t,
					`{"q": "SELECT * FROM users where name = 'John'", "format": "json"}`,
					body, body)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &strings.Builder{}
			tmpl := template.New("content")
			if _, err := tmpl.Parse(tt.content); err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}
			env := &Env{
				HostPort: hostPort,
			}
			err := tmpl.Execute(w, env)
			if err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}
			tt.content = w.String()
			rc, err := Parse(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result := rc.Do()
			tt.expectedFunc(t, result)
		})
	}
}

type Env struct {
	HostPort string
}

var hostPort string

func TestMain(m *testing.M) {
	lsnr, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	hostPort = lsnr.Addr().String()
	fmt.Println("Listening on", hostPort)
	go func() {
		http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello world"))
		})
		http.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
			greeting := r.URL.Query().Get("greeting")
			name := r.URL.Query().Get("name")
			w.Write([]byte(fmt.Sprintf("%s %s", greeting, name)))
		})
		http.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Debug", "12345")
			o := map[string]any{}
			if r.Method == http.MethodPost {
				err := json.NewDecoder(r.Body).Decode(&o)
				if err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
			} else if r.Method == http.MethodGet {
				o["q"] = r.URL.Query().Get("q")
				o["format"] = r.URL.Query().Get("format")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(o)
		})
		http.Serve(lsnr, nil)
	}()

	m.Run()

	lsnr.Close()
}
