package restclient

import (
	"net/http"
	"testing"
)

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
