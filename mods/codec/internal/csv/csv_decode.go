package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
	"golang.org/x/text/encoding"
)

type Decoder struct {
	reader       *csv.Reader
	columnTypes  []string
	columnNames  []string
	translator   transcoder.Transcoder
	comma        rune
	heading      bool
	input        spec.InputStream
	timeformat   string
	timeLocation *time.Location
	tableName    string
	charset      encoding.Encoding
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

func (dec *Decoder) SetDelimiter(delimiter string) {
	delmiter, _ := utf8.DecodeRuneInString(delimiter)
	dec.comma = delmiter
}

func (dec *Decoder) SetTableName(tableName string) {
	dec.tableName = tableName
}

func (dec *Decoder) SetTranscoder(trans transcoder.Transcoder) {
	dec.translator = trans
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
	dec.reader.Comma = dec.comma

	if dec.heading { // skip first line
		dec.reader.Read()
	}
}

func (dec *Decoder) NextRow() ([]any, error) {
	if dec.reader == nil {
		return nil, io.EOF
	}

	fields, err := dec.reader.Read()
	if err != nil {
		return nil, err
	}

	if len(fields) > len(dec.columnTypes) {
		return nil, fmt.Errorf("too many columns (%d); table '%s' has %d columns",
			len(fields), dec.tableName, len(dec.columnTypes))
	}

	values := make([]any, len(dec.columnTypes))
	errs := []error{}

	lastField := len(fields) - 1
	for i, field := range fields {
		if i == lastField && runtime.GOOS == "windows" {
			// on windows, the last field contains the trailing white spaces
			// in case of using pipe like `echo name,time,3.14 | machbase-neo shell import...`
			field = strings.TrimSpace(field)
		}
		switch dec.columnTypes[i] {
		case "varchar":
			values[i] = field
		case "string":
			values[i] = field
		case "datetime":
			values[i], err = util.ParseTime(field, dec.timeformat, dec.timeLocation)
			if err != nil {
				errs = append(errs, err)
			}
		case "float":
			values[i], err = util.ParseFloat32(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "double":
			values[i], err = util.ParseFloat64(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "int":
			values[i], err = util.ParseInt(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "int16":
			values[i], err = util.ParseInt16(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "int32":
			values[i], err = util.ParseInt32(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "int64":
			values[i], err = util.ParseInt64(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "ipv4":
			values[i], err = util.ParseIP(field)
			if err != nil {
				errs = append(errs, err)
			}
		case "ipv6":
			values[i], err = util.ParseIP(field)
			if err != nil {
				errs = append(errs, err)
			}
		default:
			return nil, fmt.Errorf("unsupported column type; %s", dec.columnTypes[i])
		}
	}
	if len(errs) == 0 && dec.translator != nil {
		result, err := dec.translator.Process(values)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("transcoder internal error '%T'", dec.translator))
		}
		if conv, ok := result.([]any); !ok {
			return nil, errors.Wrap(err, fmt.Sprintf("transcoder returns invalid type '%T'", result))
		} else {
			values = conv
		}
	}
	if len(errs) > 0 {
		return values, errs[0]
	}
	return values, nil
}
