package tql

import (
	"fmt"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

func (node *Node) compileSink(code string) (*output, error) {
	expr, err := node.Parse(code)
	if err != nil {
		return nil, err
	}
	sink, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	ret := &output{}
	switch val := sink.(type) {
	case *Encoder:
		ret.encoder = val.RowEncoder(
			opts.OutputStream(node.task.OutputWriter()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(node.task.toJsonOutput),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
	case DatabaseSink:
		ret.dbSink = val
		ret.dbSink.SetOutputStream(node.task.OutputWriter())
	default:
		return nil, fmt.Errorf("%T is not applicable for OUTPUT", val)
	}
	ret.selfNode = node
	ret.name = code
	return ret, nil
}

type output struct {
	log      TaskLog
	selfNode *Node
	name     string
	db       spi.Database

	encoder codec.RowsEncoder
	dbSink  DatabaseSink
	isChart bool

	// result data
	resultColumns spi.Columns

	closeWg   sync.WaitGroup
	encoderCh chan *Record

	lastError error
}

func (out *output) start() {
	out.encoderCh = make(chan *Record)
	out.closeWg.Add(1)
	go func() {
		defer func() {
			out.closeWg.Done()
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					out.log.LogErrorf(err.Error())
				}
			}
		}()

		encoderNeedToClose := false
		for rec := range out.encoderCh {
			if rec.IsEOF() || rec.IsCircuitBreak() {
				break
			} else if rec.IsError() {
				out.lastError = rec.Error()
				continue
			}

			if !encoderNeedToClose {
				if len(out.resultColumns) == 0 {
					arr := rec.Flatten()
					for i, v := range arr {
						out.resultColumns = append(out.resultColumns,
							&spi.Column{
								Name: fmt.Sprintf("C%02d", i),
								Type: out.columnTypeName(v),
							})
					}
				}
				out.setHeader(out.resultColumns)
				if err := out.openEncoder(); err != nil {
					out.log.LogErrorf(err.Error())
				}
				encoderNeedToClose = true
			}

			if rec.IsArray() {
				for _, v := range rec.Array() {
					if err := out.addRow(v); err != nil {
						out.log.LogErrorf(err.Error())
					}
				}
			} else if rec.IsTuple() {
				if err := out.addRow(rec); err != nil {
					out.log.LogErrorf(err.Error())
				}
			}
		}

		if encoderNeedToClose {
			if out.encoder != nil {
				out.encoder.Close()
			} else if out.dbSink != nil {
				out.dbSink.Close()
			}
		}
	}()
}

func (out *output) Name() string {
	return out.name
}

func (out *output) Receive(rec *Record) {
	out.encoderCh <- rec
}

func (out *output) stop() {
	close(out.encoderCh)
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
		return out.dbSink.Open(out.db)
	} else {
		return errors.New("no output encoder")
	}
}

func (out *output) addRow(rec *Record) error {
	if rec.IsArray() {
		for _, r := range rec.Array() {
			out.addRow(r)
		}
		return nil
	} else if !rec.IsTuple() {
		return fmt.Errorf("OUTPUT can not write %v", rec)
	}

	var addfunc func([]any) error
	if out.encoder != nil {
		addfunc = out.encoder.AddRow
	} else if out.dbSink != nil {
		addfunc = out.dbSink.AddRow
	} else {
		return errors.New("OUTPUT has no destination")
	}

	switch v := rec.Value().(type) {
	case [][]any:
		for n := range v {
			addfunc(append([]any{rec.Key()}, v[n]...))
		}
	case []any:
		addfunc(append([]any{rec.Key()}, v...))
	case any:
		addfunc([]any{rec.Key(), v})
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
