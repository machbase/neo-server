package restclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "method_and_path",
			input:    "GET /api/data",
			expected: []string{"GET", "/api/data", ""},
		},
		{
			name:     "method_and_path",
			input:    "GET /api/data HTTP/1.1",
			expected: []string{"GET", "/api/data", "HTTP/1.1", ""},
		},
		{
			name:     "method_and_path_with_query",
			input:    "GET /api/data?p1=v1&p2=a",
			expected: []string{"GET", "/api/data?p1=v1&p2=a", ""},
		},
		{
			name:     "method_and_path_with_query_and_version",
			input:    "GET /api/data?p1=v1&p2=a b HTTP/1.1",
			expected: []string{"GET", "/api/data?p1=v1&p2=a b", "HTTP/1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, version := parseCommandLine(tt.input)
			require.Equal(t, tt.expected[0], method, "method mismatch")
			require.Equal(t, tt.expected[1], path, "path mismatch")
			require.Equal(t, tt.expected[2], version, "version mismatch")
		})
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedMethod  string
		expectedPath    string
		expectedVersion string
		expectedHeaders http.Header
		expectedContent []string
	}{
		{
			name:            "basic_get_simple",
			content:         `GET /api/data`,
			expectedMethod:  "GET",
			expectedPath:    "/api/data",
			expectedVersion: "",
			expectedHeaders: http.Header{},
			expectedContent: []string{},
		},
		{
			name: "basic_get",
			content: `
				GET /api/data HTTP/1.1
				Host: example.com
				User-Agent: TestClient
			`,
			expectedMethod:  "GET",
			expectedPath:    "/api/data",
			expectedVersion: "HTTP/1.1",
			expectedHeaders: http.Header{
				"Host":       []string{"example.com"},
				"User-Agent": []string{"TestClient"},
			},
			expectedContent: []string{},
		},
		{
			name: "basic_get_params_version_oneline",
			content: `
				GET /api/data?param1=select count(*) from my_table&param2=value2 HTTP/1.0
				Host: example.com
				User-Agent: TestClient
			`,
			expectedMethod:  "GET",
			expectedPath:    "/api/data?param1=select+count%28%2A%29+from+my_table&param2=value2",
			expectedVersion: "HTTP/1.0",
			expectedHeaders: http.Header{
				"Host":       []string{"example.com"},
				"User-Agent": []string{"TestClient"},
			},
			expectedContent: []string{},
		},
		{
			name: "basic_get_params_version",
			content: `
				GET /api/data?param1=select count(*) from my_table
					&param2=value2
					HTTP/1.0
				Host: example.com
				User-Agent: TestClient
			`,
			expectedMethod:  "GET",
			expectedPath:    "/api/data?param1=select+count%28%2A%29+from+my_table&param2=value2",
			expectedVersion: "HTTP/1.0",
			expectedHeaders: http.Header{
				"Host":       []string{"example.com"},
				"User-Agent": []string{"TestClient"},
			},
			expectedContent: []string{},
		},
		{
			name: "basic_post",
			content: `
				POST /api/data HTTP/1.1
				Host: example.com
				User-Agent: TestClient
				Content-Type: application/json

				{
					"key": "value"
				}
			`,
			expectedMethod:  "POST",
			expectedPath:    "/api/data",
			expectedVersion: "HTTP/1.1",
			expectedHeaders: http.Header{
				"Host":         []string{"example.com"},
				"User-Agent":   []string{"TestClient"},
				"Content-Type": []string{"application/json"},
			},
			expectedContent: []string{
				`{`,
				`"key": "value"`,
				`}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := Parse(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rc.method != tt.expectedMethod {
				t.Errorf("expected method %q, got %q", tt.expectedMethod, rc.method)
			}
			if rc.path != tt.expectedPath {
				t.Errorf("expected path %q, got %q", tt.expectedPath, rc.path)
			}
			if rc.version != tt.expectedVersion {
				t.Errorf("expected version %q, got %q", tt.expectedVersion, rc.version)
			}

			for k, vals := range tt.expectedHeaders {
				if len(rc.header[k]) != len(vals) {
					t.Errorf("expected %d values for header %q, got %d", len(vals), k, len(rc.header[k]))
				} else {
					for i, val := range vals {
						if rc.header[k][i] != val {
							t.Errorf("expected header %q value %q, got %q", k, val, rc.header[k][i])
						}
					}
				}
			}

			if len(rc.contentLines) != len(tt.expectedContent) {
				t.Errorf("expected %d content lines, got %d", len(tt.expectedContent), len(rc.contentLines))
			} else {
				for i, content := range tt.expectedContent {
					if rc.contentLines[i] != content {
						t.Errorf("expected content line %q, got %q", content, rc.contentLines[i])
					}
				}
			}
		})
	}
}
