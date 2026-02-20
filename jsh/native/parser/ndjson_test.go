package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNDJSONBasic(t *testing.T) {
	// NDJSON (Newline Delimited JSON) test using LineReader
	ndjsonContent := `{"name":"Alice","age":30,"city":"New York"}
{"name":"Bob","age":25,"city":"Los Angeles"}
{"name":"Charlie","age":35,"city":"Chicago"}`

	reader := strings.NewReader(ndjsonContent)
	lineReader := NewLineReader(reader)

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	expectedPeople := []Person{
		{Name: "Alice", Age: 30, City: "New York"},
		{Name: "Bob", Age: 25, City: "Los Angeles"},
		{Name: "Charlie", Age: 35, City: "Chicago"},
	}

	parsedCount := 0
	for {
		line, err := lineReader.ReadLine()
		if err != nil {
			break
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		var person Person
		if err := json.Unmarshal([]byte(line), &person); err != nil {
			t.Fatalf("Failed to parse JSON at line %d: %v", parsedCount+1, err)
		}

		if parsedCount >= len(expectedPeople) {
			t.Fatalf("Parsed more objects than expected")
		}

		expected := expectedPeople[parsedCount]
		if person.Name != expected.Name || person.Age != expected.Age || person.City != expected.City {
			t.Errorf("Object %d mismatch: expected %+v, got %+v", parsedCount+1, expected, person)
		}

		parsedCount++
	}

	if parsedCount != len(expectedPeople) {
		t.Errorf("Expected to parse %d objects, got %d", len(expectedPeople), parsedCount)
	}
}

func TestNDJSONWithInvalidLines(t *testing.T) {
	// NDJSON with invalid JSON lines (non-strict mode simulation)
	ndjsonContent := `{"name":"Valid JSON"}
Invalid JSON line
{"name":"Another valid JSON"}`

	reader := strings.NewReader(ndjsonContent)
	lineReader := NewLineReader(reader)

	type Item struct {
		Name string `json:"name"`
	}

	validCount := 0
	invalidCount := 0
	lineNumber := 0

	for {
		line, err := lineReader.ReadLine()
		if err != nil {
			break
		}
		lineNumber++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var item Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			// Invalid JSON line - in non-strict mode, we just count and continue
			invalidCount++
			t.Logf("Warning at line %d: %v", lineNumber, err)
			continue
		}

		validCount++
		t.Logf("Parsed object %d: %+v", validCount, item)
	}

	if validCount != 2 {
		t.Errorf("Expected 2 valid objects, got %d", validCount)
	}

	if invalidCount != 1 {
		t.Errorf("Expected 1 invalid line, got %d", invalidCount)
	}
}

func TestNDJSONEmptyLines(t *testing.T) {
	// NDJSON with empty lines
	ndjsonContent := `{"id":1}

{"id":2}


{"id":3}`

	reader := strings.NewReader(ndjsonContent)
	lineReader := NewLineReader(reader)

	type Item struct {
		ID int `json:"id"`
	}

	parsedCount := 0
	expectedIDs := []int{1, 2, 3}

	for {
		line, err := lineReader.ReadLine()
		if err != nil {
			break
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var item Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if parsedCount >= len(expectedIDs) {
			t.Fatalf("Parsed more objects than expected")
		}

		if item.ID != expectedIDs[parsedCount] {
			t.Errorf("Expected ID %d, got %d", expectedIDs[parsedCount], item.ID)
		}

		parsedCount++
	}

	if parsedCount != len(expectedIDs) {
		t.Errorf("Expected to parse %d objects, got %d", len(expectedIDs), parsedCount)
	}
}

func TestNDJSONDifferentTypes(t *testing.T) {
	// NDJSON with different JSON structures
	ndjsonContent := `{"type":"person","name":"Alice","age":30}
{"type":"product","name":"Laptop","price":999.99}
{"type":"person","name":"Bob","age":25}`

	reader := strings.NewReader(ndjsonContent)
	lineReader := NewLineReader(reader)

	type BaseItem struct {
		Type string `json:"type"`
	}

	personCount := 0
	productCount := 0

	for {
		line, err := lineReader.ReadLine()
		if err != nil {
			break
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		var base BaseItem
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			t.Fatalf("Failed to parse base JSON: %v", err)
		}

		switch base.Type {
		case "person":
			personCount++
		case "product":
			productCount++
		default:
			t.Errorf("Unknown type: %s", base.Type)
		}
	}

	if personCount != 2 {
		t.Errorf("Expected 2 person objects, got %d", personCount)
	}

	if productCount != 1 {
		t.Errorf("Expected 1 product object, got %d", productCount)
	}
}

func TestNDJSONStrictMode(t *testing.T) {
	// NDJSON strict mode - should fail on invalid JSON
	ndjsonContent := `{"name":"Valid JSON"}
Invalid JSON line
{"name":"Another valid JSON"}`

	reader := strings.NewReader(ndjsonContent)
	lineReader := NewLineReader(reader)

	type Item struct {
		Name string `json:"name"`
	}

	lineNumber := 0
	strictMode := true

	for {
		line, err := lineReader.ReadLine()
		if err != nil {
			break
		}
		lineNumber++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var item Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			if strictMode {
				// In strict mode, we should fail on the first invalid line
				if lineNumber != 2 {
					t.Errorf("Expected error at line 2, but got error at line %d", lineNumber)
				}
				t.Logf("Strict mode: Failed at line %d as expected: %v", lineNumber, err)
				return // Exit test successfully
			}
		}
	}

	// If we reach here in strict mode, the test should fail
	if strictMode {
		t.Error("Strict mode should have failed on invalid JSON line")
	}
}
