package parser

import (
	"strings"
	"testing"
)

func TestCSVReaderBasic(t *testing.T) {
	csvContent := `name,age,city
Alice,30,New York
Bob,25,Los Angeles
Charlie,35,Chicago`

	reader := strings.NewReader(csvContent)
	options := map[string]interface{}{}
	csvReader := NewCSVReader(reader, options)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	expectedHeader := []string{"name", "age", "city"}
	if len(header) != len(expectedHeader) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeader), len(header))
	}

	for i, h := range expectedHeader {
		if header[i] != h {
			t.Errorf("Expected header[%d] = '%s', got '%s'", i, h, header[i])
		}
	}

	// Read first row
	row, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	if row[0] != "Alice" || row[1] != "30" || row[2] != "New York" {
		t.Errorf("Unexpected first row: %v", row)
	}

	// Read second row
	row, err = csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read second row: %v", err)
	}

	if row[0] != "Bob" || row[1] != "25" || row[2] != "Los Angeles" {
		t.Errorf("Unexpected second row: %v", row)
	}

	// Read third row
	row, err = csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read third row: %v", err)
	}

	if row[0] != "Charlie" || row[1] != "35" || row[2] != "Chicago" {
		t.Errorf("Unexpected third row: %v", row)
	}
}

func TestCSVReaderWithTSV(t *testing.T) {
	tsvContent := "name\tage\tcity\nDavid\t40\tBoston\nEve\t28\tSeattle"

	reader := strings.NewReader(tsvContent)
	options := map[string]interface{}{
		"separator": "\t",
	}
	csvReader := NewCSVReader(reader, options)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if header[0] != "name" || header[1] != "age" || header[2] != "city" {
		t.Errorf("Unexpected header: %v", header)
	}

	// Read first row
	row, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	if row[0] != "David" || row[1] != "40" || row[2] != "Boston" {
		t.Errorf("Unexpected first row: %v", row)
	}

	// Read second row
	row, err = csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read second row: %v", err)
	}

	if row[0] != "Eve" || row[1] != "28" || row[2] != "Seattle" {
		t.Errorf("Unexpected second row: %v", row)
	}
}

func TestCSVReaderWithQuotedFields(t *testing.T) {
	csvContent := `name,description,price
"Product A","A great product with, comma",19.99
"Product B","Another ""quoted"" product",29.99`

	reader := strings.NewReader(csvContent)
	options := map[string]interface{}{}
	csvReader := NewCSVReader(reader, options)

	// Read header
	_, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	// Read first row
	row, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	if row[0] != "Product A" {
		t.Errorf("Expected 'Product A', got '%s'", row[0])
	}
	if row[1] != "A great product with, comma" {
		t.Errorf("Expected 'A great product with, comma', got '%s'", row[1])
	}
	if row[2] != "19.99" {
		t.Errorf("Expected '19.99', got '%s'", row[2])
	}

	// Read second row
	row, err = csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read second row: %v", err)
	}

	if row[0] != "Product B" {
		t.Errorf("Expected 'Product B', got '%s'", row[0])
	}
	if row[1] != "Another \"quoted\" product" {
		t.Errorf("Expected 'Another \"quoted\" product', got '%s'", row[1])
	}
	if row[2] != "29.99" {
		t.Errorf("Expected '29.99', got '%s'", row[2])
	}
}

func TestCSVReaderReadAll(t *testing.T) {
	csvContent := `name,age,city
Alice,30,New York
Bob,25,Los Angeles
Charlie,35,Chicago`

	reader := strings.NewReader(csvContent)
	options := map[string]interface{}{}
	csvReader := NewCSVReader(reader, options)

	// Read all records
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read all records: %v", err)
	}

	if len(records) != 4 {
		t.Errorf("Expected 4 records (including header), got %d", len(records))
	}

	// Check header
	if records[0][0] != "name" || records[0][1] != "age" || records[0][2] != "city" {
		t.Errorf("Unexpected header: %v", records[0])
	}

	// Check first data row
	if records[1][0] != "Alice" || records[1][1] != "30" || records[1][2] != "New York" {
		t.Errorf("Unexpected first row: %v", records[1])
	}
}

func TestCSVReaderWithComments(t *testing.T) {
	csvContent := `# This is a comment
name,age,city
# Another comment
Alice,30,New York
Bob,25,Los Angeles`

	reader := strings.NewReader(csvContent)
	options := map[string]interface{}{
		"comment": "#",
	}
	csvReader := NewCSVReader(reader, options)

	// Read header (comments should be skipped by csv.Reader)
	header, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if header[0] != "name" || header[1] != "age" || header[2] != "city" {
		t.Errorf("Unexpected header: %v", header)
	}

	// Read first data row
	row, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	if row[0] != "Alice" || row[1] != "30" || row[2] != "New York" {
		t.Errorf("Unexpected first row: %v", row)
	}
}

func TestCSVReaderCustomOptions(t *testing.T) {
	csvContent := `name;age;city
Alice;30;New York
Bob;25;Los Angeles`

	reader := strings.NewReader(csvContent)
	options := map[string]interface{}{
		"separator":        ";",
		"trimLeadingSpace": true,
	}
	csvReader := NewCSVReader(reader, options)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if header[0] != "name" || header[1] != "age" || header[2] != "city" {
		t.Errorf("Unexpected header: %v", header)
	}

	// Read first row
	row, err := csvReader.Read()
	if err != nil {
		t.Fatalf("Failed to read first row: %v", err)
	}

	if row[0] != "Alice" || row[1] != "30" || row[2] != "New York" {
		t.Errorf("Unexpected first row: %v", row)
	}
}

func TestLineReaderLargeBuffer(t *testing.T) {
	// Create a large string that exceeds the default buffer size (4096)
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString("This is a test line with some content. ")
	}
	largeLine := sb.String()
	content := largeLine + "\nSecond line"

	reader := strings.NewReader(content)
	lineReader := NewLineReader(reader)

	// Read first (large) line
	line, err := lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read large line: %v", err)
	}

	if line != largeLine {
		t.Errorf("Large line content mismatch (length: expected %d, got %d)", len(largeLine), len(line))
	}

	// Read second line
	line, err = lineReader.ReadLine()
	if err != nil {
		t.Fatalf("Failed to read second line: %v", err)
	}

	if line != "Second line" {
		t.Errorf("Expected 'Second line', got '%s'", line)
	}
}
