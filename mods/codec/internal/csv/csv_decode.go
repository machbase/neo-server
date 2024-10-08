package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	"golang.org/x/text/encoding"
)

type Decoder struct {
	reader        *csv.Reader
	columnTypes   []string
	columnNames   []string
	comma         rune
	heading       bool
	headerColumns bool
	input         spec.InputStream
	timeformat    string
	timeLocation  *time.Location
	tableName     string
	charset       encoding.Encoding

	headerNames []string
	headerTypes []string
	headerErr   error
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) SetInputStream(in spec.InputStream) {
	dec.input = in
}

func (dec *Decoder) SetCharsetEncoding(charset encoding.Encoding) {
	dec.charset = charset
}

func (dec *Decoder) SetTimeformat(format string) {
	dec.timeformat = format
}

func (dec *Decoder) SetTimeLocation(tz *time.Location) {
	dec.timeLocation = tz
}

// Deprecated use SetHeader()
func (dec *Decoder) SetHeading(skipHeading bool) {
	dec.heading = skipHeading
}

func (dec *Decoder) SetHeader(skipHeader bool) {
	dec.heading = skipHeader
}

func (dec *Decoder) SetHeaderColumns(headerColumns bool) {
	dec.headerColumns = headerColumns
}

func (dec *Decoder) SetDelimiter(newDelimiter string) {
	delimiter, _ := utf8.DecodeRuneInString(newDelimiter)
	dec.comma = delimiter
}

func (dec *Decoder) SetTableName(tableName string) {
	dec.tableName = tableName
}

func (dec *Decoder) SetColumns(names ...string) {
	dec.columnNames = names
}

func (dec *Decoder) SetColumnTypes(types ...string) {
	dec.columnTypes = types
}

func (dec *Decoder) Open() {
	if dec.charset == nil {
		dec.reader = csv.NewReader(dec.input)
	} else {
		dec.reader = csv.NewReader(dec.charset.NewDecoder().Reader(dec.input))
	}
	if dec.comma != 0 {
		dec.reader.Comma = dec.comma
	}
	if dec.timeformat == "" {
		dec.timeformat = "ns"
	}
	if dec.timeLocation == nil {
		dec.timeLocation = time.UTC
	}

	if dec.heading { // if the first row is a header
		if header, _ := dec.reader.Read(); dec.headerColumns {
			dec.headerNames = header
		}
	}

	if len(dec.headerNames) <= len(dec.columnNames) {
		if dec.heading && dec.headerColumns {
			for _, colName := range dec.headerNames {
				colName = strings.ToUpper(colName)
				if colIdx := slices.Index(dec.columnNames, colName); colIdx >= 0 {
					dec.headerTypes = append(dec.headerTypes, dec.columnTypes[colIdx])
				} else {
					dec.headerErr = fmt.Errorf("CSV header '%s' not found in columns of table %q", colName, dec.tableName)
					break
				}
			}
		} else {
			dec.headerNames = dec.columnNames
			dec.headerTypes = dec.columnTypes
		}
	}
}

func (dec *Decoder) NextRow() ([]any, []string, error) {
	if dec.reader == nil {
		return nil, nil, io.EOF
	}
	if dec.headerErr != nil {
		return nil, nil, dec.headerErr
	}

	fields, err := dec.reader.Read()
	if err != nil {
		return nil, nil, err
	}

	if len(dec.headerTypes) > 0 && len(fields) != len(dec.headerTypes) {
		return nil, nil, fmt.Errorf("too many columns (%d); CSV header has %d fields",
			len(fields), len(dec.columnTypes))
	} else if len(fields) > len(dec.columnTypes) {
		return nil, nil, fmt.Errorf("too many columns (%d); table '%s' has %d columns",
			len(fields), dec.tableName, len(dec.columnTypes))
	}

	values := make([]any, 0, len(dec.columnTypes))
	errs := []error{}

	lastFieldOnWindows := len(fields) - 1
	if runtime.GOOS != "windows" {
		lastFieldOnWindows = -1
	}

	for i, field := range fields {
		if i == lastFieldOnWindows {
			// on windows, the last field contains the trailing white spaces
			// in case of using pipe like `echo name,time,3.14 | machbase-neo shell import...`
			field = strings.TrimSpace(field)
		}

		var value any
		var columnType string
		if len(dec.headerTypes) > 0 {
			columnType = dec.headerTypes[i]
		} else {
			columnType = dec.columnTypes[i]
		}

		switch columnType {
		case mach.DB_COLUMN_TYPE_VARCHAR, mach.DB_COLUMN_TYPE_JSON, mach.DB_COLUMN_TYPE_TEXT, "string":
			value = field
		case mach.DB_COLUMN_TYPE_DATETIME:
			value, err = util.ParseTime(field, dec.timeformat, dec.timeLocation)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_FLOAT:
			value, err = util.ParseFloat32(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_DOUBLE:
			value, err = util.ParseFloat64(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_LONG, "int64":
			value, err = util.ParseInt64(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_ULONG:
			value, err = util.ParseUint64(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_INTEGER, "int":
			value, err = util.ParseInt(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_UINTEGER:
			value, err = util.ParseUint(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_SHORT, "int16":
			value, err = util.ParseInt16(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_USHORT:
			value, err = util.ParseUint16(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "int32":
			value, err = util.ParseInt32(field)
			if err != nil {
				errs = append(errs, err)
			}
		case mach.DB_COLUMN_TYPE_IPV4, mach.DB_COLUMN_TYPE_IPV6:
			value, err = util.ParseIP(field)
			if err != nil {
				errs = append(errs, err)
			}
		default:
			return nil, nil, fmt.Errorf("unsupported column type; %s", dec.columnTypes[i])
		}
		values = append(values, value)
	}
	if len(errs) > 0 {
		return values, nil, errs[0]
	}
	if len(dec.headerNames) > 0 {
		return values, dec.headerNames, nil
	} else {
		return values, nil, nil
	}
}
