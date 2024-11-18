package fio

import (
	"io"
	"os"
	"sync"

	"github.com/machbase/neo-server/v8/mods/stream/spec"
)

type fin struct {
	path  string
	r     io.ReadCloser
	mutex sync.Mutex
}

func NewInputStream(path string) (spec.InputStream, error) {
	in := &fin{
		path: path,
	}
	if err := in.reset(); err != nil {
		return nil, err
	}
	return in, nil
}

func (in *fin) reset() error {
	in.Close()

	in.mutex.Lock()
	defer in.mutex.Unlock()

	if in.path == "-" {
		in.r = os.Stdin
	} else {
		var err error
		in.r, err = os.OpenFile(in.path, os.O_RDONLY, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func (in *fin) Read(p []byte) (int, error) {
	if in.r == nil {
		return 0, io.EOF
	}
	return in.r.Read(p)
}

func (in *fin) Close() error {
	in.mutex.Lock()
	defer in.mutex.Unlock()

	if in.r != nil && in.path != "-" {
		if err := in.r.Close(); err != nil {
			return err
		}
		in.r = nil
	}
	return nil
}
