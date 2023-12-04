package tql

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type DatabaseSink interface {
	Open(task *Task) error
	Close() (string, error)
	AddRow([]any) error
}

var (
	_ DatabaseSink = &insert{}
	_ DatabaseSink = &appender{}
)

type Encoder struct {
	format string
	opts   []opts.Option
}

func (e *Encoder) RowEncoder(args ...opts.Option) codec.RowsEncoder {
	e.opts = append(args, e.opts...)
	ret := codec.NewEncoder(e.format, e.opts...)
	return ret
}

type output struct {
	task *Task
	name string

	src chan *Record

	encoder codec.RowsEncoder
	dbSink  DatabaseSink
	isChart bool

	closeWg     sync.WaitGroup
	lastError   error
	lastMessage string

	pragma []*Line
}

func (node *Node) compileSink(code string) (ret *output, err error) {
	defer func() {
		// panic case: if the 'code' is not applicable as SINK
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = fmt.Errorf("unable to apply to SINK: %s %s", code, e.Error())
			} else {
				err = fmt.Errorf("unable to apply to SINK: %s", code)
			}
		}
	}()
	expr, err := node.Parse(code)
	if err != nil {
		return nil, err
	}
	sink, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	ret = &output{}
	switch val := sink.(type) {
	case *Encoder:
		if val == nil {
			return nil, errors.New("no encoder found")
		}
		ret.encoder = val.RowEncoder(
			opts.Logger(node.task),
			opts.OutputStream(node.task.OutputWriter()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(node.task.toJsonOutput),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
	case DatabaseSink:
		ret.dbSink = val
	default:
		return nil, fmt.Errorf("type (%T) is not applicable for SINK", val)
	}
	ret.name = asNodeName(expr)
	ret.task = node.task
	ret.src = make(chan *Record)
	return ret, nil
}

func (out *output) start() {
	out.closeWg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				out.task.LogErrorf("panic %s %v\n%s", out.name, r, w.String())
			}
		}()

		shouldClose := false
	loop:
		for {
			select {
			case <-out.task.ctx.Done():
				// task has been cancelled.
				break loop
			case rec := <-out.src:
				if rec.IsEOF() || rec.IsCircuitBreak() {
					break loop
				} else if rec.IsError() {
					out.lastError = rec.Error()
					continue
				}
				if !shouldClose {
					resultColumns := out.task.ResultColumns()
					if len(resultColumns) == 0 {
						arr := rec.Flatten()
						for i, v := range arr {
							resultColumns = append(resultColumns,
								&spi.Column{
									Name: fmt.Sprintf("column%d", i-1),
									Type: out.columnTypeName(v),
								})
						}
					}
					out.setHeader(resultColumns[1:])
					if err := out.openEncoder(); err != nil {
						out.lastError = err
						out.task.LogErrorf(err.Error())
					}
					shouldClose = true
				}
				if rec.IsArray() {
					for _, v := range rec.Array() {
						if err := out.addRow(v); err != nil {
							out.task.LogErrorf(err.Error())
						}
					}
				} else if rec.IsTuple() {
					if err := out.addRow(rec); err != nil {
						out.task.LogErrorf(err.Error())
					}
				} else if rec.IsImage() {
					if err := out.addRow(rec); err != nil {
						out.task.LogErrorf(err.Error())
					}
				}
			}
		}

		if shouldClose {
			out.closeEncoder()
		} else {
			// encoder has not been opened, which means no records are produced
			if resultColumns := out.task.ResultColumns(); len(resultColumns) > 0 {
				out.setHeader(resultColumns[1:])
				out.openEncoder()
				out.closeEncoder()
			}
		}
		out.closeWg.Done()
	}()
}

func (out *output) Name() string {
	return out.name
}

func (out *output) Receive(rec *Record) {
	out.src <- rec
}

func (out *output) stop() {
	if out.src != nil {
		close(out.src)
	}
	out.closeWg.Wait()
}

func (out *output) setHeader(cols spi.Columns) {
	if out.encoder != nil {
		codec.SetEncoderColumns(out.encoder, cols)
	}
}

func (out *output) ContentType() string {
	if out.encoder != nil {
		return out.encoder.ContentType()
	} else if out.dbSink != nil {
		return "application/json"
	}
	return "application/octet-stream"
}

func (out *output) IsChart() bool {
	return out.isChart
}

func (out *output) ContentEncoding() string {
	//ex: return "gzip" for  Content-Encoding: gzip
	return ""
}

func (out *output) openEncoder() error {
	if out.encoder != nil {
		return out.encoder.Open()
	} else if out.dbSink != nil {
		return out.dbSink.Open(out.task)
	} else {
		return errors.New("no output encoder")
	}
}

func (out *output) closeEncoder() {
	if out.encoder != nil {
		out.encoder.Close()
	} else if out.dbSink != nil {
		resultMessage, err := out.dbSink.Close()
		if out.lastError == nil && err != nil {
			out.lastError = err
		}
		out.lastMessage = resultMessage
	}
}

func (out *output) addRow(rec *Record) error {
	var addfunc func([]any) error
	if out.encoder != nil {
		addfunc = out.encoder.AddRow
	} else if out.dbSink != nil {
		addfunc = out.dbSink.AddRow
	} else {
		return fmt.Errorf("%s has no destination", out.name)
	}

	if rec.IsArray() {
		for _, r := range rec.Array() {
			out.addRow(r)
		}
		return nil
	} else if rec.IsImage() && rec.Value() != nil {
		value := rec.Value()
		if raw, ok := value.([]byte); ok {
			return addfunc([]any{rec.contentType, raw})
		} else {
			return fmt.Errorf("%s can not write invalid image data (%T)", out.name, value)
		}
	} else if !rec.IsTuple() {
		return fmt.Errorf("%s can not write %v", out.name, rec)
	}

	if value := rec.Value(); value != nil {
		switch v := value.(type) {
		case [][]any:
			var err error
			for n := range v {
				err = addfunc(v[n])
				if err != nil {
					break
				}
			}
			return err
		case []any:
			return addfunc(v)
		case any:
			return addfunc([]any{v})
		}
	}
	return nil
}

func (out *output) columnTypeName(v any) string {
	switch v.(type) {
	default:
		return fmt.Sprintf("%T", v)
	case string:
		return "string"
	case *time.Time:
		return "datetime"
	case time.Time:
		return "datetime"
	case *float32:
		return "float"
	case float32:
		return "float"
	case *float64:
		return "double"
	case float64:
		return "double"
	}
}
