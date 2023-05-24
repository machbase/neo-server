package tagql

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	spi "github.com/machbase/neo-spi"
)

type TagQL interface {
	ToSQL() string
	Execute(ctx context.Context, db spi.Database, encoder codec.RowsEncoder) error
}

type Context struct {
	BaseTimeColumn string
	MaxLimit       int
	DefaultRange   time.Duration
	MaxRange       time.Duration
}

type tagQL struct {
	table string
	tag   string

	yieldColumns   string
	baseTimeColumn string

	strTime   string
	timeRange time.Duration
	strLimit  string
	timeGroup time.Duration

	mapExprs []*expression.Expression
}

var yieldFunctions = map[string]expression.Function{
	"STDDEV":          func(args ...any) (any, error) { return nil, nil },
	"AVG":             func(args ...any) (any, error) { return nil, nil },
	"SUM":             func(args ...any) (any, error) { return nil, nil },
	"COUNT":           func(args ...any) (any, error) { return nil, nil },
	"TS_CHANGE_COUNT": func(args ...any) (any, error) { return nil, nil },
	"SUMSQ":           func(args ...any) (any, error) { return nil, nil },
	"FIRST":           func(args ...any) (any, error) { return nil, nil },
	"LAST":            func(args ...any) (any, error) { return nil, nil },
}

var regexpTagQL = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)

func ParseTagQL(query string) (TagQL, error) {
	ctx := &Context{
		BaseTimeColumn: "time",
		MaxLimit:       10000,
		MaxRange:       100 * time.Second,
		DefaultRange:   1 * time.Second,
	}
	return ParseTagQLContext(ctx, query)
}

func ParseTagQLContext(ctx *Context, query string) (TagQL, error) {
	subs := regexpTagQL.FindAllStringSubmatch(query, -1)
	if len(subs) != 1 || len(subs[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}

	tq := &tagQL{}
	tq.baseTimeColumn = ctx.BaseTimeColumn

	tq.table = strings.ToUpper(strings.TrimSpace(subs[0][1]))

	uri, err := url.Parse("tag:///" + query)
	if err != nil {
		return nil, err
	}

	tq.table = strings.ToUpper(strings.TrimPrefix(path.Dir(uri.Path), "/"))
	tq.tag = path.Base(uri.Path)
	queryPart := uri.RawQuery

	var params map[string][]string
	if queryPart != "" {
		urlParams, err := url.ParseQuery(queryPart)
		if err != nil {
			return nil, err
		}
		params = urlParams
	}

	getParam := func(k string, def string) string {
		if vals, ok := params[k]; ok {
			return vals[len(vals)-1]
		} else {
			return def
		}
	}
	tq.strTime = strings.ToLower(getParam("time", "last"))
	tq.strLimit = getParam("limit", strconv.Itoa(ctx.MaxLimit))
	if d := getParam("group", ""); d == "" {
		tq.timeGroup = 0
	} else {
		var err error
		tq.timeGroup, err = time.ParseDuration(d)
		if err != nil {
			return nil, errors.New("invalid group syntax")
		}
	}
	if d := getParam("range", ""); d == "" {
		tq.timeRange = ctx.DefaultRange
	} else {
		var err error
		tq.timeRange, err = time.ParseDuration(d)
		if err != nil {
			return nil, errors.New("invalid range syntax")
		}
	}
	expressionParts := params["yield"]
	if len(expressionParts) == 0 {
		expressionParts = []string{"value"}
	}
	tq.yieldColumns = strings.Join(expressionParts, ", ")
	for _, part := range expressionParts {
		// validates the syntax: e.g) invalid token, undefined function...
		_ /* expr */, err := expression.NewWithFunctions(part, yieldFunctions)
		if err != nil {
			return nil, err
		}
	}

	mapParts := params["map"]
	for _, part := range mapParts {
		expr, err := expression.NewWithFunctions(part, newMapFunctions())
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, expr)
	}
	return tq, nil
}

func (tq *tagQL) Execute(ctx context.Context, db spi.Database, encoder codec.RowsEncoder) (err error) {
	queryCtx := &do.QueryContext{
		DB: db,
		OnFetchStart: func(cols spi.Columns) {
			encoder.Open(cols)
		},
		OnFetch: func(nrow int64, values []any) bool {
			var param []any = values
			for _, expr := range tq.mapExprs {
				var ret any
				var retValid bool
				ret, err = expr.Eval(MapData(param))
				if err != nil {
					return false
				}
				if ret == nil {
					// aggregation function can return nil
					param = nil
					return true
				}
				if param, retValid = ret.([]any); !retValid {
					err = fmt.Errorf("map function %s returns invalid value", expr.String())
					return false
				}
			}
			if param != nil {
				if err = encoder.AddRow(param); err != nil {
					return false
				}
			}
			return true
		},
		OnFetchEnd: func() {
			var param = endOfDataFrame // indicator of end of dataframe
			for _, expr := range tq.mapExprs {
				ret, _ := expr.Eval(MapData(param))
				if ret == nil {
					param = endOfDataFrame
					continue
				} else {
					var ok bool
					if param, ok = ret.([]any); !ok {
						param = endOfDataFrame
					}
				}
			}
			if param != nil && !IsEndOfDataFrame(param) {
				encoder.AddRow(param)
			}
			encoder.Close()
		},
		OnExecuted: nil, // never happen in tagQL
	}

	_, err = do.Query(queryCtx, tq.ToSQL())
	return err
}

var endOfDataFrame = []any{io.EOF}

func IsEndOfDataFrame(v any) bool {
	if arr, ok := v.([]any); ok {
		return len(arr) >= 1 && arr[0] == io.EOF
	}
	return false
}

type MapData []any

func (p MapData) Get(name string) (any, error) {
	switch len(p) {
	case 0:
		return nil, errors.New("empty parameter")
	case 1:
		if p[0] == io.EOF {
			return io.EOF, nil
		}
		return nil, fmt.Errorf("not enough parameters (len:1)")
	}
	if name == "K" || name == "k" {
		return p[0], nil
	} else if name == "V" || name == "v" {
		return p[1:], nil
	} else {
		return nil, fmt.Errorf("parameter '%s' is not defined", name)
	}
}

func (p MapData) IsEOF() bool {
	return IsEndOfDataFrame(p)
}

func (p MapData) String() string {
	buf := []string{"tagql.MapData"}
	for _, v := range p {
		buf = append(buf, fmt.Sprintf("%T(%v)", v, v))
	}
	return strings.Join(buf, ", ")
}

func (tq *tagQL) ToSQL() string {
	if tq.timeGroup == 0 {
		return tq.toSql()
	} else {
		return tq.toSqlGroup()
	}
}

func (tq *tagQL) toSqlGroup() string {
	ret := ""
	if tq.strTime == "last" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN
				    (SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			GROUP BY %s
			ORDER BY %s
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.timeRange, tq.table, tq.tag,
			tq.table, tq.tag,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.strLimit,
		)
	} else if tq.strTime == "now" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			GROUP BY %s
			ORDER BY %s
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn, tq.timeRange,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.strLimit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN %s - %d AND %s
			GROUP BY %s
			ORDER BY %s
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.strTime, tq.timeRange, tq.strTime,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.strLimit,
		)
	}
	return ret
}

func (tq *tagQL) toSql() string {
	ret := ""
	if tq.strTime == "last" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN 
					(SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.timeRange, tq.table, tq.tag,
			tq.table, tq.tag,
			tq.strLimit,
		)
	} else if tq.strTime == "now" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn, tq.timeRange,
			tq.strLimit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s 
			WHERE
				name = '%s'
			AND %s
				BETWEEN %s - %d AND %s
			LIMIT %s`,
			tq.baseTimeColumn, tq.yieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.strTime, tq.timeRange, tq.strTime,
			tq.strLimit)
	}
	return ret
}
