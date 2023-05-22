package util

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/machbase/neo-server/mods/util/expression"
)

type TagPath struct {
	Table string
	Tag   string
	Field TagPathField
}

type TagPathField struct {
	Columns []string
	expr    *expression.Expression
}

func (tpf TagPathField) Eval(args ...any) (float64, error) {
	if len(tpf.Columns) != len(args) {
		return math.NaN(), fmt.Errorf("require %d args, but have %d", len(tpf.Columns), len(args))
	}
	var result any
	var err error
	if tpf.expr == nil {
		result = args[0]
	} else {
		p := map[string]any{}
		for i := range tpf.Columns {
			p[tpf.Columns[i]] = args[i]
		}
		result, err = tpf.expr.Evaluate(p)
		if err != nil {
			return math.NaN(), err
		}
	}
	switch v := result.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	default:
		return math.NaN(), fmt.Errorf("evaluted values should be float, but have %T", v)
	}
}

var _dummy_functions = map[string]expression.Function{}

var regexpTagPath = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)

// parse
// "<table>/<tag>"
// "<table>/<tag>#column"
// "<table>/<tag>#function()"
// "<table>/<tag>#function(<column>)"
// "<table>/<tag>#func2(func1())"
// "<table>/<tag>#func2(func1(<column>))"
func ParseTagPath(path string) (*TagPath, error) {
	return ParseTagPathWithFunctions(path, _dummy_functions)
}

func ParseTagPathWithFunctions(path string, functions map[string]expression.Function) (*TagPath, error) {
	toks := regexpTagPath.FindAllStringSubmatch(path, -1)
	// fmt.Println("PATH", path)
	// for i := range toks {
	// 	for n := range toks[i] {
	// 		fmt.Printf("  toks[%d][%d] %s\n", i, n, toks[i][n])
	// 	}
	// }
	if len(toks) != 1 || len(toks[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}
	r := &TagPath{}
	r.Table = strings.ToUpper(strings.TrimSpace(toks[0][1]))
	termParts := strings.SplitN(toks[0][2], "#", 2)
	r.Tag = termParts[0]
	r.Field = TagPathField{}

	if len(termParts) == 1 {
		r.Field.Columns = []string{"VALUE"}
	} else if len(termParts) == 2 {
		expr, err := expression.NewWithFunctions(termParts[1], functions)
		if err != nil {
			return nil, err
		}
		r.Field.Columns = expr.Vars()
		r.Field.expr = expr
	}
	return r, nil
}

type WritePath struct {
	Table     string
	Format    string
	Transform string
	Compress  string
}

// parse
// "<table>"                      default format is "json"
// "<table>:<format>"             format "json", "csv"
// "<table>:<format>:<compress>"  transformer "gzip" or "-", "" for no-compression
// "<table>:<format>:<transform>:<compress>" transformer "gzip" or "-", "" for no-compression
func ParseWritePath(path string) (*WritePath, error) {
	toks := strings.Split(path, ":")
	toksLen := len(toks)
	if toksLen == 0 || toksLen > 4 {
		return nil, errors.New("invalid syntax")
	}

	r := &WritePath{}
	switch toksLen {
	case 1:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
	case 2:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
	case 3:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
		r.Compress = strings.ToLower(strings.TrimSpace(toks[2]))
	case 4:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
		r.Transform = strings.ToLower(strings.TrimSpace(toks[2]))
		r.Compress = strings.ToLower(strings.TrimSpace(toks[3]))
	}
	return r, nil
}
