package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/dop251/goja"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewCSVReader", NewCSVReader)
	m.Set("NewLineReader", NewLineReader)
}

// CSVReader wraps Go's csv.Reader
type CSVReader struct {
	reader *csv.Reader
}

// NewCSVReader creates a new CSV reader
func NewCSVReader(reader io.Reader, options map[string]interface{}) *CSVReader {
	csvReader := csv.NewReader(reader)

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
		reader: csvReader,
	}
}

// Read reads one CSV record (row)
func (r *CSVReader) Read() ([]string, error) {
	return r.reader.Read()
}

// ReadAll reads all remaining CSV records
func (r *CSVReader) ReadAll() ([][]string, error) {
	return r.reader.ReadAll()
}

// LineReader reads lines from a reader
type LineReader struct {
	reader io.Reader
	buffer *bytes.Buffer
	eof    bool
}

// NewLineReader creates a new line reader
func NewLineReader(reader io.Reader) *LineReader {
	return &LineReader{
		reader: reader,
		buffer: &bytes.Buffer{},
		eof:    false,
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
			return line, nil
		}

		// If we reached EOF and have remaining data, return it
		if r.eof {
			if r.buffer.Len() > 0 {
				line := r.buffer.String()
				r.buffer.Reset()
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
