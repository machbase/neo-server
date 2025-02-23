package tql

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util/charset"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func (x *Node) fmCharset(charsetName string) (opts.Option, error) {
	cs, ok := charset.Encoding(charsetName)
	if !ok || cs == nil {
		return nil, fmt.Errorf("invalid charset %q", charsetName)
	}
	return opts.CharsetEncoding(cs), nil
}

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
		ret.srcString = src
	case []byte:
		ret.srcBytes = src
	case io.Reader:
		ret.srcReader = src
	case *FilePath:
		if src.AbsPath != "" {
			ret.srcFile = src.AbsPath
		} else if src.HttpPath != "" {
			ret.srcHttp = src.HttpPath
		} else {
			return ErrArgs(fnName, 0, "reader or string")
		}
	default:
		return ErrArgs(fnName, 0, "reader or string")
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case *separator:
			ret.delimiter = v.c
		case *trimspace:
			ret.trimspace = v.flag
		default:
			return ErrArgs(fnName, 1, fmt.Sprintf("unknown options %T", arg))
		}
	}
	return nil
}

type FilePath struct {
	HttpPath string
	AbsPath  string
}

func (x *Node) fmFile(path string) (*FilePath, error) {
	if strings.HasPrefix(path, "http://") {
		return &FilePath{HttpPath: path}, nil
	} else if strings.HasPrefix(path, "https://") {
		return &FilePath{HttpPath: path}, nil
	} else {
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
}

type separator struct {
	c byte
}

func (x *Node) fmSeparator(c byte) *separator {
	return &separator{c: c}
}

type trimspace struct {
	flag bool
}

func (x *Node) fmTrimspace(b bool) *trimspace {
	return &trimspace{flag: b}
}

type bytesSource struct {
	toString  bool
	delimiter byte
	trimspace bool

	srcReader io.Reader
	srcString string
	srcBytes  []byte
	srcFile   string
	srcHttp   string
}

func (src *bytesSource) gen(node *Node) {
	var reader io.Reader
	if src.srcString != "" {
		reader = bytes.NewBufferString(src.srcString)
	} else if src.srcBytes != nil {
		reader = bytes.NewBuffer(src.srcBytes)
	} else if src.srcReader != nil {
		reader = src.srcReader
	} else if src.srcFile != "" {
		content, err := os.Open(src.srcFile)
		if err != nil {
			node.task.LogErrorf("Fail to read %q, %s", src.srcFile, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		defer content.Close()
		reader = content
	} else if src.srcHttp != "" {
		req, err := http.NewRequestWithContext(node.task.ctx, "GET", src.srcHttp, nil)
		if err != nil {
			node.task.LogErrorf("Fail to request %q, %s", src.srcHttp, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		httpClient := node.task.NewHttpClient()
		resp, err := httpClient.Do(req)
		if err != nil {
			node.task.LogErrorf("Fail to GET %q, %s", src.srcHttp, err.Error())
			ErrorRecord(err).Tell(node.next)
			return
		}
		defer resp.Body.Close()
		reader = resp.Body
	} else {
		ErrorRecord(fmt.Errorf("no data location is specified")).Tell(node.next)
		return
	}

	buff := bufio.NewReader(reader)

	var label string
	if src.toString {
		label = "STRING"
	} else {
		label = "BYTES"
	}
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnString(label),
	})

	num := 1
	for {
		if src.toString {
			v, err := buff.ReadString(src.delimiter)
			vlen := len(v)
			for vlen > 0 && v[vlen-1] == src.delimiter {
				if src.delimiter == '\n' && vlen > 1 && v[vlen-2] == '\r' {
					v = v[0 : vlen-2]
				} else {
					v = v[0 : vlen-1]
				}
				vlen = len(v)
			}
			if src.trimspace {
				v = strings.TrimSpace(v)
			}
			NewRecord(num, v).Tell(node.next)
			if err != nil {
				break
			}
		} else {
			v, err := buff.ReadBytes(src.delimiter)
			vlen := len(v)
			for vlen > 0 && v[vlen-1] == src.delimiter {
				if src.delimiter == '\n' && vlen > 1 && v[vlen-2] == '\r' {
					v = v[0 : vlen-2]
				} else {
					v = v[0 : vlen-1]
				}
				vlen = len(v)
			}
			NewRecord(num, v).Tell(node.next)
			if err != nil {
				break
			}
		}
		num++
	}
}
