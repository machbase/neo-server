package fio

import (
	"bufio"
	"io"
	"os"
	"sync"

	"github.com/machbase/neo-server/mods/stream/spec"
)

type fout struct {
	path  string
	w     io.WriteCloser
	buf   *bufio.Writer
	mutex sync.Mutex
}

func NewOutputStream(path string) (spec.OutputStream, error) {
	out := &fout{
		path: path,
	}
	if err := out.reset(); err != nil {
		return nil, err
	}
	return out, nil
}

func (out *fout) Write(p []byte) (n int, err error) {
	if out.buf == nil {
		return 0, io.EOF
	}
	return out.buf.Write(p)
}

func (out *fout) Flush() error {
	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.buf == nil {
		return nil
	}
	return out.buf.Flush()
}

// Deprecated do not call from outside.
func (out *fout) reset() error {
	out.Close()

	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.path == "-" {
		out.w = os.Stdout
	} else {
		var err error
		out.w, err = os.OpenFile(out.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
	}
	out.buf = bufio.NewWriter(out.w)
	return nil
}

func (out *fout) Close() error {
	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.buf != nil {
		if err := out.buf.Flush(); err != nil {
			return err
		}
		out.buf = nil
	}
	if out.w != nil && out.path != "-" {
		if err := out.w.Close(); err != nil {
			return err
		}
		out.w = nil
	}
	return nil
}
