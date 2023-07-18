package fsrc

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/fcom"
	spi "github.com/machbase/neo-spi"
)

func Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, functions)
}

type Input interface {
	Run(InputDeligate) error
}

type inputParameters struct {
	Body   io.Reader
	params map[string][]string
}

func (p *inputParameters) Get(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := p.params[strings.TrimPrefix(name, "$")]; ok {
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
			return p, nil
		case "PI":
			return math.Pi, nil
		case "nil":
			return nil, nil
		}
	}
}

func Compile(code string, dataReader io.Reader, params map[string][]string) (Input, error) {
	expr, err := Parse(code)
	if err != nil {
		return nil, err
	}
	ret, err := expr.Eval(&inputParameters{Body: dataReader, params: params})
	if err != nil {
		return nil, err
	}
	input, ok := ret.(Input)
	if !ok {
		return nil, fmt.Errorf("compile error, %v", input)
	}
	return input, nil
}

var functions = map[string]expression.Function{
	"from":         srcf_from,
	"range":        srcf_range,
	"between":      srcf_between,
	"limit":        srcf_limit,
	"dump":         srcf_dump,
	"freq":         srcf_freq,
	"oscillator":   src_oscillator,
	"sphere":       src_sphere,
	"FAKE":         src_FAKE,
	"CSV":          src_CSV,
	"file":         src_file,         // CSV()
	"col":          src_col,          // CSV()
	"header":       src_header,       // CSV()
	"datetimeType": src_datetimeType, // col()
	"stringType":   src_stringType,   // col()
	"doubleType":   src_doubleType,   // col()
	"STRING":       src_STRING,
	"BYTES":        src_BYTES,
	"delimiter":    srcf_delimiter,
	"QUERY":        srcf_QUERY,
	"SQL":          src_SQL,
	"INPUT":        srcf_INPUT,
}

func init() {
	for k, v := range fcom.Functions {
		functions[k] = v
	}
}

func Functions() []string {
	ret := []string{}
	for k := range functions {
		ret = append(ret, k)
	}
	return ret
}

type input struct {
	dbSrc     dbSource
	fakeSrc   fakeSource
	readerSrc readerSource
}

var _ Input = &input{}

func (in *input) Run(deligate InputDeligate) error {
	if in.dbSrc == nil && in.fakeSrc == nil && in.readerSrc == nil {
		return errors.New("nil source")
	}
	if deligate == nil {
		return errors.New("nil deligate")
	}

	fetched := 0
	executed := false
	if in.dbSrc != nil {
		queryCtx := &do.QueryContext{
			DB: deligate.Database(),
			OnFetchStart: func(c spi.Columns) {
				deligate.FeedHeader(c)
			},
			OnFetch: func(nrow int64, values []any) bool {
				fetched++
				if deligate.ShouldStop() {
					return false
				} else {
					deligate.Feed(values)
					return true
				}
			},
			OnFetchEnd: func() {},
			OnExecuted: func(usermsg string, rowsAffected int64) {
				executed = true
			},
		}
		if msg, err := do.Query(queryCtx, in.dbSrc.ToSQL()); err != nil {
			deligate.Feed(nil)
			return err
		} else {
			if executed {
				deligate.FeedHeader(spi.Columns{{Name: "message", Type: "string"}})
				deligate.Feed([]any{msg})
				deligate.Feed(nil)
			} else if fetched == 0 {
				deligate.Feed([]any{context.ExecutionEOF})
				deligate.Feed(nil)
			} else {
				deligate.Feed(nil)
			}
			return nil
		}
	} else if in.fakeSrc != nil {
		deligate.FeedHeader(in.fakeSrc.Header())
		for values := range in.fakeSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.fakeSrc.Stop()
				break
			}
		}
		deligate.Feed(nil)
		return nil
	} else if in.readerSrc != nil {
		deligate.FeedHeader(in.readerSrc.Header())
		for values := range in.readerSrc.Gen() {
			deligate.Feed(values)
			if deligate.ShouldStop() {
				in.readerSrc.Stop()
				break
			}
		}
		deligate.Feed(nil)
		return nil
	} else {
		return errors.New("no source")
	}
}

// src=INPUT('value', 'STDDEV(val)', range('last', '10s', '1s'), limit(100000) )
func srcf_INPUT(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(INPUT) invalid number of args (n:%d)", len(args))
	}
	if s, ok := args[0].(dbSource); ok {
		return &input{
			dbSrc: s,
		}, nil
	} else if s, ok := args[0].(fakeSource); ok {
		return &input{
			fakeSrc: s,
		}, nil
	} else if s, ok := args[0].(readerSource); ok {
		return &input{
			readerSrc: s,
		}, nil
	} else {
		return nil, fmt.Errorf("f(INPUT) unknown type of arg, %T", args[0])
	}
}
