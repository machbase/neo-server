package tql

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/machbase/neo-server/mods/util/ssfs"
	spi "github.com/machbase/neo-spi"
)

// STRING(CTX.Body | 'string' | file('path') [, separator()])
func (x *Node) fmString(origin any, args ...any) (any, error) {
	ret := &bytesSource{task: x.task, toString: true}
	err := ret.init(origin, args...)
	return ret, err
}

// BYTES(CTX.Body | 'string' | file('path') [, separator()])
func (x *Node) fmBytes(origin any, args ...any) (any, error) {
	ret := &bytesSource{task: x.task}
	err := ret.init(origin, args...)
	return ret, err
}

func (ret *bytesSource) init(origin any, args ...any) error {
	switch src := origin.(type) {
	case string:
		ret.reader = bytes.NewBufferString(src)
	case io.Reader:
		ret.reader = src
	case *FilePath:
		content, err := os.ReadFile(src.AbsPath)
		if err != nil {
			return err
		}
		ret.reader = bytes.NewBuffer(content)
	default:
		return ErrArgs("BYTES", 0, "reader or string")
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case *separator:
			ret.delimiter = v.c
		}
	}
	return nil
}

type FilePath struct {
	AbsPath string
}

func (x *Node) fmFile(path string) (*FilePath, error) {
	serverFs := ssfs.Default()
	if serverFs == nil {
		return nil, os.ErrNotExist
	}
	realPath, err := serverFs.RealPath(path)
	if err != nil {
		return nil, err
	}
	return &FilePath{AbsPath: realPath}, nil
}

type separator struct {
	c byte
}

func (x *Node) fmSeparator(c byte) *separator {
	return &separator{c: c}
}

type bytesSource struct {
	task      *Task
	toString  bool
	delimiter byte

	reader    io.Reader
	ch        chan *Record
	alive     bool
	closeWait sync.WaitGroup
}

func (src *bytesSource) Gen() <-chan *Record {
	src.ch = make(chan *Record)
	src.alive = true
	src.closeWait.Add(1)
	buff := bufio.NewReader(src.reader)
	num := 0
	go func() {
		src.task.SetResultColumns([]*spi.Column{{
			Name: "string", Type: spi.ColumnBufferTypeString,
		}})
		for src.alive {
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
			src.ch <- NewRecord(num, str)
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

func (src *bytesSource) stop() {
	src.alive = false
	src.closeWait.Wait()
}
