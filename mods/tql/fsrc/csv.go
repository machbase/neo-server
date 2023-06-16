package fsrc

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/tql/conv"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

type readerSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

var _ readerSource = &csvSrc{}

type csvSrc struct {
	fd      io.ReadCloser
	columns map[int]*columnOpt

	reader    *csv.Reader
	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (src *csvSrc) Gen() <-chan []any {
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
			values := make([]any, len(fields))
			for i := 0; i < len(fields); i++ {
				colOpt := src.columns[i]
				if colOpt != nil {
					switch dataType := colOpt.dataType.(type) {
					case *anyOpt:
						switch dataType.typeName {
						case "float":
							fallthrough
						case "float32":
							fallthrough
						case "float64":
							fallthrough
						case "double":
							dataType.typeName = "double"
							values[i], err = strconv.ParseFloat(fields[i], 64)
							if err != nil {
								src.ch <- nil
								break
							}
						case "boolean":
							fallthrough
						case "bool":
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

func (src *csvSrc) Stop() {
	src.alive = false
	src.closeWait.Wait()
	if src.fd != nil {
		src.fd.Close()
	}
}

func (fs *csvSrc) Header() spi.Columns {
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

/*
INPUT( CSV( file('./path.csv') ))
*/
func src_CSV(args ...any) (any, error) {
	ret := &csvSrc{columns: make(map[int]*columnOpt)}

	var file *fileOpt
	var reader io.Reader

	for _, arg := range args {
		switch v := arg.(type) {
		case *fileOpt:
			file = v
		case *columnOpt:
			ret.columns[v.idx] = v
		case io.Reader:
			reader = v
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
		stat, err := os.Stat(file.path)
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			return nil, errors.New("f(CSV) file path is a directory")
		}

		if fd, err := os.Open(file.path); err != nil {
			return nil, err
		} else {
			ret.fd = fd
		}
		ret.reader = csv.NewReader(ret.fd)
	}

	return ret, nil
}

type fileOpt struct {
	path string
}

func src_file(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("file", 1, len(args))
	}
	if str, ok := args[0].(string); ok {
		return &fileOpt{path: str}, nil
	} else {
		return nil, errors.New("f(file) path should be string")
	}
}

type columnOpt struct {
	idx      int
	dataType colOpt
	label    string
}

func src_col(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, conv.ErrInvalidNumOfArgs("col", 3, len(args))
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

func src_stringType(args ...any) (any, error) {
	return &stringOpt{}, nil
}

type doubleOpt struct{}

func (o *doubleOpt) spiType() string { return "double" }

func src_doubleType(args ...any) (any, error) {
	return &doubleOpt{}, nil
}

type datetimeOpt struct {
	timeformat   string
	timeLocation *time.Location
}

func (o *datetimeOpt) spiType() string { return "datetime" }

func src_datetimeType(args ...any) (any, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, conv.ErrInvalidNumOfArgs("datetime", 2, len(args))
	}
	var err error
	ret := &datetimeOpt{timeformat: "ns", timeLocation: time.UTC}
	if ret.timeformat, err = conv.String(args, 0, "datetime", "string"); err != nil {
		return ret, err
	}
	if len(args) == 2 {
		var tz string
		if tz, err = conv.String(args, 1, "datetime", "string"); err != nil {
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
