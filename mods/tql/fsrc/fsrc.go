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

func errInvalidNumOfArgs(name string, expect int, actual int) error {
	return fmt.Errorf("f(%s) invalid number of args; expect:%d, actual:%d", name, expect, actual)
}

func errWrongTypeOfArgs(name string, idx int, expect string, actual any) error {
	return fmt.Errorf("f(%s) arg(%d) should be %s, but %T", name, idx, expect, actual)
}

func Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, functions)
}

type Input interface {
	Run(InputDeligate) error
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
	"from":      srcf_from,
	"range":     srcf_range,
	"limit":     srcf_limit,
	"dump":      srcf_dump,
	"freq":      srcf_freq,
	"oscilator": src_oscilator,
	"FAKE":      src_FAKE,
	"QUERY":     srcf_QUERY,
	"SQL":       src_SQL,
	"INPUT":     srcf_INPUT,
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
	dbSrc   dbSource
	fakeSrc fakeSource
}

var _ Input = &input{}

func (in *input) Run(deligate InputDeligate) error {
	if in.dbSrc == nil && in.fakeSrc == nil {
		return errors.New("nil source")
	}
	if deligate == nil {
		return errors.New("nil deligate")
	}

	if in.dbSrc != nil {
		queryCtx := &do.QueryContext{
			DB: deligate.Database(),
			OnFetchStart: func(c spi.Columns) {
				deligate.FeedHeader(c)
			},
			OnFetch: func(nrow int64, values []any) bool {
				if deligate.ShouldStop() {
					return false
				} else {
					deligate.Feed(values)
					return true
				}
			},
			OnFetchEnd: func() {
				deligate.Feed(nil)
			},
			OnExecuted: nil, // tql src can not run executable sql
		}
		_, err := do.Query(queryCtx, in.dbSrc.ToSQL())
		if err != nil {
			deligate.Feed(nil)
		}
		return err
	} else {
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
	} else {
		return nil, fmt.Errorf("f(INPUT) unknown type of arg, %T", args[0])
	}
}

type timeRange struct {
	ts       string
	tsTime   time.Time
	duration time.Duration
	period   time.Duration
}

func srcf_range(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(range) invalid number of args (n:%d)", len(args))
	}
	ret := &timeRange{}
	if str, ok := args[0].(string); ok {
		if str != "now" && str != "last" {
			return nil, fmt.Errorf("f(range) 1st args should be time or 'now', 'last', but %T", args[0])
		}
		ret.ts = str
	} else {
		if num, ok := args[0].(float64); ok {
			ret.tsTime = time.Unix(0, int64(num))
		} else {
			if ts, ok := args[0].(time.Time); ok {
				ret.tsTime = ts
			} else {
				return nil, fmt.Errorf("f(range) 1st args should be time or 'now', 'last', but %T", args[0])
			}
		}
	}
	if str, ok := args[1].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.duration = d
		} else {
			return nil, fmt.Errorf("f(range) 2nd args should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.duration = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 2nd args should be duration, but %T", args[1])
	}
	if len(args) == 2 {
		return ret, nil
	}

	if str, ok := args[2].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.period = d
		} else {
			return nil, fmt.Errorf("f(range) 3rd args should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.period = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 3rd args should be duration, but %T", args[1])
	}
	if ret.duration <= ret.period {
		return nil, fmt.Errorf("f(range) 3rd args should be smaller than 2nd")
	}

	return ret, nil
}
