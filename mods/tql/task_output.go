package tql

import (
	"fmt"

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
		fmt.Println("xxxxxxxxxx")
		ret.encoder = val.RowEncoder(
			opts.OutputStream(node.task.OutputWriter()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(node.task.ShouldJsonOutput()),
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
	return ret, nil
}

type output struct {
	selfNode *Node

	encoder codec.RowsEncoder
	dbSink  DatabaseSink
	isChart bool

	// result data
	resultColumns spi.Columns

	encoderCh chan []any
}

func (out *output) start() {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				out.selfNode.LogErrorf(err.Error())
			}
		}
	}()

	out.encoderCh = make(chan []any)
	encoderNeedToClose := false
	for arr := range out.encoderCh {
		if !encoderNeedToClose {
			if len(out.resultColumns) == 0 {
				for i, v := range arr {
					out.resultColumns = append(out.resultColumns,
						&spi.Column{
							Name: fmt.Sprintf("C%02d", i),
							Type: out.selfNode.task.columnTypeName(v),
						})
				}
			}
			out.setHeader(out.resultColumns)
			if err := out.openEncoder(); err != nil {
				out.selfNode.LogErrorf(err.Error())
			}
			encoderNeedToClose = true
		}
		if len(arr) == 0 {
			continue
		}
		if rec, ok := arr[0].(*Record); ok && rec.IsEOF() {
			continue
		}
		if err := out.addRow(arr); err != nil {
			out.selfNode.LogErrorf(err.Error())
		}
	}

	if encoderNeedToClose {
		if out.encoder != nil {
			fmt.Println("before close")
			out.encoder.Close()
			fmt.Println("after close")
		} else if out.dbSink != nil {
			out.dbSink.Close()
		}
	}
}

func (out *output) stop() {
	close(out.encoderCh)
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
		return out.dbSink.Open(out.selfNode.task.db)
	} else {
		return errors.New("no output encoder")
	}
}

func (out *output) addRow(vals []any) error {
	if out.encoder != nil {
		return out.encoder.AddRow(vals)
	} else if out.dbSink != nil {
		return out.dbSink.AddRow(vals)
	}
	return errors.New("no output encoder")
}
