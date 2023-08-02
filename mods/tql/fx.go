package tql

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type Task interface {
	expression.Parameters

	Context() context.Context

	Functions() map[string]expression.Function
	GetFunction(name string) expression.Function

	SetParams(map[string][]string)
	Params() map[string][]string

	SetDataReader(io.Reader)
	DataReader() io.Reader

	SetDataWriter(io.Writer) error
	DataWriter() io.Writer
	SetOutputStream(spec.OutputStream)
	OutputStream() spec.OutputStream

	SetJsonOutput(flag bool)
	ShouldJsonOutput() bool

	AddPragma(p string)
}

var (
	_ Task = &task{}
)

type task struct {
	ctx          context.Context
	functions    map[string]expression.Function
	params       map[string][]string
	dataReader   io.Reader
	dataWriter   io.Writer
	outputStream spec.OutputStream
	toJsonOutput bool

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string
}

func NewTaskContext(ctx context.Context) Task {
	ret := NewTask().(*task)
	ret.ctx = ctx
	return ret
}

func (x *task) Context() context.Context {
	return x.ctx
}

func (x *task) Functions() map[string]expression.Function {
	return x.functions
}

func (x *task) GetFunction(name string) expression.Function {
	return x.functions[name]
}

func (x *task) SetDataReader(r io.Reader) {
	x.dataReader = r
}

func (x *task) DataReader() io.Reader {
	return x.dataReader
}

func (x *task) SetDataWriter(w io.Writer) error {
	var err error
	x.dataWriter = w
	if w == nil {
		x.outputStream, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	} else {
		x.outputStream = &stream.WriterOutputStream{Writer: w}
	}
	return nil
}

func (x *task) DataWriter() io.Writer {
	return x.dataWriter
}

func (x *task) SetOutputStream(o spec.OutputStream) {
	x.outputStream = o
	x.dataWriter = o
}

func (x *task) OutputStream() spec.OutputStream {
	return x.outputStream
}

func (x *task) SetJsonOutput(flag bool) {
	x.toJsonOutput = flag
}

func (x *task) ShouldJsonOutput() bool {
	return x.toJsonOutput
}

func (x *task) SetParams(p map[string][]string) {
	if x.params == nil {
		x.params = map[string][]string{}
	}
	for k, v := range p {
		x.params[k] = v
	}
}

func (x *task) Params() map[string][]string {
	return x.params
}

func (x *task) Get(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := x.params[strings.TrimPrefix(name, "$")]; ok {
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	} else {
		switch name {
		default:
			return nil, fmt.Errorf("undefined variable '%s'", name)
		case "CTX":
			return x, nil
		case "PI":
			return math.Pi, nil
		case "outputstream":
			return x.outputStream, nil
		case "nil":
			return nil, nil
		}
	}
}

func (x *task) AddPragma(p string) {
	x.pragma = append(x.pragma, p)
}
