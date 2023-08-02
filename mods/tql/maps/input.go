package maps

import (
	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
)

type FakeSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()

	implFakeSource()
}

var (
	_ FakeSource = &meshgrid{}
	_ FakeSource = &linspace{}
	_ FakeSource = &sphere{}
	_ FakeSource = &oscillator{}
)

func (*meshgrid) implFakeSource()   {}
func (*linspace) implFakeSource()   {}
func (*sphere) implFakeSource()     {}
func (*oscillator) implFakeSource() {}

type ReaderSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

var (
	_ ReaderSource = &bytesSource{}
	_ ReaderSource = &csvSource{}
)

type DatabaseSource interface {
	ToSQL() string
}

var (
	_ DatabaseSource = &Sql{}
	_ DatabaseSource = &Query{}
)

// Deprecated: no more required
func INPUT(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("INPUT", 1, len(args))
	}
	return args[0], nil
}
