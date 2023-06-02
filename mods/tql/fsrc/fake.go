package fsrc

import (
	spi "github.com/machbase/neo-spi"
)

type fakeSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

/*
INPUT(

	FAKE(
	    oscilator(
	            range(time('now','-10s'), '10s', '1ms'),
	            freq(100, amplitude [,phase [, bias]]),
	            freq(240, amplitude [,phase [, bias]]),
	        )
	    )
	)
*/
func src_FAKE(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, errInvalidNumOfArgs("FAKE", 1, len(args))
	}
	if gen, ok := args[0].(fakeSource); !ok {
		return gen, nil
	} else {
		return nil, errWrongTypeOfArgs("FAKE", 0, "fakeSource", args[0])
	}
}

func src_oscilator(args ...any) (any, error) {
	ret := oscilator{}

	return ret, nil
}

type oscilator struct {
	timeRange *timeRange
}

var _ fakeSource = &oscilator{}

func (fs *oscilator) Header() spi.Columns {
	return []*spi.Column{}
}

func (fs *oscilator) Gen() <-chan []any {
	return nil
}

func (fs *oscilator) Stop() {

}

type freq struct {
	hertz     float64
	amplitude float64
	phase     float64
	bias      float64
}

// freq(240, amplitude [,phase [, bias]])
func srcf_freq(args ...any) (any, error) {
	ret := &freq{}
	return ret, nil
}
