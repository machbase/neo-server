package maps

import (
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
)

func OUTPUT(outstream spec.OutputStream, sink any) (any, error) {
	switch sink := sink.(type) {
	case *Encoder:
		codecOpts := []opts.Option{
			opts.AssetHost("/web/echarts/"),
			opts.OutputStream(outstream),
		}
		codecOpts = append(codecOpts, sink.opts...)
		return codec.NewEncoder(sink.format, codecOpts...), nil
	case DatabaseSink:
		sink.SetOutputStream(outstream)
		return sink, nil
	default:
		return nil, conv.ErrWrongTypeOfArgs("OUTPUT", 1, "encoder or dbsink", sink)
	}
}

type DatabaseSink interface {
	Open(db spi.Database) error
	Close()
	AddRow([]any) error

	SetOutputStream(spec.OutputStream)
}

var _ DatabaseSink = &insert{}

var _ DatabaseSink = &appender{}

type Encoder struct {
	format string
	opts   []opts.Option
}
