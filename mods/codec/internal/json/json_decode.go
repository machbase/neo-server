package json

import (
	gojson "encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/stream/spec"
	"github.com/pkg/errors"
)

type Decoder struct {
	columnTypes  []api.DataType
	reader       *gojson.Decoder
	dataDepth    int
	nrow         int64
	input        spec.InputStream
	timeformat   string
	timeLocation *time.Location
	tableName    string
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) SetInputStream(in spec.InputStream) {
	dec.input = in
}

func (dec *Decoder) SetTimeformat(format string) {
	dec.timeformat = format
}

func (dec *Decoder) SetTimeLocation(tz *time.Location) {
	dec.timeLocation = tz
}

func (dec *Decoder) SetTableName(tableName string) {
	dec.tableName = tableName
}

func (dec *Decoder) SetColumnTypes(types ...api.DataType) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
}

func (dec *Decoder) NextRow() ([]any, []string, error) {
	fields, err := dec.nextRow0()
	if err != nil {
		return nil, nil, err
	}

	dec.nrow++

	if len(fields) != len(dec.columnTypes) {
		return nil, nil, fmt.Errorf("rows[%d] number of columns not matched (%d); table '%s' has %d columns",
			dec.nrow, len(fields), dec.tableName, len(dec.columnTypes))
	}

	values := make([]any, len(dec.columnTypes))
	for i, field := range fields {
		if field == nil {
			values[i] = nil
			continue
		}
		values[i], err = dec.columnTypes[i].Apply(field, dec.timeformat, dec.timeLocation)
		if err != nil {
			return nil, nil, fmt.Errorf("rows[%d] column[%d] is not a %s, but %T", dec.nrow, i, dec.columnTypes[i], field)
		}
	}
	return values, nil, nil
}

func (dec *Decoder) nextRow0() ([]any, error) {
	if dec.reader == nil {
		dec.reader = gojson.NewDecoder(dec.input)
		dec.reader.UseNumber()
		// find first '{'
		if tok, err := dec.reader.Token(); err != nil {
			return nil, err
		} else {
			delim, ok := tok.(gojson.Delim)
			if !ok {
				return nil, errors.New("missing top level delimiter")
			}

			if delim == '{' {
				// find "data" field
				found := false
				for {
					if tok, err := dec.reader.Token(); err != nil {
						return nil, err
					} else if key, ok := tok.(string); ok && key == "data" {
						found = true
						break
					}
				}
				if !found {
					return nil, errors.New("'data' field not found")
				}
				// find "rows" field
				found = false
				for {
					if tok, err := dec.reader.Token(); err != nil {
						return nil, err
					} else if key, ok := tok.(string); ok && key == "rows" {
						found = true
						break
					}
				}
				// find data's array '['
				if tok, err := dec.reader.Token(); err != nil {
					return nil, err
				} else if delim, ok := tok.(gojson.Delim); !ok || delim != '[' {
					return nil, errors.New("'data' field should be an array")
				}
				dec.dataDepth = 1
			} else if delim == '[' {
				// top level is '[', means rows only format
				dec.dataDepth = 1
			} else {
				return nil, errors.New("invalid top level delimiter")
			}
		}
	}

	if dec.dataDepth == 0 {
		return nil, io.EOF
	}

	tuple := make([]any, 0)
	for dec.reader.More() {
		tok, err := dec.reader.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := tok.(gojson.Delim); ok {
			if delim == '[' {
				dec.dataDepth++
			} else if delim == '{' {
				return nil, fmt.Errorf("invalid data format at %d", dec.reader.InputOffset())
			}
			tuple = tuple[:0]
			continue
		} else {
			// append element of tuple
			tuple = append(tuple, tok)
		}
	}

	tok, err := dec.reader.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(gojson.Delim); ok {
		if delim == ']' {
			dec.dataDepth--
		}
	} else {
		return nil, fmt.Errorf("invalid syntax at %d", dec.reader.InputOffset())
	}

	if len(tuple) == 0 {
		return nil, io.EOF
	}
	return tuple, nil
}
