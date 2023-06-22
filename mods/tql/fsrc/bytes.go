package fsrc

import (
	"bufio"
	"bytes"
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
				var v string
				if v, err = buff.ReadString(src.delimiter); len(v) == 0 {
					break
				} else {
					str = v
				}
			} else {
				var v []byte
				if v, err = buff.ReadBytes(src.delimiter); len(v) == 0 {
					break
				} else {
					str = v
				}
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

func src_bytes(typ string, args ...any) (any, error) {
	ret := &bytesSrc{}
	if v, err := conv.Reader(args, 0, typ, "io.Reader"); err != nil {
		if s, err := conv.String(args, 0, typ, "reader or string"); err != nil {
			return nil, err
		} else {
			ret.reader = bytes.NewBufferString(s)
		}
	} else {
		ret.reader = v
	}
	if typ == "STRING" {
		ret.toString = true
	}
	for _, arg := range args[1:] {
		switch v := arg.(type) {
		case *delimiter:
			ret.delimiter = v.c
		}
	}
	return ret, nil
}

// STRING(CTX.Body [, delimeter()])
func src_STRING(args ...any) (any, error) {
	return src_bytes("STRING", args...)
}

// BYTES(CTX.Body [, delimeter()])
func src_BYTES(args ...any) (any, error) {
	return src_bytes("BYTES", args...)
}

type delimiter struct {
	c byte
}

func srcf_delimiter(args ...any) (any, error) {
	ret := &delimiter{}
	if v, err := conv.Byte(args, 0, "delimiter", "byte"); err != nil {
		return nil, err
	} else {
		ret.c = v
	}
	return ret, nil
}
