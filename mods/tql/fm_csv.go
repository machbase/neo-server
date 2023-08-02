package tql

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

func fmCsv(args ...any) (any, error) {
	isSource := false
	if len(args) > 0 {
		switch args[0].(type) {
		case *FilePath:
			isSource = true
		case io.Reader:
			isSource = true
		case string:
			isSource = true
		}
	}
	if isSource {
		ret, err := newCsvSource(args...)
		return ret, err
	} else {
		ret := newEncoder("csv", args...)
		return ret, nil
	}
}

type csvSource struct {
	fd        io.ReadCloser
	columns   map[int]*columnOpt
	hasHeader bool

	reader    *csv.Reader
	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (src *csvSource) Gen() <-chan []any {
	src.ch = make(chan []any)
	src.alive = true
	src.closeWait.Add(1)
	go func() {
		rownum := 0
		for src.alive {
			fields, err := src.reader.Read()
			if len(fields) == 0 || err != nil {
				break
			}
			if rownum == 0 && src.hasHeader {
				continue // skip header
			}
			values := make([]any, len(fields))
			for i := 0; i < len(fields); i++ {
				colOpt := src.columns[i]
				if colOpt != nil {
					switch dataType := colOpt.dataType.(type) {
					case *anyOpt:
						switch dataType.typeName {
						case "float", "float32", "float64", "double":
							dataType.typeName = "double"
							values[i], err = strconv.ParseFloat(fields[i], 64)
							if err != nil {
								src.ch <- nil
								break
							}
						case "bool", "boolean":
							dataType.typeName = "boolean"
							values[i], err = strconv.ParseBool(fields[i])
							if err != nil {
								src.ch <- nil
								break
							}
						case "string":
							values[i] = fields[i]
						default:
							values[i] = fields[i]
						}
					case *stringOpt:
						values[i] = fields[i]
					case *doubleOpt:
						values[i], err = strconv.ParseFloat(fields[i], 64)
						if err != nil {
							src.ch <- nil
							break
						}
					case *epochtimeOpt:
						if t, err := strconv.ParseInt(fields[i], 10, 64); err != nil {
							src.ch <- nil
							break
						} else {
							values[i] = t * dataType.unit
						}
					case *datetimeOpt:
						values[i], err = util.ParseTime(fields[i], dataType.timeformat, dataType.timeLocation)
						if err != nil {
							src.ch <- nil
							break
						}
					}
				} else {
					values[i] = fields[i]
				}
			}
			rownum++
			src.ch <- values
		}
		close(src.ch)
		src.closeWait.Done()
	}()
	return src.ch
}

func (src *csvSource) Stop() {
	src.alive = false
	src.closeWait.Wait()
	if src.fd != nil {
		src.fd.Close()
	}
}

func (fs *csvSource) Header() spi.Columns {
	if len(fs.columns) == 0 {
		return []*spi.Column{}
	}
	max := 0
	for i := range fs.columns {
		if i > max {
			max = i
		}
	}
	ret := make([]*spi.Column, max+1)
	for i, c := range fs.columns {
		ret[i] = &spi.Column{Name: c.label, Type: c.dataType.spiType()}
	}

	return ret
}

func newCsvSource(args ...any) (*csvSource, error) {
	ret := &csvSource{columns: make(map[int]*columnOpt)}

	var file *FilePath
	var reader io.Reader

	for _, arg := range args {
		switch v := arg.(type) {
		case *FilePath:
			file = v
		case *columnOpt:
			ret.columns[v.idx] = v
		case *headerOpt:
			ret.hasHeader = v.hasHeader
		case io.Reader:
			reader = v
		case string:
			reader = bytes.NewBufferString(v)
		default:
			return nil, fmt.Errorf("f(CSV) unknown argument, %T", v)
		}
	}

	if file == nil && reader == nil {
		return nil, errors.New("f(CSV) file path or data reader is not specified")
	}

	if reader != nil {
		ret.reader = csv.NewReader(reader)
	} else if file != nil {
		stat, err := os.Stat(file.AbsPath)
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			return nil, errors.New("f(CSV) file path is a directory")
		}

		if fd, err := os.Open(file.AbsPath); err != nil {
			return nil, err
		} else {
			ret.fd = fd
		}
		ret.reader = csv.NewReader(ret.fd)
	}

	return ret, nil
}

type headerOpt struct {
	hasHeader bool
}

func ToHeader(args ...any) (any, error) {
	flag, err := convBool(args, 0, "header", "boolean")
	if err != nil {
		return nil, err
	}
	return &headerOpt{hasHeader: flag}, nil
}

type columnOpt struct {
	idx      int
	dataType colOpt
	label    string
}

// Deprecated: use ToField() instead
func fmCol(args ...any) (any, error) {
	fmt.Println("WARN col() is deprecated. use field() instead")
	return fmField(args...)
}

func fmField(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, ErrInvalidNumOfArgs("col", 3, len(args))
	}
	col := &columnOpt{}
	if d, ok := args[0].(float64); ok {
		col.idx = int(d)
	} else {
		return nil, errors.New("f(col) first argument should be int")
	}

	if v, ok := args[1].(colOpt); ok {
		col.dataType = v
	} else {
		if str, ok := args[1].(string); ok {
			col.dataType = &anyOpt{typeName: str}
		} else {
			return nil, errors.New("f(col) second argument should be data type")
		}
	}

	if str, ok := args[2].(string); ok {
		col.label = str
	} else {
		return nil, errors.New("f(col) third argument should be label")
	}

	return col, nil
}

type colOpt interface {
	spiType() string
}

type anyOpt struct {
	typeName string
}

func (o *anyOpt) spiType() string { return o.typeName }

type stringOpt struct{}

func (o *stringOpt) spiType() string { return "string" }

func fmStringType(args ...any) (any, error) {
	return &stringOpt{}, nil
}

type doubleOpt struct{}

func (o *doubleOpt) spiType() string { return "double" }

func fmDoubleType(args ...any) (any, error) {
	return &doubleOpt{}, nil
}

type epochtimeOpt struct {
	unit int64
}

func (o *epochtimeOpt) spiType() string { return "datetime" }

type datetimeOpt struct {
	timeformat   string
	timeLocation *time.Location
}

func (o *datetimeOpt) spiType() string { return "datetime" }

func fmDatetimeType(args ...any) (any, error) {
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
		return &epochtimeOpt{unit: 1}, nil
	case "us":
		return &epochtimeOpt{unit: 1000}, nil
	case "ms":
		return &epochtimeOpt{unit: 1000000}, nil
	case "s":
		return &epochtimeOpt{unit: 1000000000}, nil
	}

	if len(args) == 2 {
		var tz string
		if tz, err = convString(args, 1, "datetime", "string"); err != nil {
			return ret, err
		} else {
			switch strings.ToUpper(tz) {
			case "UTC":
				tz = "UTC"
			case "LOCAL":
				tz = "Local"
			case "GMT":
				tz = "GMT"
			}
			loc, err := time.LoadLocation(tz)
			if err == nil {
				ret.timeLocation = loc
			} else {
				ret.timeLocation, err = util.GetTimeLocation(tz)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return ret, nil
}
