package fsrc

import spi "github.com/machbase/neo-spi"

type fakeSource interface {
	Header() spi.Columns
	Gen() <-chan []any
}

var _ fakeSource = &oscilatorSource{}

type oscilatorSource struct {
}

func (fs *oscilatorSource) Header() spi.Columns {
	return []*spi.Column{}
}

func (fs *oscilatorSource) Gen() <-chan []any {
	return nil
}
