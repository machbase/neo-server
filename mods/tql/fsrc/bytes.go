package fsrc

import (
	"bufio"
	"io"
	"sync"

	"github.com/machbase/neo-server/mods/tql/conv"
	spi "github.com/machbase/neo-spi"
)

var _ readerSource = &bytesSrc{}

type bytesSrc struct {
	toString  bool
	delimiter byte

	reader    io.Reader
	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (src *bytesSrc) Gen() <-chan []any {
	src.ch = make(chan []any)
	src.alive = true
	src.closeWait.Add(1)
	buff := bufio.NewReader(src.reader)
	num := 0
	go func() {
		for {
			var str any
			var err error
			if src.toString {
				str, err = buff.ReadString(src.delimiter)
			} else {
				str, err = buff.ReadBytes(src.delimiter)
			}
			if err != nil && err != io.EOF {
				break
			}
			src.ch <- []any{num, str}
			num++
			if err == io.EOF {
				break
			}

		}
		close(src.ch)
		src.closeWait.Done()
	}()
	return src.ch
}

func (src *bytesSrc) Stop() {
	src.alive = false
	src.closeWait.Wait()
}

func (src *bytesSrc) Header() spi.Columns {
	return []*spi.Column{{
		Name: "string", Type: spi.ColumnBufferTypeString,
	}}
}

func src_STRING(args ...any) (any, error) {
	ret := &bytesSrc{toString: true}
	if v, err := conv.Reader(args, 0, "STRING", "io.Reader"); err != nil {
		return nil, err
	} else {
		ret.reader = v
	}
	return ret, nil
}

func src_BYTES(args ...any) (any, error) {
	ret := &bytesSrc{toString: false}
	if v, err := conv.Reader(args, 0, "BYTES", "io.Reader"); err != nil {
		return nil, err
	} else {
		ret.reader = v
	}
	return ret, nil
}
