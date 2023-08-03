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
		ret.encoder = val.RowEncoder(
			opts.OutputStream(node.task.OutputStream()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(node.task.ShouldJsonOutput()),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
	case DatabaseSink:
		ret.dbSink = val
		ret.dbSink.SetOutputStream(node.task.OutputStream())
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
}

func (out *output) SetHeader(cols spi.Columns) {
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

func (out *output) AddRow(vals []any) error {
	if out.encoder != nil {
		return out.encoder.AddRow(vals)
	} else if out.dbSink != nil {
		return out.dbSink.AddRow(vals)
	}
	return errors.New("no output encoder")
}

func (out *output) Open(db spi.Database) error {
	if out.encoder != nil {
		return out.encoder.Open()
	} else if out.dbSink != nil {
		return out.dbSink.Open(db)
	}
	return errors.New("no output encoder")
}

func (out *output) Close() {
	if out.encoder != nil {
		out.encoder.Close()
	} else if out.dbSink != nil {
		out.dbSink.Close()
	}
}
