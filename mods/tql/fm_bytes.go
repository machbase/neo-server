package tql

import (
	"bufio"
	"bytes"
	"io"
	"os"

	"github.com/machbase/neo-server/mods/util/ssfs"
	spi "github.com/machbase/neo-spi"
)

// STRING(payload() | 'string' | file('path') [, separator()])
func (x *Node) fmString(origin any, args ...any) (any, error) {
	ret := &bytesSource{toString: true}
	err := ret.init(origin, args...)
	if err != nil {
		return nil, err
	}
	ret.gen(x)
	return nil, err
}

// BYTES(payload() | 'string' | file('path') [, separator()])
func (x *Node) fmBytes(origin any, args ...any) (any, error) {
	ret := &bytesSource{}
	err := ret.init(origin, args...)
	if err != nil {
		return nil, err
	}
	ret.gen(x)
	return nil, err
}

func (ret *bytesSource) init(origin any, args ...any) error {
	fnName := "BYTES"
	if ret.toString {
		fnName = "STRING"
	}

	switch src := origin.(type) {
	case string:
		ret.reader = bytes.NewBufferString(src)
	case []byte:
		ret.reader = bytes.NewBuffer(src)
	case io.Reader:
		ret.reader = src
	case *FilePath:
		content, err := os.ReadFile(src.AbsPath)
		if err != nil {
			return err
		}
		ret.reader = bytes.NewBuffer(content)
	default:
		return ErrArgs(fnName, 0, "reader or string")
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case *separator:
			ret.delimiter = v.c
		default:
			return ErrArgs(fnName, 1, "require the separator() option")
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
	toString  bool
	delimiter byte
	reader    io.Reader
}

func (src *bytesSource) gen(node *Node) {
	buff := bufio.NewReader(src.reader)

	var label string
	if src.toString {
		label = "string"
	} else {
		label = "bytes"
	}
	node.task.SetResultColumns([]*spi.Column{
		{Name: "id", Type: "int"},
		{Name: label, Type: spi.ColumnBufferTypeString},
	})

	num := 1
	for {
		if src.toString {
			v, err := buff.ReadString(src.delimiter)
			for len(v) > 0 && v[len(v)-1] == src.delimiter {
				v = v[0 : len(v)-1]
			}
			NewRecord(num, v).Tell(node.next)
			if err != nil {
				break
			}
		} else {
			v, err := buff.ReadBytes(src.delimiter)
			for len(v) > 0 && v[len(v)-1] == src.delimiter {
				v = v[0 : len(v)-1]
			}
			NewRecord(num, v).Tell(node.next)
			if err != nil {
				break
			}
		}
		num++
	}
}
