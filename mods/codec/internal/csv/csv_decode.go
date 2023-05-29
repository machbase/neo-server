package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Decoder struct {
	reader       *csv.Reader
	columnTypes  []string
	translator   transcoder.Transcoder
	comma        rune
	heading      bool
	input        spec.InputStream
	timeformat   string
	timeLocation *time.Location
	tableName    string
	columns      spi.Columns
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

func (dec *Decoder) SetHeading(skipHeading bool) {
	dec.heading = skipHeading
}

func (dec *Decoder) SetDelimiter(delimiter string) {
	delmiter, _ := utf8.DecodeRuneInString(delimiter)
	dec.comma = delmiter
}

func (dec *Decoder) SetTable(tableName string) {
	dec.tableName = tableName
}

func (dec *Decoder) SetTranscoder(trans transcoder.Transcoder) {
	dec.translator = trans
}

func (dec *Decoder) SetColumns(cols spi.Columns) {
	dec.columns = cols
}

func (dec *Decoder) Open() {
	dec.reader = csv.NewReader(dec.input)
	dec.reader.Comma = dec.comma
	dec.columnTypes = dec.columns.Types()

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
	lastField := len(fields) - 1
	for i, field := range fields {
		if i == lastField && runtime.GOOS == "windows" {
			// on windows, the last field contains the trailing white spaces
			// in case of using pipe like `echo name,time,3.14 | machbase-neo shell import...`
			field = strings.TrimSpace(field)
		}
		switch dec.columnTypes[i] {
		case "string":
			values[i] = field
		case "datetime":
			values[i], err = util.ParseTime(field, dec.timeformat, dec.timeLocation)
			if err != nil {
				return nil, err
			}
		case "double":
			if values[i], err = strconv.ParseFloat(field, 64); err != nil {
				values[i] = math.NaN()
			}
		case "int":
			if values[i], err = strconv.ParseInt(field, 10, 32); err != nil {
				values[i] = math.NaN()
			}
		case "int32":
			if values[i], err = strconv.ParseInt(field, 10, 32); err != nil {
				values[i] = math.NaN()
			}
		case "int64":
			if values[i], err = strconv.ParseInt(field, 10, 64); err != nil {
				values[i] = math.NaN()
			}
		default:
			return nil, fmt.Errorf("unsupported column type; %s", dec.columnTypes[i])
		}
	}
	if dec.translator != nil {
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
	return values, nil
}
