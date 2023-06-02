package fsrc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fcom"
	spi "github.com/machbase/neo-spi"
)

func Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, functions)
}

type Input interface {
	Run(InputDeligate) error
	Source() Source
}

type Source interface {
	ToSQL() string
}

type inputParameters struct {
	params map[string][]string
}

func (p *inputParameters) Get(name string) (any, error) {
	if name == "CTX" {
		return p, nil
	} else if name == "nil" {
		return nil, nil
	} else if strings.HasPrefix(name, "$") {
		if p, ok := p.params[strings.TrimPrefix(name, "$")]; ok {
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	}
	return nil, fmt.Errorf("undefined variable '%s'", name)
}

func Compile(text string, params map[string][]string) (Input, error) {
	expr, err := Parse(text)
	if err != nil {
		return nil, err
	}
	ret, err := expr.Eval(&inputParameters{params})
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
	"from":  srcf_from,
	"range": srcf_range,
	"limit": srcf_limit,
	"dump":  srcf_dump,
	"QUERY": srcf_QUERY,
	"SQL":   src_SQL,
	"INPUT": srcf_INPUT,
}

func init() {
	for k, v := range fcom.Functions {
		functions[k] = v
	}
}

func NewDefaultInput() Input {
	return &input{
		src: &querySrc{
			columns:   []string{},
			timeRange: &queryRange{ts: "last", duration: time.Second, groupBy: 0},
			limit:     &queryLimit{limit: 1000000},
		},
	}
}

type input struct {
	src Source
}

var _ Input = &input{}

func (in *input) Run(deligate InputDeligate) error {
	if in.src == nil {
		return errors.New("nil source")
	}
	if deligate == nil {
		return errors.New("nil deligate")
	}

	queryCtx := &do.QueryContext{
		DB: deligate.Database(),
		OnFetchStart: func(c spi.Columns) {
			deligate.FeedHeader(c)
		},
		OnFetch: func(nrow int64, values []any) bool {
			if deligate.ShouldStop() {
				return false
			}
			deligate.Feed(values)
			return true
		},
		OnFetchEnd: func() {
			deligate.Feed(nil)
		},
		OnExecuted: nil, // never happen in tagQL
	}
	_, err := do.Query(queryCtx, in.src.ToSQL())
	if err != nil {
		deligate.Feed(nil)
	}
	return err
}

func (in *input) Source() Source {
	return in.src
}

// src=INPUT('value', 'STDDEV(val)', range('last', '10s', '1s'), limit(100000) )
func srcf_INPUT(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(INPUT) invalid number of args (n:%d)", len(args))
	}
	if s, ok := args[0].(Source); !ok {
		return nil, fmt.Errorf("f(INPUT) unknown type of arg, %T", args[0])
	} else {
		return &input{
			src: s,
		}, nil
	}
}
