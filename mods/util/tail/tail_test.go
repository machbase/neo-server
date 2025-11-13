package tail

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
	tail := NewTail(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer tail.Stop()

	// Append new lines
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	fmt.Fprintln(f, "line 3")
	fmt.Fprintln(f, "line 4")
	f.Close()

	// Read the new lines
	timeout := time.After(2 * time.Second)
	lines := []string{}

	for i := 0; i < 2; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			t.Fatal("Timeout waiting for lines")
		}
	}

	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	if lines[0] != "line 3" {
		t.Errorf("Expected 'line 3', got '%s'", lines[0])
	}

	if lines[1] != "line 4" {
		t.Errorf("Expected 'line 4', got '%s'", lines[1])
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
	tail := NewTail(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer tail.Stop()

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

	for i := 0; i < 3; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			t.Logf("Got %d lines so far: %v", len(lines), lines)
			t.Fatal("Timeout waiting for lines")
		}
	}

	// Should have read line 2 from old file, and lines from new file
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d: %v", len(lines), lines)
	}

	// Check that we got line 2
	foundLine2 := false
	foundLine3 := false
	foundLine4 := false

	for _, line := range lines {
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
	tail := NewTail(testFile, WithPollInterval(100*time.Millisecond))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer tail.Stop()

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

	// Read the new line
	timeout := time.After(2 * time.Second)

	select {
	case line := <-tail.Lines():
		if line != "line 3 (after truncate)" {
			t.Errorf("Expected 'line 3 (after truncate)', got '%s'", line)
		}
	case <-timeout:
		t.Fatal("Timeout waiting for line after truncation")
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

	tail := NewTail(testFile, WithPollInterval(100*time.Millisecond), WithPattern("error", "thing"), WithPattern("info:"))
	if err := tail.Start(); err != nil {
		t.Fatalf("Failed to start tail: %v", err)
	}
	defer tail.Stop()

	// Append new lines
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	fmt.Fprintln(f, "error: something went wrong")
	fmt.Fprintln(f, "warning: check this out")
	fmt.Fprintln(f, "warning: otherthing")
	fmt.Fprintln(f, "error: another thing error occurred")
	fmt.Fprintln(f, "info: just an informational message")
	fmt.Fprintln(f, "warning: be cautious")
	f.Close()

	// Read the filtered lines
	timeout := time.After(2 * time.Second)
	lines := []string{}

	for i := 0; i < 3; i++ {
		select {
		case line := <-tail.Lines():
			lines = append(lines, line)
		case <-timeout:
			fmt.Println("Lines received so far:", lines)
			t.Fatal("Timeout waiting for lines")
		}
	}

	expectedLines := []string{
		"error: something went wrong",
		"error: another thing error occurred",
		"info: just an informational message",
	}

	if len(lines) != len(expectedLines) {
		t.Fatalf("Expected %d lines, got %d", len(expectedLines), len(lines))
	}

	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("Expected '%s', got '%s'", expected, lines[i])
		}
	}
}
