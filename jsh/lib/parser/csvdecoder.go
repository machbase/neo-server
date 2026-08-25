package parser

import (
	"bytes"

	"github.com/dop251/goja"
)

// CSVDecoder is an incremental, line oriented CSV decoder backing lib/parser/csv.js.
// It splits the input on '\n' before parsing fields, which keeps the exact semantics
// of the previous JavaScript implementation (newlines inside quoted fields are not supported).
type CSVDecoder struct {
	rt               *goja.Runtime
	separator        byte
	quote            byte
	escape           byte
	commentChar      byte
	skipComments     bool
	trimLeadingSpace bool
	skipLines        int64

	carry        []byte
	fieldBuf     []byte
	skippedLines int64
	lineNumber   int64
	bytesWritten int64
	bytesRead    int64
	recordLines  []int64
}

func optByte(options map[string]any, key string, def byte) byte {
	if v, ok := options[key].(string); ok && len(v) > 0 {
		return v[0]
	}
	return def
}

// NewCSVDecoder creates an incremental CSV decoder.
func NewCSVDecoder(options map[string]any) *CSVDecoder {
	if options == nil {
		options = map[string]any{}
	}
	d := &CSVDecoder{
		separator:        optByte(options, "separator", ','),
		quote:            optByte(options, "quote", '"'),
		commentChar:      optByte(options, "commentChar", '#'),
		trimLeadingSpace: true,
	}
	d.escape = optByte(options, "escape", d.quote)
	if v, ok := options["skipComments"].(bool); ok {
		d.skipComments = v
	}
	if v, ok := options["trimLeadingSpace"].(bool); ok {
		d.trimLeadingSpace = v
	}
	if v, ok := options["skipLines"].(int64); ok {
		d.skipLines = v
	}
	return d
}

func csvChunkBytes(chunk any) []byte {
	switch v := chunk.(type) {
	case nil:
		return nil
	case string:
		return []byte(v)
	case []byte:
		return v
	case goja.ArrayBuffer:
		return v.Bytes()
	case []any:
		buf := make([]byte, 0, len(v))
		for _, it := range v {
			switch n := it.(type) {
			case int64:
				buf = append(buf, byte(n))
			case float64:
				buf = append(buf, byte(n))
			}
		}
		return buf
	default:
		return nil
	}
}

// Write feeds a chunk and returns the records of every complete line it contains.
func (d *CSVDecoder) Write(chunk any) goja.Value {
	return d.toJS(d.Decode(chunk))
}

// Flush decodes the trailing line that is not terminated by a newline.
func (d *CSVDecoder) Flush() goja.Value {
	return d.toJS(d.DecodeTail())
}

// toJS materializes plain JavaScript arrays; a Go slice would be wrapped in a
// reflection proxy that allocates a new object on every element access.
func (d *CSVDecoder) toJS(records [][]string) goja.Value {
	if d.rt == nil {
		return nil
	}
	if len(records) == 0 {
		return goja.Null()
	}
	rows := make([]any, len(records))
	for i, rec := range records {
		fields := make([]any, len(rec))
		for j, f := range rec {
			fields[j] = f
		}
		rows[i] = d.rt.NewArray(fields...)
	}
	return d.rt.NewArray(rows...)
}

// Decode feeds a chunk and returns the records of every complete line it contains.
func (d *CSVDecoder) Decode(chunk any) [][]string {
	in := csvChunkBytes(chunk)
	d.bytesWritten += int64(len(in))
	d.recordLines = d.recordLines[:0]

	var data []byte
	if len(d.carry) > 0 {
		data = make([]byte, 0, len(d.carry)+len(in))
		data = append(data, d.carry...)
		data = append(data, in...)
		d.carry = d.carry[:0]
	} else {
		data = in
	}

	var records [][]string
	start := 0
	for start < len(data) {
		idx := bytes.IndexByte(data[start:], '\n')
		if idx < 0 {
			break
		}
		line := data[start : start+idx]
		start += idx + 1
		d.bytesRead += int64(len(line)) + 1
		if rec, ok := d.decodeLine(line); ok {
			records = append(records, rec)
			d.recordLines = append(d.recordLines, d.lineNumber)
		}
	}
	if start < len(data) {
		d.carry = append(d.carry[:0], data[start:]...)
	}
	return records
}

// DecodeTail decodes the trailing line that is not terminated by a newline.
func (d *CSVDecoder) DecodeTail() [][]string {
	d.recordLines = d.recordLines[:0]
	if len(d.carry) == 0 {
		return nil
	}
	line := d.carry
	d.carry = nil
	d.bytesRead += int64(len(line))
	if len(bytes.TrimSpace(line)) == 0 {
		return nil
	}
	rec, ok := d.decodeLine(line)
	if !ok {
		return nil
	}
	d.recordLines = append(d.recordLines, d.lineNumber)
	return [][]string{rec}
}

func (d *CSVDecoder) BytesWritten() int64 { return d.bytesWritten }
func (d *CSVDecoder) BytesRead() int64    { return d.bytesRead }
func (d *CSVDecoder) LineNumber() int64   { return d.lineNumber }

// RecordLines returns the physical line number of each record of the last Write/Flush call.
func (d *CSVDecoder) RecordLines() []int64 { return d.recordLines }

func (d *CSVDecoder) decodeLine(line []byte) ([]string, bool) {
	d.lineNumber++
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	if d.skippedLines < d.skipLines {
		d.skippedLines++
		return nil, false
	}
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false
	}
	if d.skipComments && trimmed[0] == d.commentChar {
		return nil, false
	}
	return d.parseFields(line), true
}

func trimLeadingSpaceBytes(f []byte) []byte {
	i := 0
	for i < len(f) {
		switch f[i] {
		case ' ', '\t', '\r', '\v', '\f':
			i++
			continue
		}
		break
	}
	return f[i:]
}

func (d *CSVDecoder) parseFields(line []byte) []string {
	if bytes.IndexByte(line, d.quote) < 0 {
		fields := make([]string, 0, bytes.Count(line, []byte{d.separator})+1)
		start := 0
		for i := 0; i <= len(line); i++ {
			if i < len(line) && line[i] != d.separator {
				continue
			}
			f := line[start:i]
			if d.trimLeadingSpace {
				f = trimLeadingSpaceBytes(f)
			}
			fields = append(fields, string(f))
			start = i + 1
		}
		return fields
	}

	fields := make([]string, 0, bytes.Count(line, []byte{d.separator})+1)
	buf := d.fieldBuf[:0]
	inQuotes := false
	for i := 0; i < len(line); {
		c := line[i]
		if inQuotes {
			switch {
			case c == d.escape && i+1 < len(line) && line[i+1] == d.quote:
				buf = append(buf, d.quote)
				i += 2
			case c == d.quote:
				inQuotes = false
				i++
			default:
				buf = append(buf, c)
				i++
			}
			continue
		}
		switch c {
		case d.quote:
			inQuotes = true
			i++
		case d.separator:
			fields = append(fields, d.finishField(buf))
			buf = buf[:0]
			i++
		default:
			buf = append(buf, c)
			i++
		}
	}
	fields = append(fields, d.finishField(buf))
	d.fieldBuf = buf[:0]
	return fields
}

func (d *CSVDecoder) finishField(buf []byte) string {
	if d.trimLeadingSpace {
		buf = trimLeadingSpaceBytes(buf)
	}
	return string(buf)
}
