package tailer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTail(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create and write initial content
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fmt.Fprintln(f, "line 1")
	fmt.Fprintln(f, "line 2")
	f.Close()

	// Start tailing
	tail := New(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer func() {
		tail.Stop()
		// Give time for file handles to close on Windows
		time.Sleep(50 * time.Millisecond)
	}()

	// Append new lines
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	fmt.Fprintln(f, "line 3")
	fmt.Fprintln(f, "line 4")
	f.Close()

	// Read the lines (should include last 2 lines from initial file + 2 new lines)
	timeout := time.After(2 * time.Second)
	lines := []string{}

	for i := 0; i < 4; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			t.Fatal("Timeout waiting for lines")
		}
	}

	if len(lines) != 4 {
		t.Fatalf("Expected 4 lines, got %d: %v", len(lines), lines)
	}

	if lines[0] != "line 1" {
		t.Errorf("Expected 'line 1', got '%s'", lines[0])
	}

	if lines[1] != "line 2" {
		t.Errorf("Expected 'line 2', got '%s'", lines[1])
	}

	if lines[2] != "line 3" {
		t.Errorf("Expected 'line 3', got '%s'", lines[2])
	}

	if lines[3] != "line 4" {
		t.Errorf("Expected 'line 4', got '%s'", lines[3])
	}
}

func TestTailRotation(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create and write initial content
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fmt.Fprintln(f, "line 1")
	f.Close()

	// Start tailing
	tail := New(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer func() {
		tail.Stop()
		// Give time for file handles to close on Windows
		time.Sleep(50 * time.Millisecond)
	}()

	// Append a line
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	fmt.Fprintln(f, "line 2")
	f.Close()

	// Simulate log rotation
	rotatedFile := filepath.Join(tmpDir, "test.log.old")
	if err := os.Rename(testFile, rotatedFile); err != nil {
		t.Fatalf("Failed to rotate file: %v", err)
	}

	// Create new file with same name
	f, err = os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}
	fmt.Fprintln(f, "line 3 (new file)")
	f.Close()

	// Wait a bit for rotation detection
	time.Sleep(300 * time.Millisecond)

	// Append more lines to new file
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open new file: %v", err)
	}
	fmt.Fprintln(f, "line 4 (new file)")
	f.Close()

	// Read the lines
	timeout := time.After(2 * time.Second)
	lines := []string{}

	// Should get: line 1 (initial), line 2 (before rotation), line 3 (new file), line 4 (new file)
	for i := 0; i < 4; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			t.Logf("Got %d lines so far: %v", len(lines), lines)
			t.Fatal("Timeout waiting for lines")
		}
	}

	// Should have read initial line, line 2 from old file, and lines from new file
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines, got %d: %v", len(lines), lines)
	}

	// Check that we got the lines
	foundLine1 := false
	foundLine2 := false
	foundLine3 := false
	foundLine4 := false

	for _, line := range lines {
		if line == "line 1" {
			foundLine1 = true
		}
		if line == "line 2" {
			foundLine2 = true
		}
		if line == "line 3 (new file)" {
			foundLine3 = true
		}
		if line == "line 4 (new file)" {
			foundLine4 = true
		}
	}

	if !foundLine1 {
		t.Error("Did not find 'line 1' from initial file")
	}
	if !foundLine2 {
		t.Error("Did not find 'line 2' from old file")
	}
	if !foundLine3 {
		t.Error("Did not find 'line 3 (new file)' from new file")
	}
	if !foundLine4 {
		t.Error("Did not find 'line 4 (new file)' from new file")
	}
}

func TestTailTruncation(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create and write initial content
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fmt.Fprintln(f, "line 1")
	fmt.Fprintln(f, "line 2")
	f.Close()

	// Start tailing
	tail := New(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer func() {
		tail.Stop()
		// Give time for file handles to close on Windows
		time.Sleep(50 * time.Millisecond)
	}()

	// Truncate and write new content
	// First truncate the file
	f, err = os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to truncate file: %v", err)
	}
	f.Close()

	// Wait for tail to detect the truncation
	time.Sleep(150 * time.Millisecond)

	// Now write new content
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	fmt.Fprintln(f, "line 3 (after truncate)")
	f.Sync() // Ensure data is written to disk
	f.Close()

	// Wait for the data to be read
	time.Sleep(150 * time.Millisecond)

	// Read the lines (should include last 2 from initial + line after truncate)
	timeout := time.After(2 * time.Second)
	lines := []string{}

	for i := 0; i < 3; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			if i < 3 {
				// We expect at least the initial lines and the line after truncate
				t.Logf("Got %d lines: %v", len(lines), lines)
				break
			}
			t.Fatal("Timeout waiting for line after truncation")
		}
	}

	// Should have initial lines plus line after truncate
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines, got %d: %v", len(lines), lines)
	}

	// Check we got the expected lines
	foundLine1 := false
	foundLine2 := false
	foundLine3 := false

	for _, line := range lines {
		if line == "line 1" {
			foundLine1 = true
		}
		if line == "line 2" {
			foundLine2 = true
		}
		if line == "line 3 (after truncate)" {
			foundLine3 = true
		}
	}

	if !foundLine1 {
		t.Error("Did not find 'line 1' from initial file")
	}
	if !foundLine2 {
		t.Error("Did not find 'line 2' from initial file")
	}
	if !foundLine3 {
		t.Error("Did not find 'line 3 (after truncate)'")
	}
}

func TestTailGrepPattern(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create and write initial content
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fmt.Fprintln(f, "info: all is well")
	f.Close()

	tail := New(testFile, WithPollInterval(100*time.Millisecond), WithPattern("error", "thing"), WithPattern("info:"))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer func() {
		tail.Stop()
		// Give time for file handles to close on Windows
		time.Sleep(50 * time.Millisecond)
	}()

	// Append new lines
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	fmt.Fprintln(f, "error: something went wrong")
	fmt.Fprintln(f, "warning: check this out")
	fmt.Fprintln(f, "warning: other thing")
	fmt.Fprintln(f, "error: another thing error occurred")
	fmt.Fprintln(f, "info: just an informational message")
	fmt.Fprintln(f, "warning: be cautious")
	f.Close()

	// Read the filtered lines
	timeout := time.After(2 * time.Second)
	lines := []string{}

	// Should get: initial "info: all is well" + 3 new matching lines = 4 total
	for i := 0; i < 4; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			fmt.Println("Lines received so far:", lines)
			t.Fatal("Timeout waiting for lines")
		}
	}

	expectedLines := []string{
		"info: all is well",
		"error: something went wrong",
		"error: another thing error occurred",
		"info: just an informational message",
	}

	if len(lines) != len(expectedLines) {
		t.Fatalf("Expected %d lines, got %d: %v", len(expectedLines), len(lines), lines)
	}

	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("Expected '%s', got '%s'", expected, lines[i])
		}
	}
}

func TestTailWithColoringPlugin(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create and write initial content with different log levels
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fmt.Fprintln(f, "TRACE This is a trace message")
	fmt.Fprintln(f, "DEBUG This is a debug message")
	f.Close()

	// Create tail with coloring plugin
	coloringPlugin := NewSyntaxColoring("levels")
	tail := New(testFile,
		WithPollInterval(100*time.Millisecond),
		WithPlugins(coloringPlugin),
	)

	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer func() {
		tail.Stop()
		// Give time for file handles to close on Windows
		time.Sleep(50 * time.Millisecond)
	}()

	// Append new lines with different log levels
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	fmt.Fprintln(f, "INFO This is an info message")
	fmt.Fprintln(f, "WARN This is a warning message")
	fmt.Fprintln(f, "ERROR This is an error message")
	f.Close()

	// Read the colored lines
	timeout := time.After(2 * time.Second)
	lines := []string{}

	// Should get: 2 initial lines + 3 new lines = 5 total
	for i := 0; i < 5; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			t.Fatalf("Timeout waiting for lines, got %d lines: %v", len(lines), lines)
		}
	}

	if len(lines) != 5 {
		t.Fatalf("Expected 5 lines, got %d: %v", len(lines), lines)
	}

	// Check that TRACE has dark gray color codes
	if lines[0] != "\033[90mTRACE\033[0m This is a trace message" {
		t.Errorf("Expected TRACE to be colored, got '%s'", lines[0])
	}

	// Check that DEBUG has light gray color codes
	if lines[1] != "\033[37mDEBUG\033[0m This is a debug message" {
		t.Errorf("Expected DEBUG to be colored, got '%s'", lines[1])
	}

	// Check that INFO has green color codes
	if lines[2] != "\033[32mINFO\033[0m This is an info message" {
		t.Errorf("Expected INFO to be colored, got '%s'", lines[2])
	}

	// Check that WARN has yellow color codes
	if lines[3] != "\033[33mWARN\033[0m This is a warning message" {
		t.Errorf("Expected WARN to be colored, got '%s'", lines[3])
	}

	// Check that ERROR has red color codes
	if lines[4] != "\033[31mERROR\033[0m This is an error message" {
		t.Errorf("Expected ERROR to be colored, got '%s'", lines[4])
	}
}
