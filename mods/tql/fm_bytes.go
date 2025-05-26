package tql

import (
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
	Path     string // Original path
	HttpPath string
	AbsPath  string
}

func (x *Node) fmFile(path string) (*FilePath, error) {
	if strings.HasPrefix(path, "http://") {
		return &FilePath{Path: path, HttpPath: path}, nil
	} else if strings.HasPrefix(path, "https://") {
		return &FilePath{Path: path, HttpPath: path}, nil
	} else {
		serverFs := ssfs.Default()
		if serverFs == nil {
			return nil, os.ErrNotExist
		}
		realPath, err := serverFs.RealPath(path)
		if err != nil {
			return nil, err
		}
		return &FilePath{Path: path, AbsPath: realPath}, nil
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

	yield := func(num int, data []byte) {
		if src.toString {
			rec := string(data)
			if src.trimspace {
				rec = strings.TrimSpace(rec)
			}
			NewRecord(num, rec).Tell(node.next)
		} else {
			NewRecord(num, data).Tell(node.next)
		}
	}

	num := 1
	remains := []byte{}
	totalBytes := 0
	isEOF := false
	for !node.task.shouldStop() && !isEOF {
		readbuf := make([]byte, 4024)
		n, err := reader.Read(readbuf)
		totalBytes += n
		if err != nil || n == 0 {
			isEOF = true
		}
		remains = append(remains, readbuf[:n]...)

		for {
			idx := bytes.IndexByte(remains, src.delimiter)
			if idx == -1 {
				break
			}
			line := remains[:idx+1]
			if len(remains) > idx+1 {
				remains = remains[idx+1:]
			} else {
				remains = remains[:0]
			}

			vlen := len(line)
			if src.delimiter == '\n' && vlen > 1 && line[vlen-2] == '\r' {
				line = line[0 : vlen-2]
			} else {
				line = line[0 : vlen-1]
			}

			yield(num, line)
			num++
		}
	}
	if len(remains) > 0 {
		yield(num, remains)
	}
}
