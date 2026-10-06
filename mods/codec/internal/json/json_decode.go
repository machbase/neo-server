package json

import (
	gojson "encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Decoder struct {
	columnTypes  []api.DataType
	reader       *gojson.Decoder
	dataDepth    int
	pending      [][]any
	nrow         int64
	input        io.Reader
	timeformat   string
	timeLocation *time.Location
	tableName    string
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) SetInputStream(in io.Reader) {
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
		if dec.columnTypes[i] == api.DataTypeVector {
			if array, ok := field.([]any); ok {
				values[i], err = util.VectorFromJSON(array)
			} else {
				values[i], err = dec.columnTypes[i].Apply(field, dec.timeformat, dec.timeLocation)
			}
		} else {
			values[i], err = dec.columnTypes[i].Apply(field, dec.timeformat, dec.timeLocation)
		}
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
				// Keep rows-only input streaming; a scalar first item means one row.
				if !dec.reader.More() {
					if _, err := dec.reader.Token(); err != nil {
						return nil, err
					}
				} else {
					first, err := dec.reader.Token()
					if err != nil {
						return nil, err
					}
					if first == gojson.Delim('[') {
						var row []any
						for dec.reader.More() {
							var field any
							if err := dec.reader.Decode(&field); err != nil {
								return nil, err
							}
							row = append(row, field)
						}
						if end, err := dec.reader.Token(); err != nil || end != gojson.Delim(']') {
							return nil, errors.New("invalid rows-only JSON array")
						}
						dec.pending = append(dec.pending, row)
						dec.dataDepth = 1
					} else if _, ok := first.(gojson.Delim); ok {
						return nil, errors.New("invalid top level JSON row")
					} else {
						row := []any{first}
						for dec.reader.More() {
							var field any
							if err := dec.reader.Decode(&field); err != nil {
								return nil, err
							}
							row = append(row, field)
						}
						if end, err := dec.reader.Token(); err != nil || end != gojson.Delim(']') {
							return nil, errors.New("invalid single JSON row")
						}
						dec.pending = append(dec.pending, row)
					}
				}
			} else {
				return nil, errors.New("invalid top level delimiter")
			}
		}
	}
	if len(dec.pending) > 0 {
		row := dec.pending[0]
		dec.pending = dec.pending[1:]
		return row, nil
	}

	if dec.dataDepth == 0 {
		return nil, io.EOF
	}

	if dec.reader.More() {
		var tuple []any
		if err := dec.reader.Decode(&tuple); err != nil {
			return nil, err
		}
		return tuple, nil
	}
	tok, err := dec.reader.Token()
	if err != nil {
		return nil, err
	}
	if tok != gojson.Delim(']') {
		return nil, fmt.Errorf("invalid syntax at %d", dec.reader.InputOffset())
	}
	dec.dataDepth = 0
	return nil, io.EOF
}
