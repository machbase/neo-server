package maps

import (
	"fmt"

	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
)

type ChannelSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

var (
	// fake
	_ ChannelSource = &meshgrid{}
	_ ChannelSource = &linspace{}
	_ ChannelSource = &sphere{}
	_ ChannelSource = &oscillator{}
	// reader
	_ ChannelSource = &bytesSource{}
	_ ChannelSource = &csvSource{}
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
	fmt.Println("WARN INPUT() is deprecated. no more need to use")
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("INPUT", 1, len(args))
	}
	return args[0], nil
}
