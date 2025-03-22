package ndjson

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
)

type Decoder struct {
	reader       *json.Decoder
	columnTypes  []api.DataType
	columnNames  []string
	nrow         int64
	input        io.Reader
	timeformat   string
	timeLocation *time.Location
	tableName    string
	values       []any
	columns      []string
	jsonObj      map[string]any
	columnUppers []string
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

func (dec *Decoder) SetColumns(names ...string) {
	dec.columnNames = names
	dec.columnUppers = make([]string, len(names))
	for i, name := range names {
		dec.columnUppers[i] = strings.ToUpper(name)
	}
}

func (dec *Decoder) SetColumnTypes(types ...api.DataType) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
}

func (dec *Decoder) NextRow() ([]any, []string, error) {
	if dec.reader == nil {
		dec.reader = json.NewDecoder(dec.input)
		dec.reader.UseNumber()
	}

	// decode json into a map
	if dec.jsonObj == nil {
		dec.jsonObj = map[string]any{}
	}
	for k := range dec.jsonObj {
		delete(dec.jsonObj, k)
	}
	err := dec.reader.Decode(&dec.jsonObj)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range dec.jsonObj {
		dec.jsonObj[strings.ToUpper(k)] = v
	}
	dec.nrow++

	// clear and reuse slices
	dec.values = make([]any, 0, len(dec.columnNames))     // dec.values[:0]
	dec.columns = make([]string, 0, len(dec.columnNames)) //dec.columns[:0]

	for idx, colName := range dec.columnNames {
		field, ok := dec.jsonObj[dec.columnUppers[idx]]
		if !ok {
			continue
		}
		dec.columns = append(dec.columns, colName)
		var value any
		if field == nil {
			dec.values = append(dec.values, nil)
			continue
		}
		value, err = dec.columnTypes[idx].Apply(field, dec.timeformat, dec.timeLocation)
		if err != nil {
			return nil, nil, fmt.Errorf("rows[%d] field[%s] is not a %s, but %T", dec.nrow, colName, dec.columnTypes[idx], field)
		}
		dec.values = append(dec.values, value)
	}
	retVals, retCols := dec.values, dec.columns
	return retVals, retCols, nil
}
