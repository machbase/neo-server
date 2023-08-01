package maps

import spi "github.com/machbase/neo-spi"

type FakeSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()

	implFakeSource()
}

var _ FakeSource = &meshgrid{}
var _ FakeSource = &linspace{}
var _ FakeSource = &sphere{}
var _ FakeSource = &oscillator{}

func (*meshgrid) implFakeSource()   {}
func (*linspace) implFakeSource()   {}
func (*sphere) implFakeSource()     {}
func (*oscillator) implFakeSource() {}

type ReaderSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

var _ ReaderSource = &bytesSource{}

type DatabaseSource interface {
	ToSQL() string
}

var _ DatabaseSource = &Sql{}

var _ DatabaseSource = &Query{}
