package stream

import (
	"errors"
	"io"
	"strings"

	"github.com/machbase/neo-server/mods/stream/internal/fio"
	"github.com/machbase/neo-server/mods/stream/internal/pio"
	"github.com/machbase/neo-server/mods/stream/spec"
)

func NewOutputStream(output string) (out spec.OutputStream, err error) {
	var outputFields = strings.Fields(output)
	if len(outputFields) > 0 && outputFields[0] == "exec" {
		binArgs := strings.TrimSpace(strings.TrimPrefix(output, "exec"))
		out, err = pio.New(binArgs)
		if err != nil {
			return
		}
	} else {
		out, err = fio.NewOutputStream(output)
		if err != nil {
			return
		}
	}
	return
}

type WriterOutputStream struct {
	Writer io.Writer
}

func (out *WriterOutputStream) Write(buf []byte) (int, error) {
	return out.Writer.Write(buf)
}

func (out *WriterOutputStream) Flush() error {
	return nil
}

func (out *WriterOutputStream) Close() error {
	if wc, ok := out.Writer.(io.Closer); ok {
		return wc.Close()
	}
	return nil
}

func NewInputStream(input string) (in spec.InputStream, err error) {
	var inputFields = strings.Fields(input)
	if len(inputFields) > 0 && inputFields[0] == "exec" {
		return nil, errors.New("not implemented")
	} else {
		in, err = fio.NewInputStream(input)
		return
	}
}

type ReaderInputStream struct {
	Reader io.Reader
}

func (in *ReaderInputStream) Read(p []byte) (int, error) {
	return in.Reader.Read(p)
}

func (in *ReaderInputStream) Close() error {
	if rc, ok := in.Reader.(io.Closer); ok {
		return rc.Close()
	}
	return nil
}
