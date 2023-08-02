package maps

import (
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
)

// Deprecated: no more required
func OUTPUT(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("OUTPUT", 1, len(args))
	}
	return args[0], nil
}

type DatabaseSink interface {
	Open(db spi.Database) error
	Close()
	AddRow([]any) error

	SetOutputStream(spec.OutputStream)
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
	e.opts = append(e.opts, args...)
	ret := codec.NewEncoder(e.format, e.opts...)
	return ret
}
