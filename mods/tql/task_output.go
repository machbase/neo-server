package tql

import (
	"fmt"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

func (x *Task) compileSink(code string) (*output, error) {
	expr, err := x.parseSink(code)
	if err != nil {
		return nil, err
	}
	sink, err := expr.Eval(x)
	if err != nil {
		return nil, err
	}

	switch val := sink.(type) {
	case *Encoder:
		ret := &output{}
		ret.encoder = val.RowEncoder(
			opts.OutputStream(x.OutputStream()),
			opts.AssetHost("/web/echarts/"),
			opts.ChartJson(x.ShouldJsonOutput()),
		)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		}
		return ret, nil
	case DatabaseSink:
		ret := &output{}
		ret.dbSink = val
		ret.dbSink.SetOutputStream(x.OutputStream())
		return ret, nil
	default:
		return nil, fmt.Errorf("%T is not applicable for OUTPUT", val)
	}
}

type output struct {
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
