package tagql

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	expressionParts := params["field"]
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
		expr, err := expression.NewWithFunctions(part, mapFunctions)
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, expr)
	}
	return tq, nil
}

func (tq *tagQL) Execute(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) (err error) {
	R := make(chan any)
	closeOnce := sync.Once{}

	ctxArr := NewContextChain(ctxCtx, tq.mapExprs, R)
	var firstCtx *ExecutionContext
	if len(ctxArr) > 0 {
		firstCtx = ctxArr[0]
	}

	var errorToStop error
	var waitWg sync.WaitGroup

	queryCtx := &do.QueryContext{
		DB: db,
		OnFetchStart: func(cols spi.Columns) {
			encoder.Open(cols)
		},
		OnFetch: func(nrow int64, values []any) bool {
			if errorToStop != nil {
				return false
			}
			if firstCtx != nil {
				firstCtx.C <- &ExecutionParam{Ctx: firstCtx, K: values[0], V: values[1:]}
			} else {
				encoder.AddRow(values)
			}
			return true
		},
		OnFetchEnd: func() {
			if firstCtx != nil {
				firstCtx.C <- ExecutionEOF
			} else {
				encoder.Close()
			}
		},
		OnExecuted: nil, // never happen in tagQL
	}

	waitWg.Add(len(ctxArr))
	go func() {
		for ret := range R {
			switch castV := ret.(type) {
			case *ExecutionParam:
				if castV == ExecutionEOF {
					waitWg.Done()
				} else {
					switch tV := castV.V.(type) {
					case []any:
						if subarr, ok := tV[0].([][]any); ok {
							for _, subitm := range subarr {
								fields := append([]any{castV.K}, subitm...)
								encoder.AddRow(fields)
							}
						} else {
							fields := append([]any{castV.K}, tV...)
							encoder.AddRow(fields)
						}
					case [][]any:
						for _, row := range tV {
							fields := append([]any{castV.K}, row...)
							encoder.AddRow(fields)
						}
					}
				}
			case []*ExecutionParam:
				for _, v := range castV {
					switch tV := v.V.(type) {
					case []any:
						fields := append([]any{v.K}, tV...)
						encoder.AddRow(fields)
					case [][]any:
						for _, row := range tV {
							fields := append([]any{v.K}, row...)
							encoder.AddRow(fields)
						}
					}
				}
			case error:
				errorToStop = castV
			}
		}
	}()

	_, err = do.Query(queryCtx, tq.ToSQL())

	waitWg.Wait()
	for _, ctx := range ctxArr {
		ctx.Stop()
	}
	closeOnce.Do(func() { close(R) })
	encoder.Close()

	if errorToStop != nil {
		return errorToStop
	}
	return err
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
