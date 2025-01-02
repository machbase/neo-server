package tql

import (
	"errors"
	"regexp"
	"strings"

	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"
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
