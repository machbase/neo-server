package tql

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	codecOpts "github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util"
	"golang.org/x/text/encoding"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

func (x *Node) fmCsv(args ...any) (any, error) {
	isSource := false
	if len(args) > 0 {
		switch args[0].(type) {
		case *FilePath:
			isSource = true
		case io.Reader:
			isSource = true
		case string:
			isSource = true
		case []byte:
			isSource = true
		}
	}
	if isSource {
		ret, err := newCsvSource(args...)
		if err != nil {
			return nil, err
		}
		ret.gen(x)
		return nil, err
	} else {
		ret, err := newEncoder("csv", args...)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
}

type csvSource struct {
	columns   map[int]*columnOpt
	hasHeader bool

	srcReader   io.Reader
	srcString   string
	srcBytes    []byte
	srcFile     string
	srcHttp     string
	srcEncoding encoding.Encoding

	printer            *message.Printer
	printProgressCount int64
}

func (src *csvSource) gen(node *Node) {
	var reader *csv.Reader
	if src.srcString != "" {
		reader = csv.NewReader(bytes.NewBufferString(src.srcString))
	} else if src.srcBytes != nil {
		reader = csv.NewReader(bytes.NewBuffer(src.srcBytes))
	} else if src.srcReader != nil {
		reader = csv.NewReader(src.srcReader)
	} else if src.srcFile != "" {
		stat, err := os.Stat(src.srcFile)
		if err != nil {
			node.task.LogErrorf("Fail to read %q, %s", src.srcFile, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		if stat.IsDir() {
			node.task.LogErrorf("Fail to read %q, it is a directory", src.srcFile)
			ErrorRecord(errors.New("failed to read directory as CSV")).Tell(node.next)
			return
		}
		content, err := os.Open(src.srcFile)
		if err != nil {
			node.task.LogErrorf("Fail to read %q, %s", src.srcFile, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		defer content.Close()

		var inputReader io.Reader = content
		if strings.HasSuffix(src.srcFile, ".gz") {
			gzReader, err := gzip.NewReader(content)
			if err != nil {
				node.task.LogErrorf("Fail to create gzip reader for %q, %s", src.srcFile, err.Error())
				ErrorRecord(err).Tell(node.next)
				return
			}
			defer gzReader.Close()
			inputReader = gzReader
		}

		if src.srcEncoding != nil {
			reader = csv.NewReader(src.srcEncoding.NewDecoder().Reader(inputReader))
		} else {
			reader = csv.NewReader(inputReader)
		}
	} else if src.srcHttp != "" {
		req, err := http.NewRequestWithContext(node.task.ctx, "GET", src.srcHttp, nil)
		if err != nil {
			node.task.LogErrorf("Fail to request %q, %s", src.srcHttp, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		httpClient := node.task.NewHttpClient()
		resp, err := httpClient.Do(req)
		if err != nil {
			node.task.LogErrorf("Fail to GET %q, %s", src.srcHttp, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		defer resp.Body.Close()
		var inputReader io.Reader = resp.Body
		if strings.HasSuffix(src.srcHttp, ".gz") || resp.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				node.task.LogErrorf("Fail to create gzip reader for %q, %s", src.srcHttp, err.Error())
				ErrorRecord(err).Tell(node.next)
				return
			}
			defer gzReader.Close()
			inputReader = gzReader
		}
		if src.srcEncoding != nil {
			reader = csv.NewReader(src.srcEncoding.NewDecoder().Reader(inputReader))
		} else {
			reader = csv.NewReader(inputReader)
		}
	}
	if reader == nil {
		node.task.LogErrorf("CSV() no input is specified")
		return
	}

	rownum := 0
	headerProcessed := false
	for {
		fields, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				node.task.LogErrorf("CSV() invalid input, %s", err.Error())
			}
			return
		}
		if len(fields) == 0 {
			node.task.LogError("CSV() invalid input")
			return
		}
		if !headerProcessed {
			if src.hasHeader {
				for i, label := range fields {
					if _, ok := src.columns[i]; !ok {
						src.columns[i] = &columnOpt{idx: i, dataType: &stringOpt{}, label: label}
					}
				}
			} else {
				for i := range fields {
					if _, ok := src.columns[i]; !ok {
						src.columns[i] = &columnOpt{idx: i, dataType: &stringOpt{}, label: fmt.Sprintf("column%d", i)}
					}
				}
			}
			headerProcessed = true // done processing header
			node.task.SetResultColumns(src.header())
			if src.hasHeader {
				continue
			}
		}
		values := make([]any, len(fields))
		for i := 0; i < len(fields); i++ {
			colOpt := src.columns[i]
			if colOpt == nil {
				values[i] = fields[i]
				continue
			}
			switch dataType := colOpt.dataType.(type) {
			case *anyOpt:
				switch dataType.typeName {
				case "float", "float32", "float64", "double":
					dataType.typeName = "double"
					values[i], err = strconv.ParseFloat(fields[i], 64)
				case "bool", "boolean":
					dataType.typeName = "boolean"
					values[i], err = strconv.ParseBool(fields[i])
				case "string":
					values[i] = fields[i]
				default:
					values[i] = fields[i]
				}
			case *stringOpt:
				values[i] = fields[i]
			case *boolOpt:
				values[i], err = strconv.ParseBool(fields[i])
			case *doubleOpt:
				values[i], err = strconv.ParseFloat(fields[i], 64)
			case *epochTimeOpt:
				var parsed int64
				parsed, err = strconv.ParseInt(fields[i], 10, 64)
				if err == nil {
					ts := parsed * dataType.unit
					values[i] = time.Unix(0, ts)
				}
			case *datetimeOpt:
				values[i], err = util.ParseTime(fields[i], dataType.timeformat, dataType.timeLocation)
			}
			if err != nil {
				node.task.LogWarnf("CSV() invalid number format (at line %d)", rownum)
				break
			}
		}
		rownum++
		if err == nil {
			NewRecord(rownum, values).Tell(node.next)
			if src.printProgressCount > 0 && int64(rownum)%src.printProgressCount == 0 {
				if src.printer == nil {
					src.printer = message.NewPrinter(language.English)
				}
				node.task.LogInfo(src.printer.Sprintf("Loading %v records", number.Decimal(rownum)))
			}
		} else {
			err = nil
		}
	}
}

// implements codec.opts.CanSetHeading
func (src *csvSource) SetHeading(has bool) {
	src.hasHeader = has
}

// implements codec.opts.CanSetHeader
func (src *csvSource) SetHeader(has bool) {
	src.hasHeader = has
}

// implements codec.opts.CanSetCharsetEncoding
func (src *csvSource) SetCharsetEncoding(enc encoding.Encoding) {
	src.srcEncoding = enc
}

func (fs *csvSource) header() api.Columns {
	if len(fs.columns) == 0 {
		return []*api.Column{api.MakeColumnRownum()}
	}
	max := 0
	for i := range fs.columns {
		if i > max {
			max = i
		}
	}
	ret := make([]*api.Column, max+2)
	ret[0] = api.MakeColumnRownum()
	for i, c := range fs.columns {
		ret[i+1] = &api.Column{Name: c.label, DataType: c.dataType.dataType()}
	}
	return ret
}

func newCsvSource(args ...any) (*csvSource, error) {
	ret := &csvSource{columns: make(map[int]*columnOpt)}

	for _, arg := range args {
		switch v := arg.(type) {
		case *FilePath:
			if v.HttpPath != "" {
				ret.srcHttp = v.HttpPath
			} else {
				ret.srcFile = v.AbsPath
			}
		case *columnOpt:
			ret.columns[v.idx] = v
		case codecOpts.Option:
			v(ret)
		case io.Reader:
			ret.srcReader = v
		case string:
			ret.srcString = v
		case PrintProgressCount:
			ret.printProgressCount = int64(v)
		case []byte:
			ret.srcBytes = v
		default:
			return nil, fmt.Errorf("f(CSV) unknown argument, %T", v)
		}
	}

	return ret, nil
}

type PrintProgressCount int64

func (x *Node) fmLogProgress(args ...any) (any, error) {
	if len(args) != 1 {
		return PrintProgressCount(500_000), nil // default 500K
	}
	if v, ok := args[0].(float64); ok {
		return PrintProgressCount(v), nil
	}
	return 0, errors.New("f(printProgressCount) argument should be int")
}

type columnOpt struct {
	idx      int
	dataType colOpt
	label    string
}

// Deprecated: use ToField() instead
func (x *Node) fmCol(args ...any) (any, error) {
	fmt.Println("WARN col() is deprecated. use field() instead")
	return x.fmField(args...)
}

func (x *Node) fmField(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, ErrInvalidNumOfArgs("field", 3, len(args))
	}
	col := &columnOpt{}
	if d, ok := args[0].(float64); ok {
		col.idx = int(d)
	} else {
		return nil, errors.New("f(field) first argument should be int")
	}

	if v, ok := args[1].(colOpt); ok {
		col.dataType = v
	} else {
		if str, ok := args[1].(string); ok {
			col.dataType = &anyOpt{typeName: str}
		} else {
			return nil, errors.New("f(field) second argument should be data type")
		}
	}

	if str, ok := args[2].(string); ok {
		col.label = str
	} else {
		return nil, errors.New("f(field) third argument should be label")
	}

	return col, nil
}

type colOpt interface {
	dataType() api.DataType
}

type anyOpt struct {
	typeName string
}

func (o *anyOpt) dataType() api.DataType { return api.DataTypeAny }

type stringOpt struct{}

func (o *stringOpt) dataType() api.DataType { return api.DataTypeString }

func (x *Node) fmStringType(args ...any) (any, error) {
	return &stringOpt{}, nil
}

type doubleOpt struct{}

func (o *doubleOpt) dataType() api.DataType { return api.DataTypeFloat64 }

func (x *Node) fmDoubleType(args ...any) (any, error) {
	return &doubleOpt{}, nil
}

type boolOpt struct{}

func (o *boolOpt) dataType() api.DataType { return api.DataTypeBoolean }

func (x *Node) fmBoolType(args ...any) (any, error) {
	return &boolOpt{}, nil
}

type epochTimeOpt struct {
	unit int64
}

func (o *epochTimeOpt) dataType() api.DataType { return api.DataTypeDatetime }

type datetimeOpt struct {
	timeformat   string
	timeLocation *time.Location
}

func (o *datetimeOpt) dataType() api.DataType { return api.DataTypeDatetime }

func (x *Node) fmDatetimeType(args ...any) (any, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, ErrInvalidNumOfArgs("datetime", 2, len(args))
	}
	var err error
	ret := &datetimeOpt{timeformat: "ns", timeLocation: time.UTC}
	if ret.timeformat, err = convString(args, 0, "datetime", "string"); err != nil {
		return ret, err
	}
	switch ret.timeformat {
	case "ns":
		return &epochTimeOpt{unit: 1}, nil
	case "us":
		return &epochTimeOpt{unit: 1000}, nil
	case "ms":
		return &epochTimeOpt{unit: 1000000}, nil
	case "s":
		return &epochTimeOpt{unit: 1000000000}, nil
	}

	if len(args) == 2 {
		var tz string
		if tz, err = convString(args, 1, "datetime", "string"); err != nil {
			return ret, err
		} else {
			if timeLocation, err := util.ParseTimeLocation(tz, nil); err == nil {
				ret.timeLocation = timeLocation
			} else {
				return nil, err
			}
		}
	}
	return ret, nil
}
