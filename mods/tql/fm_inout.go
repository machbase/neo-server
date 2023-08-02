package tql

import (
	"fmt"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type ChannelSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

var (
	// fake sources
	_ ChannelSource = &meshgrid{}
	_ ChannelSource = &linspace{}
	_ ChannelSource = &sphere{}
	_ ChannelSource = &oscillator{}
	// reader sources
	_ ChannelSource = &bytesSource{}
	_ ChannelSource = &csvSource{}
)

type DatabaseSource interface {
	ToSQL() string
}

var (
	_ DatabaseSource = &sqlSource{}
	_ DatabaseSource = &querySource{}
)

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

// Deprecated: no more required
func (x *task) fmINPUT(args ...any) (any, error) {
	fmt.Println("WARN INPUT() is deprecated. no more need to use")
	if len(args) != 1 {
		return nil, ErrInvalidNumOfArgs("INPUT", 1, len(args))
	}
	return args[0], nil
}

// Deprecated: no more required
func (x *task) fmOUTPUT(args ...any) (any, error) {
	fmt.Println("WARN OUTPUT() is deprecated. no more need to use")
	if len(args) != 1 {
		return nil, ErrInvalidNumOfArgs("OUTPUT", 1, len(args))
	}
	return args[0], nil
}
