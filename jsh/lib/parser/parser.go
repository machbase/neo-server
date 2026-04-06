package parser

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/dop251/goja"
)

//go:embed parser.js
var parser_js []byte

//go:embed csv.js
var csv_js []byte

//go:embed ndjson.js
var ndjson_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"parser/index.js":  parser_js,
		"parser/csv.js":    csv_js,
		"parser/ndjson.js": ndjson_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewCSVReader", NewCSVReader)
	m.Set("NewLineReader", NewLineReader)
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read += int64(n)
	}
	return n, err
}

// CSVReader wraps Go's csv.Reader
type CSVReader struct {
	reader      *csv.Reader
	counter     *countingReader
	decodedRead int64
}

// NewCSVReader creates a new CSV reader
func NewCSVReader(reader io.Reader, options map[string]interface{}) *CSVReader {
	counter := &countingReader{reader: reader}
	csvReader := csv.NewReader(counter)

	// Apply options
	if separator, ok := options["separator"].(string); ok && len(separator) > 0 {
		csvReader.Comma = rune(separator[0])
	}
	if comment, ok := options["comment"].(string); ok && len(comment) > 0 {
		csvReader.Comment = rune(comment[0])
	}
	if fieldsPerRecord, ok := options["fieldsPerRecord"].(int64); ok {
		csvReader.FieldsPerRecord = int(fieldsPerRecord)
	} else {
		csvReader.FieldsPerRecord = -1 // Variable number of fields
	}
	if lazyQuotes, ok := options["lazyQuotes"].(bool); ok {
		csvReader.LazyQuotes = lazyQuotes
	} else {
		csvReader.LazyQuotes = true
	}
	if trimLeadingSpace, ok := options["trimLeadingSpace"].(bool); ok {
		csvReader.TrimLeadingSpace = trimLeadingSpace
	}
	if reuseRecord, ok := options["reuseRecord"].(bool); ok {
		csvReader.ReuseRecord = reuseRecord
	}

	return &CSVReader{
		reader:  csvReader,
		counter: counter,
	}
}

// Read reads one CSV record (row)
func (r *CSVReader) Read() ([]string, error) {
	rec, err := r.reader.Read()
	if err != nil {
		return rec, err
	}
	for i, field := range rec {
		r.decodedRead += int64(len(field))
		if i < len(rec)-1 {
			r.decodedRead++
		}
	}
	return rec, nil
}

// ReadAll reads all remaining CSV records
func (r *CSVReader) ReadAll() ([][]string, error) {
	records, err := r.reader.ReadAll()
	if err != nil {
		return records, err
	}
	for _, rec := range records {
		for i, field := range rec {
			r.decodedRead += int64(len(field))
			if i < len(rec)-1 {
				r.decodedRead++
			}
		}
	}
	return records, nil
}

func (r *CSVReader) BytesWritten() int64 {
	if r.counter == nil {
		return 0
	}
	return r.counter.read
}

func (r *CSVReader) BytesRead() int64 {
	return r.decodedRead
}

// LineReader reads lines from a reader
type LineReader struct {
	reader      io.Reader
	counter     *countingReader
	buffer      *bytes.Buffer
	eof         bool
	decodedRead int64
}

// NewLineReader creates a new line reader
func NewLineReader(reader io.Reader) *LineReader {
	counter := &countingReader{reader: reader}
	return &LineReader{
		reader:  counter,
		counter: counter,
		buffer:  &bytes.Buffer{},
		eof:     false,
	}
}

// ReadLine reads a single line from the reader
// Returns the line (without newline character) and an error
func (r *LineReader) ReadLine() (string, error) {
	if r.eof && r.buffer.Len() == 0 {
		return "", io.EOF
	}

	for {
		// Check if we have a complete line in the buffer
		if line, found := r.scanLine(); found {
			r.decodedRead += int64(len(line))
			return line, nil
		}

		// If we reached EOF and have remaining data, return it
		if r.eof {
			if r.buffer.Len() > 0 {
				line := r.buffer.String()
				r.buffer.Reset()
				r.decodedRead += int64(len(line))
				return line, nil
			}
			return "", io.EOF
		}

		// Read more data
		chunk := make([]byte, 4096)
		n, err := r.reader.Read(chunk)
		if n > 0 {
			r.buffer.Write(chunk[:n])
		}
		if err != nil {
			if err == io.EOF {
				r.eof = true
				// Continue to process remaining buffer
			} else {
				return "", err
			}
		}
	}
}

// scanLine scans the buffer for a complete line
// Returns the line (without newline) and whether a line was found
func (r *LineReader) scanLine() (string, bool) {
	data := r.buffer.Bytes()

	// Look for \n or \r\n
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			// Found a line
			line := string(data[:i])

			// Remove the line from buffer (including \n)
			r.buffer.Next(i + 1)

			// Remove trailing \r if present (for \r\n)
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			return line, true
		}
	}

	return "", false
}

// ReadAll reads all remaining lines
func (r *LineReader) ReadAll() ([]string, error) {
	lines := make([]string, 0)

	for {
		line, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return lines, err
		}
		lines = append(lines, line)
	}

	return lines, nil
}

// Peek returns the next n bytes without advancing the reader
func (r *LineReader) Peek(n int) ([]byte, error) {
	for r.buffer.Len() < n && !r.eof {
		chunk := make([]byte, 4096)
		read, err := r.reader.Read(chunk)
		if read > 0 {
			r.buffer.Write(chunk[:read])
		}
		if err != nil {
			if err == io.EOF {
				r.eof = true
				break
			}
			return nil, err
		}
	}

	data := r.buffer.Bytes()
	if len(data) < n {
		return data, fmt.Errorf("not enough data")
	}

	return data[:n], nil
}

func (r *LineReader) BytesWritten() int64 {
	if r.counter == nil {
		return 0
	}
	return r.counter.read
}

func (r *LineReader) BytesRead() int64 {
	return r.decodedRead
}
