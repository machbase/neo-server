package json

import (
	gojson "encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
)

type Decoder struct {
	columnTypes  []string
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

func (dec *Decoder) SetColumnTypes(types ...string) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
}

func (dec *Decoder) NextRow() ([]any, error) {
	fields, err := dec.nextRow0()
	if err != nil {
		return nil, err
	}

	dec.nrow++

	if len(fields) != len(dec.columnTypes) {
		return nil, fmt.Errorf("#[%d] number of columns not matched (%d); table '%s' has %d columns",
			dec.nrow, len(fields), dec.tableName, len(dec.columnTypes))
	}

	values := make([]any, len(dec.columnTypes))
	for i, field := range fields {
		switch dec.columnTypes[i] {
		case "varchar":
			fallthrough
		case "string":
			switch v := field.(type) {
			case string:
				values[i] = v
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not a string", dec.nrow, i)
			}
		case "datetime":
			if v, ok := field.(string); ok && dec.timeformat != "" {
				if values[i], err = util.ParseTime(v, dec.timeformat, dec.timeLocation); err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not a datetime convertable, %s", dec.nrow, i, err.Error())
				}
			} else {
				var strexp string
				switch v := field.(type) {
				case float64: // json has only float type, no int
					strexp = strconv.FormatInt(int64(v), 10)
				case string:
					strexp = v
				case gojson.Number:
					if n, err := v.Int64(); err != nil {
						return nil, fmt.Errorf("#[%d] column[%d] is not a datetime convertable", dec.nrow, i)
					} else {
						strexp = strconv.FormatInt(n, 10)
					}
				default:
					return nil, fmt.Errorf("#[%d] column[%d] is not datetime convertable", dec.nrow, i)
				}
				var ts int64
				if ts, err = strconv.ParseInt(strexp, 10, 64); err != nil {
					return nil, errors.Wrapf(err, "#[%d] column[%d] is not datetime convertable", dec.nrow, i)
				}
				switch dec.timeformat {
				case "s":
					values[i] = time.Unix(ts, 0)
				case "ms":
					values[i] = time.Unix(0, ts*int64(time.Millisecond))
				case "us":
					values[i] = time.Unix(0, ts*int64(time.Microsecond))
				default: // "ns"
					values[i] = time.Unix(0, ts)
				}
			}
		case "float":
			switch v := field.(type) {
			case float64:
				values[i] = float32(v)
			case gojson.Number:
				values[i], err = v.Float64()
				if err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
			}
		case "double":
			switch v := field.(type) {
			case float64:
				values[i] = v
			case gojson.Number:
				values[i], err = v.Float64()
				if err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
			}
		case "int":
			switch v := field.(type) {
			case float64:
				values[i] = int(v)
			case gojson.Number:
				if ival, err := v.Int64(); err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				} else {
					values[i] = int(ival)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with int", dec.nrow, i)
			}
		case "int16":
			switch v := field.(type) {
			case float64:
				values[i] = int16(v)
			case gojson.Number:
				if ival, err := v.Int64(); err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				} else {
					values[i] = int16(ival)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with int32", dec.nrow, i)
			}
		case "int32":
			switch v := field.(type) {
			case float64:
				values[i] = int32(v)
			case gojson.Number:
				if ival, err := v.Int64(); err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				} else {
					values[i] = int32(ival)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with int32", dec.nrow, i)
			}
		case "int64":
			switch v := field.(type) {
			case float64:
				values[i] = int64(v)
			case gojson.Number:
				values[i], err = v.Int64()
				if err != nil {
					return nil, fmt.Errorf("#[%d] column[%d] is not compatible with double", dec.nrow, i)
				}
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with int64", dec.nrow, i)
			}
		case "ipv4":
			fallthrough
		case "ipv6":
			switch v := field.(type) {
			case string:
				addr := net.ParseIP(v)
				values[i] = addr
			default:
				return nil, fmt.Errorf("#[%d] column[%d] is not compatible with %s", dec.nrow, i, dec.columnTypes[i])
			}
		default:
			return nil, fmt.Errorf("unsupported column type; %s", dec.columnTypes[i])
		}
	}
	return values, nil
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

/*
	if format == "json" {
		result := gjson.ParseBytes(payload)
		head := result.Get("0")
		if head.IsArray() {
			// if payload contains multiple tuples
			cols, err := appender.Columns()
			if err != nil {
				peerLog.Errorf("fail to get appender columns, %s", err.Error())
				return nil
			}
			result.ForEach(func(key, value gjson.Result) bool {
				fields := value.Array()
				vals, err := convAppendColumns(fields, cols, appender.TableType())
				if err != nil {
					return false
				}
				err = appender.Append(vals...)
				if err != nil {
					peerLog.Warnf("append fail %s %d %s [%+v]", table, appender.TableType(), err.Error(), vals)
					return false
				}
				return true
			})
			return err
		} else {
			// a single tuple
			fields := result.Array()
			cols, err := appender.Columns()
			if err != nil {
				peerLog.Errorf("fail to get appender columns, %s", err.Error())
				return nil
			}
			vals, err := convAppendColumns(fields, cols, appender.TableType())
			if err != nil {
				return err
			}
			err = appender.Append(vals...)
			if err != nil {
				peerLog.Warnf("append fail %s %d %s [%+v]", table, appender.TableType(), err.Error(), vals)
				return err
			}
			return nil
		}
	} else if format == "csv" {
	}
*/
