package parser

import (
	"strings"
	"testing"
)

func TestLineReaderBasic(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	// Read first line
	line, err := lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read first line: %v", err)
	}
	if line != "Line 1" {
		t.Errorf("Expected 'Line 1', got '%s'", line)
	}

	// Read second line
	line, err = lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read second line: %v", err)
	}
	if line != "Line 2" {
		t.Errorf("Expected 'Line 2', got '%s'", line)
	}

	// Read third line
	line, err = lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read third line: %v", err)
	}
	if line != "Line 3" {
		t.Errorf("Expected 'Line 3', got '%s'", line)
	}
}

func TestLineReaderWithCRLF(t *testing.T) {
	content := "Line 1\r\nLine 2\r\nLine 3"
	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	// Read first line
	line, err := lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read first line: %v", err)
	}
	if line != "Line 1" {
		t.Errorf("Expected 'Line 1', got '%s'", line)
	}

	// Read second line
	line, err = lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read second line: %v", err)
	}
	if line != "Line 2" {
		t.Errorf("Expected 'Line 2', got '%s'", line)
	}

	// Read third line
	line, err = lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read third line: %v", err)
	}
	if line != "Line 3" {
		t.Errorf("Expected 'Line 3', got '%s'", line)
	}
}

func TestLineReaderReadAll(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3\nLine 4"
	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	// Read all lines
	lines, err := lineReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read all lines: %v", err)
	}

	expectedLines := []string{"Line 1", "Line 2", "Line 3", "Line 4"}
	if len(lines) != len(expectedLines) {
		t.Errorf("Expected %d lines, got %d", len(expectedLines), len(lines))
	}

	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("Expected line %d to be '%s', got '%s'", i, expected, lines[i])
		}
	}
}

func TestLineReaderPeek(t *testing.T) {
	content := "Hello World"
	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	// Peek first 5 bytes
	data, err := lineReader.Peek(5)
	if err != nil {
		t.Fatalf("Failed to peek: %v", err)
	}

	if string(data) != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", string(data))
	}

	// Peek should not consume data, so reading should still return the same data
	line, err := lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read line after peek: %v", err)
	}

	if line != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", line)
	}
}

func TestLineReaderEmptyLines(t *testing.T) {
	content := "Line 1\n\nLine 3\n\n\nLine 6"
	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	lines, err := lineReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read all lines: %v", err)
	}

	// Empty lines should be included
	expectedLines := []string{"Line 1", "", "Line 3", "", "", "Line 6"}
	if len(lines) != len(expectedLines) {
		t.Errorf("Expected %d lines, got %d", len(expectedLines), len(lines))
	}

	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("Expected line %d to be '%s', got '%s'", i, expected, lines[i])
		}
	}
}
