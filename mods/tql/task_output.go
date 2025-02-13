package tql

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
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
	format      string
	opts        []opts.Option
	cacheOption *CacheOption
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

	encoder  codec.RowsEncoder
	dbSink   DatabaseSink
	isChart  bool
	isGeoMap bool

	closeWg     sync.WaitGroup
	lastError   error
	lastMessage string

	cacheOption *CacheOption
	cacheWriter *bytes.Buffer
	cachedData  []byte

	pragma  []*Line
	tqlLine *Line
}

func (node *Node) compileSink(code *Line) (ret *output, err error) {
	defer func() {
		// panic case: if the 'code' is not applicable as SINK
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = fmt.Errorf("unable to apply to SINK: %s %s", code.text, e.Error())
				debug.PrintStack()
			} else {
				err = fmt.Errorf("unable to apply to SINK: %s", code.text)
			}
		}
	}()
	expr, err := node.Parse(code.text)
	if err != nil {
		return nil, err
	}
	node.name = asNodeName(expr)
	sink, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	if sink == nil {
		if code.text == "" {
			return nil, errors.New("NULL is not applicable for SINK")
		} else {
			return nil, fmt.Errorf("%q is not applicable for SINK", code.text)
		}
	}
	ret = &output{}
	switch val := sink.(type) {
	case *Encoder:
		if val == nil {
			return nil, errors.New("no encoder found")
		}
		var writer io.Writer = node.task.OutputWriter()
		// check cache option
		if val.cacheOption != nil && tqlResultCache != nil {
			ret.cacheOption = val.cacheOption
			if item := tqlResultCache.Get(val.cacheOption.key); item != nil {
				// get cached data
				ret.cachedData = item.Data
				// check preemptive update is set and valid
				if preemptiveUpdateRatio := val.cacheOption.preemptiveUpdate; preemptiveUpdateRatio > 0 && preemptiveUpdateRatio < 1 {
					// check if the cache is required to be updated in advance
					preemptiveTTL := time.Duration(float64(item.TTL) * (1 - preemptiveUpdateRatio))
					preemptiveUpdateAt := item.ExpiresAt.Add(-1 * preemptiveTTL)
					if preemptiveUpdateAt.Before(time.Now()) {
						if u := item.updates.Add(1); u == 1 {
							// update cache
							ret.cacheWriter = &bytes.Buffer{}
							writer = io.MultiWriter(ret.cacheWriter)
						}
					}
				}
			} else {
				ret.cacheWriter = &bytes.Buffer{}
				writer = io.MultiWriter(ret.cacheWriter, node.task.OutputWriter())
			}
		}

		options := []opts.Option{
			opts.Logger(node.task),
			opts.OutputStream(writer),
			opts.ChartJson(node.task.toJsonOutput),
			opts.GeoMapJson(node.task.toJsonOutput),
			opts.VolatileFileWriter(node.task),
		}
		ret.encoder = val.RowEncoder(options...)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		} else if _, ok := ret.encoder.(opts.CanSetGeoMapJson); ok {
			ret.isGeoMap = true
		}
	case DatabaseSink:
		ret.dbSink = val
	default:
		return nil, fmt.Errorf("type (%T) is not applicable for SINK", val)
	}
	ret.name = asNodeName(expr)
	ret.task = node.task
	ret.tqlLine = code
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
		saneEncoder := true
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
				if !shouldClose && saneEncoder {
					resultColumns := out.task.ResultColumns()
					if len(resultColumns) == 0 {
						arr := rec.Flatten()
						for i, v := range arr {
							resultColumns = append(resultColumns,
								&api.Column{
									Name:     fmt.Sprintf("column%d", i-1),
									DataType: api.DataTypeOf(v),
								})
						}
					}
					out.setHeader(resultColumns[1:])
					if err := out.openEncoder(); err == nil {
						// success to open sink encoder
						shouldClose = true
						saneEncoder = true
					} else {
						// fail to open sink encoder
						out.lastError = err
						out.task.LogError(err.Error())
						out.task.fireCircuitBreak(nil)
						saneEncoder = false
					}
				}
				if !saneEncoder {
					continue
				}
				if rec.IsArray() {
					for _, v := range rec.Array() {
						if err := out.addRow(v); err != nil {
							out.task.LogError(err.Error())
						}
					}
				} else if rec.IsTuple() {
					if err := out.addRow(rec); err != nil {
						out.task.LogError(err.Error())
					}
				} else if rec.IsImage() {
					if err := out.addRow(rec); err != nil {
						out.task.LogError(err.Error())
					}
				}
			}
		}
		if shouldClose && saneEncoder {
			out.closeEncoder()
		} else if saneEncoder {
			// encoder has not been opened, which means no records are produced
			if resultColumns := out.task.ResultColumns(); len(resultColumns) > 0 {
				out.setHeader(resultColumns[1:])
				if err := out.openEncoder(); err == nil {
					out.closeEncoder()
				}
			}
		}

		if out.cacheOption != nil && out.cacheOption.key != "" && out.cacheWriter != nil && tqlResultCache != nil {
			if data := out.cacheWriter.Bytes(); len(data) > 0 {
				tqlResultCache.Set(out.cacheOption.key, data, out.cacheOption.ttl)
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

func (out *output) setHeader(cols api.Columns) {
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

func (out *output) HttpHeaders() map[string][]string {
	if out.encoder != nil {
		return out.encoder.HttpHeaders()
	} else if out.dbSink != nil {
		return nil
	}
	return nil
}

func (out *output) IsChart() bool {
	return out.isChart
}

func (out *output) IsGeoMap() bool {
	return out.isGeoMap
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
	var addFunc func([]any) error
	if out.encoder != nil {
		addFunc = out.encoder.AddRow
	} else if out.dbSink != nil {
		addFunc = out.dbSink.AddRow
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
			return addFunc([]any{rec.contentType, raw})
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
				err = addFunc(v[n])
				if err != nil {
					break
				}
			}
			return err
		case []any:
			return addFunc(v)
		case any:
			return addFunc([]any{v})
		}
	}
	return nil
}
