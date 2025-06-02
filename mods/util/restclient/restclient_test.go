package restclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
		{
			name: "do_post_formdata",
			content: `
				POST http://{{ .HostPort }}/api/echo
				Content-Type: application/x-www-form-urlencoded

				q=SELECT * FROM users where name = 'John'
				&format=json
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
		{
			name: "do_post_multipart",
			content: `
				POST http://{{ .HostPort }}/api/upload
				Content-Type: multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW

------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="name"

John
------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="image"; filename="1.png"
Content-Type: image/png

< 1.png
------WebKitFormBoundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="doc"; filename="1.xml"
Content-Type: text/xml

<@utf-8 1.xml
------WebKitFormBoundary7MA4YWxkTrZu0gW--
`,
			expectedFunc: func(t *testing.T, rr *RestResult) {
				require.NoError(t, rr.Err)
				body := rr.Body.String()
				require.JSONEq(t,
					`{"name": "John", "image": "1.png", "doc": "1.xml"}`,
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
			rc.SetFileLoader(func(name string) (io.ReadCloser, error) {
				return os.Open(filepath.Join("./test", name))
			})
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
				switch r.Header.Get("Content-Type") {
				case "application/json":
					err := json.NewDecoder(r.Body).Decode(&o)
					if err != nil {
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}
				case "application/x-www-form-urlencoded":
					o["q"] = r.PostFormValue("q")
					o["format"] = r.PostFormValue("format")
				}
			} else if r.Method == http.MethodGet {
				o["q"] = r.URL.Query().Get("q")
				o["format"] = r.URL.Query().Get("format")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(o)
		})
		http.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				http.Error(w, "Failed to parse form", http.StatusBadRequest)
				return
			}
			name := r.FormValue("name")

			imagePart, imagePartHeader, err := r.FormFile("image")
			if err != nil {
				http.Error(w, "Failed to get image", http.StatusBadRequest)
				return
			}
			defer imagePart.Close()
			partExpect, err := os.ReadFile(filepath.Join("test", imagePartHeader.Filename))
			if err != nil {
				http.Error(w, "Failed to read image file", http.StatusInternalServerError)
				return
			}
			partData := &bytes.Buffer{}
			io.Copy(partData, imagePart)
			if !bytes.Equal(partExpect, partData.Bytes()) {
				http.Error(w, "Uploaded image content does not match expected content", http.StatusBadRequest)
				return
			}

			docPart, docPartHeader, err := r.FormFile("doc")
			if err != nil {
				http.Error(w, "Failed to get image", http.StatusBadRequest)
				return
			}
			defer docPart.Close()

			partExpect, err = os.ReadFile(filepath.Join("test", docPartHeader.Filename))
			if err != nil {
				http.Error(w, "Failed to read image file", http.StatusInternalServerError)
				return
			}
			docData := &bytes.Buffer{}
			io.Copy(docData, docPart)
			if !bytes.Equal(partExpect, docData.Bytes()) {
				http.Error(w, "Uploaded doc content does not match expected content", http.StatusBadRequest)
				return
			}

			// Here you would typically save the image to a file or process it.
			response := map[string]string{
				"name":  name,
				"image": imagePartHeader.Filename,
				"doc":   docPartHeader.Filename,
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				return
			}
		})
		http.Serve(lsnr, nil)
	}()

	m.Run()

	lsnr.Close()
}
