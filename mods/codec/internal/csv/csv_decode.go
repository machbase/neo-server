package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Decoder struct {
	reader      *csv.Reader
	columnTypes []string
	ctx         *spi.RowsDecoderContext
	translator  transcoder.Transcoder
}

func NewDecoder(ctx *spi.RowsDecoderContext, delimiter string, heading bool, translator transcoder.Transcoder) spi.RowsDecoder {
	delmiter, _ := utf8.DecodeRuneInString(delimiter)

	rr := &Decoder{ctx: ctx}
	rr.reader = csv.NewReader(ctx.Reader)
	rr.reader.Comma = delmiter
	rr.columnTypes = ctx.Columns.Types()
	rr.translator = translator

	if heading { // skip first line
		rr.reader.Read()
	}
	return rr
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
			len(fields), dec.ctx.TableName, len(dec.columnTypes))
	}

	values := make([]any, len(dec.columnTypes))
	for i, field := range fields {
		// on windows, trailing white spaces remains
		// when using pipe like `echo n,t,3.14 | machbase-neo shell import...`
		field = strings.TrimSpace(field)
		switch dec.columnTypes[i] {
		case "string":
			values[i] = field
		case "datetime":
			values[i], err = util.ParseTime(field, dec.ctx.TimeFormat, dec.ctx.TimeLocation)
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
