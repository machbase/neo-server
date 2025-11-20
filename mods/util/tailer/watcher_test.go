package tailer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHandler_ServeHTTP tests the main routing logic
func TestHandler_ServeHTTP(t *testing.T) {
	terminal := NewTerminal(
		WithTail(createTestFile(t, "test1.log", "line1\n")),
	)
	defer terminal.Close()

	handler := terminal.Handler("/test")

	tests := []struct {
		name           string
		path           string
		expectSSE      bool
		expectStaticFS bool
	}{
		{
			name:           "SSE stream endpoint",
			path:           "/test/watch.stream",
			expectSSE:      true,
			expectStaticFS: false,
		},
		{
			name:           "Static file endpoint",
			path:           "/test/",
			expectSSE:      false,
			expectStaticFS: true,
		},
		{
			name:           "Static file with path",
			path:           "/test/xterm.js",
			expectSSE:      false,
			expectStaticFS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			// Add context with timeout for SSE endpoints
			if tt.expectSSE {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				req = req.WithContext(ctx)
			}

			rec := httptest.NewRecorder()

			if tt.expectSSE {
				// Run in goroutine for SSE to prevent blocking
				done := make(chan struct{})
				go func() {
					handler.ServeHTTP(rec, req)
					close(done)
				}()
				<-done
			} else {
				handler.ServeHTTP(rec, req)
			}

			if tt.expectSSE {
				if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
					t.Errorf("Expected status OK or BadRequest, got %d", rec.Code)
				}
				if tt.expectSSE && rec.Code == http.StatusOK {
					contentType := rec.Header().Get("Content-Type")
					if contentType != "text/event-stream" {
						t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
					}
				}
			}
		})
	}
}

// TestHandler_serveWatcher_SingleFile tests SSE streaming with a single file
func TestHandler_serveWatcher_SingleFile(t *testing.T) {
	tmpFile := createTestFile(t, "single.log", "initial line\n")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	// Start the handler in a goroutine
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Write some lines to the file
	time.Sleep(200 * time.Millisecond)
	appendToFile(t, tmpFile, "line 1\nline 2\nline 3\n")

	// Wait for context to timeout or handler to finish
	<-done

	// Check response headers
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	result := rec.Body.String()

	// Verify SSE format (data: prefix)
	if !strings.Contains(result, "data:") {
		t.Error("Response should contain SSE data format")
	}

	// Verify some content was streamed
	if len(result) == 0 {
		t.Error("Expected some data to be streamed")
	}
}

// TestHandler_serveWatcher_MultipleFiles tests SSE streaming with multiple files
func TestHandler_serveWatcher_MultipleFiles(t *testing.T) {
	tmpFile1 := createTestFile(t, "multi1.log", "")
	tmpFile2 := createTestFile(t, "multi2.log", "")

	terminal := NewTerminal(
		WithTailLabel("file1", tmpFile1, WithPollInterval(100*time.Millisecond)),
		WithTailLabel("file2", tmpFile2, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	// Request with file selection
	req := httptest.NewRequest(http.MethodGet, "/watch.stream?file=file1&file=file2", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Write to both files
	time.Sleep(200 * time.Millisecond)
	appendToFile(t, tmpFile1, "from file1\n")
	appendToFile(t, tmpFile2, "from file2\n")

	<-done

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	result := rec.Body.String()
	if !strings.Contains(result, "data:") {
		t.Error("Response should contain SSE data format")
	}
}

// TestHandler_serveWatcher_NoFilesSelected tests error case when no files selected
func TestHandler_serveWatcher_NoFilesSelected(t *testing.T) {
	tmpFile1 := createTestFile(t, "noselect1.log", "")
	tmpFile2 := createTestFile(t, "noselect2.log", "")

	terminal := NewTerminal(
		WithTailLabel("file1", tmpFile1),
		WithTailLabel("file2", tmpFile2),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	// Request without file parameter for multiple files
	req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "no logs selected") {
		t.Error("Expected 'no logs selected' error message")
	}
}

// TestHandler_serveWatcher_WithFilters tests SSE streaming with filter patterns
func TestHandler_serveWatcher_WithFilters(t *testing.T) {
	tmpFile := createTestFile(t, "filter.log", "")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	// Request with filter parameter (OR and AND logic)
	req := httptest.NewRequest(http.MethodGet, "/watch.stream?filter=ERROR&&critical||WARNING", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Write lines with different patterns
	time.Sleep(200 * time.Millisecond)
	appendToFile(t, tmpFile, "INFO: normal log\n")
	appendToFile(t, tmpFile, "ERROR: critical issue\n")
	appendToFile(t, tmpFile, "WARNING: potential problem\n")
	appendToFile(t, tmpFile, "DEBUG: trace info\n")

	<-done

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// TestHandler_serveWatcher_ContextCancellation tests proper cleanup on context cancellation
func TestHandler_serveWatcher_ContextCancellation(t *testing.T) {
	tmpFile := createTestFile(t, "cancel.log", "initial\n")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Cancel context after a short delay
	time.Sleep(500 * time.Millisecond)
	cancel()

	// Handler should exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Handler did not exit after context cancellation")
	}
}

// TestHandler_serveWatcher_TerminalClose tests cleanup when terminal is closed
func TestHandler_serveWatcher_TerminalClose(t *testing.T) {
	tmpFile := createTestFile(t, "termclose.log", "initial\n")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)

	handler := terminal.Handler("/")

	req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Close terminal after a short delay
	time.Sleep(500 * time.Millisecond)
	terminal.Close()

	// Handler should exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Handler did not exit after terminal close")
	}
}

// TestHandler_serveWatcher_SSEHeaders tests that proper SSE headers are set
func TestHandler_serveWatcher_SSEHeaders(t *testing.T) {
	tmpFile := createTestFile(t, "headers.log", "test\n")

	terminal := NewTerminal(
		WithTail(tmpFile),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	<-done

	// Check SSE headers
	expectedHeaders := map[string]string{
		"Content-Type":                "text/event-stream",
		"Cache-Control":               "no-cache",
		"Connection":                  "keep-alive",
		"Access-Control-Allow-Origin": "*",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := rec.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %s, got %s", header, expectedValue, actualValue)
		}
	}
}

// TestHandler_serveStatic tests static file serving
func TestHandler_serveStatic(t *testing.T) {
	tmpFile := createTestFile(t, "static.log", "test\n")

	terminal := NewTerminal(
		WithTail(tmpFile),
	)
	defer terminal.Close()

	handler := terminal.Handler("/app")

	tests := []struct {
		name       string
		path       string
		expectCode int
	}{
		{
			name:       "Index page",
			path:       "/app/",
			expectCode: http.StatusOK,
		},
		{
			name:       "Static JS file",
			path:       "/app/xterm.js",
			expectCode: http.StatusOK,
		},
		{
			name:       "Static CSS file",
			path:       "/app/xterm.css",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, rec.Code)
			}
		})
	}
}

// TestHandler_serveStatic_IndexTemplate tests index page template rendering
func TestHandler_serveStatic_IndexTemplate(t *testing.T) {
	tmpFile := createTestFile(t, "index.log", "test\n")

	terminal := NewTerminal(
		WithTailLabel("testfile", tmpFile),
		WithLocalization(map[string]string{"Log Viewer": "Template Test"}),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Check that template contains expected content
	if !strings.Contains(body, "Template Test") {
		t.Error("Index page should contain terminal title")
	}
}

// TestTerminal_Handler tests Handler creation
func TestTerminal_Handler(t *testing.T) {
	terminal := NewTerminal()
	defer terminal.Close()

	handler := terminal.Handler("/prefix")

	if handler.CutPrefix != "/prefix" {
		t.Errorf("Expected CutPrefix '/prefix', got '%s'", handler.CutPrefix)
	}

	if handler.fsServer == nil {
		t.Error("fsServer should not be nil")
	}
}

// TestTerminal_Close tests terminal close functionality
func TestTerminal_Close(t *testing.T) {
	terminal := NewTerminal()

	// Close should not panic
	terminal.Close()

	// Verify close channel is closed
	select {
	case <-terminal.closeCh:
		// Success - channel is closed
	default:
		t.Error("closeCh should be closed")
	}
}

// Helper functions

func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, name)

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return tmpFile
}

func appendToFile(t *testing.T, filepath, content string) {
	t.Helper()
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
}

// TestHandler_serveWatcher_EmptyFilter tests handling of empty filter strings
func TestHandler_serveWatcher_EmptyFilter(t *testing.T) {
	tmpFile := createTestFile(t, "emptyfilter.log", "")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	// Request with empty filter parameter
	req := httptest.NewRequest(http.MethodGet, "/watch.stream?filter=", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	appendToFile(t, tmpFile, "test line\n")

	<-done

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// TestHandler_serveWatcher_ComplexFilter tests complex filter combinations
func TestHandler_serveWatcher_ComplexFilter(t *testing.T) {
	tmpFile := createTestFile(t, "complexfilter.log", "")

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(100*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	// Complex filter: (ERROR AND critical) OR (WARNING AND high)
	req := httptest.NewRequest(http.MethodGet, "/watch.stream?filter=ERROR&&critical||WARNING&&high", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	appendToFile(t, tmpFile, "ERROR: critical system failure\n")
	appendToFile(t, tmpFile, "WARNING: high memory usage\n")
	appendToFile(t, tmpFile, "INFO: normal operation\n")

	<-done

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// Benchmark tests

func BenchmarkHandler_serveWatcher_SingleFile(b *testing.B) {
	tmpFile := createBenchFile(b, "bench.log", strings.Repeat("benchmark line\n", 1000))

	terminal := NewTerminal(
		WithTail(tmpFile, WithPollInterval(50*time.Millisecond)),
	)
	defer terminal.Close()

	handler := terminal.Handler("/")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/watch.stream", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		cancel()
	}
}

func createBenchFile(b *testing.B, name, content string) string {
	b.Helper()
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, name)

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create bench file: %v", err)
	}

	return tmpFile
}
