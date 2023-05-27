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
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

type TagQL interface {
	ToSQL() string
	Execute(ctx context.Context, db spi.Database, deligate *ExecuteDeligate) error
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

	fieldColumns   string
	baseTimeColumn string

	strTime   string
	timeRange time.Duration
	strLimit  string
	timeGroup time.Duration

	mapExprs []string
	sinkExpr string
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
	tq.fieldColumns = strings.Join(expressionParts, ", ")
	for _, part := range expressionParts {
		// validates the syntax: e.g) invalid token, undefined function...
		_ /* expr */, err := expression.NewWithFunctions(part, fieldFunctions)
		if err != nil {
			return nil, err
		}
	}

	mapParts := params["map"]
	for _, part := range mapParts {
		// validates the syntax
		_ /*expr */, err := expression.NewWithFunctions(part, mapFunctions)
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, part)
	}

	sinkParts := params["sink"]
	if len(sinkParts) > 0 {
		// only take the last one
		part := sinkParts[len(sinkParts)-1]
		// validates the syntax
		_ /*expr*/, err := expression.NewWithFunctions(part, sinkFunctions)
		if err != nil {
			return nil, err
		}
		tq.sinkExpr = part
	}

	return tq, nil
}

type ExecuteDeligate struct {
	OnStart      func(contentType string, compress string)
	OutputStream func() spec.OutputStream
}

func (tq *tagQL) Execute(ctxCtx context.Context, db spi.Database, deligate *ExecuteDeligate) (err error) {
	deferHooks := []func(){}
	defer func() {
		for _, f := range deferHooks {
			f()
		}
	}()

	var output spec.OutputStream
	if deligate != nil && deligate.OutputStream != nil {
		output = deligate.OutputStream()
	} else {
		output, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	}
	sinkExpr, err := expression.NewWithFunctions(normalizeSinkFuncExpr(tq.sinkExpr), sinkFunctions)
	if err != nil {
		return err
	}
	sinkRet, err := sinkExpr.Evaluate(map[string]any{"outstream": output})
	if err != nil {
		return err
	}
	encoder, ok := sinkRet.(codec.RowsEncoder)
	if !ok {
		return fmt.Errorf("invalid sink type: %T", sinkRet)
	}

	chain, err := NewExecutionChain(ctxCtx, tq.mapExprs)
	if err != nil {
		return err
	}

	var cols spi.Columns
	queryCtx := &do.QueryContext{
		DB: db,
		OnFetchStart: func(c spi.Columns) {
			cols = c
		},
		OnFetch: func(nrow int64, values []any) bool {
			if chain.Error() != nil {
				return false
			}
			chain.Source(values)
			return true
		},
		OnFetchEnd: func() {
			chain.Source(nil)
		},
		OnExecuted: nil, // never happen in tagQL
	}

	go chain.Start()
	go func() {
		open := false
		for arr := range chain.Sink() {
			if !open {
				if cols == nil {
					for i, v := range arr {
						cols = append(cols, &spi.Column{
							Name: fmt.Sprintf("col#%d", i),
							Type: fmt.Sprintf("%T", v)})
					}
				}
				if deligate != nil && deligate.OnStart != nil {
					deligate.OnStart(encoder.ContentType(), "" /*compression*/)
				}
				// TODO can not trust column types if arr comes through map()
				encoder.Open(cols)
				deferHooks = append(deferHooks, func() {
					// if close encoder right away without defer,
					// it will crash, because it could be earlier than all map() pipe to be closed
					encoder.Close()
				})
				open = true
			}
			encoder.AddRow(arr)
		}
	}()
	_, err = do.Query(queryCtx, tq.ToSQL())

	chain.Wait()
	chain.Stop()

	if chain.Error() != nil {
		return chain.Error()
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
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.fieldColumns, tq.table,
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
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.fieldColumns, tq.table,
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
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.fieldColumns, tq.table,
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
			tq.baseTimeColumn, tq.fieldColumns, tq.table,
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
			tq.baseTimeColumn, tq.fieldColumns, tq.table,
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
			tq.baseTimeColumn, tq.fieldColumns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.strTime, tq.timeRange, tq.strTime,
			tq.strLimit)
	}
	return ret
}
