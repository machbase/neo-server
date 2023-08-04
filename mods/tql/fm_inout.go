package tql

import (
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type DataSource interface {
	Gen() <-chan *Record
	stop()
}

var (
	// fake sources
	_ DataSource = &meshgrid{}
	_ DataSource = &linspace{}
	_ DataSource = &sphere{}
	_ DataSource = &oscillator{}
	// reader sources
	_ DataSource = &bytesSource{}
	_ DataSource = &csvSource{}
	// database sources
	_ DataSource = &databaseSource{}
	_ DataSource = &bridgeNode{}
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
func (node *Node) fmINPUT(args ...any) (any, error) {
	node.task.LogWarnf("INPUT() is deprecated.")
	if len(args) != 1 {
		return nil, ErrInvalidNumOfArgs("INPUT", 1, len(args))
	}
	return args[0], nil
}

// Deprecated: no more required
func (node *Node) fmOUTPUT(args ...any) (any, error) {
	node.task.LogWarnf("OUTPUT() is deprecated.")
	if len(args) != 1 {
		return nil, ErrInvalidNumOfArgs("OUTPUT", 1, len(args))
	}
	return args[0], nil
}
