package util

import (
	"errors"
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

var regexpTagPath = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)

// parse
// "<table>/<tag>"
// "<table>/<tag>#column"
// "<table>/<tag>#function()"
// "<table>/<tag>#function(<column>)"
// "<table>/<tag>#func2(func1())"
// "<table>/<tag>#func2(func1(<column>))"
func ParseTagPath(path string) (*TagPath, error) {
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
		expr, err := expression.NewWithFunctions(termParts[1], map[string]expression.Function{
			"kalman": func(args ...any) (any, error) { return nil, nil },
			"fft":    func(args ...any) (any, error) { return nil, nil },
		})
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
