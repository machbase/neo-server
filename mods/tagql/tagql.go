package tagql

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
)

type TagQL interface {
	ToSQL() string
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

	columns []string
	expr    *expression.Expression

	baseTimeColumn string

	strTime   string
	timeRange time.Duration
	strLimit  string
	timeGroup time.Duration
}

var defaultFunctions = map[string]expression.Function{}

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
	toks := regexpTagQL.FindAllStringSubmatch(query, -1)
	// fmt.Println("TAGQL", query)
	// for i := range toks {
	// 	for n := range toks[i] {
	// 		fmt.Printf("  toks[%d][%d] %s\n", i, n, toks[i][n])
	// 	}
	// }
	if len(toks) != 1 || len(toks[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}

	tq := &tagQL{}
	tq.baseTimeColumn = ctx.BaseTimeColumn

	tq.table = strings.ToUpper(strings.TrimSpace(toks[0][1]))
	termParts := strings.SplitN(toks[0][2], "#", 2)
	tq.tag = termParts[0]

	var params map[string][]string
	if len(termParts) == 1 {
		tq.columns = []string{"value"}
	} else if len(termParts) == 2 {
		var expressionPart string
		paramParts := strings.SplitN(termParts[1], "?", 2)
		if len(paramParts) == 1 {
			expressionPart = termParts[1]
		} else if len(paramParts) == 2 {
			expressionPart = paramParts[0]
			urlParams, err := url.ParseQuery(paramParts[1])
			if err != nil {
				return nil, err
			}
			params = urlParams
		}
		expr, err := expression.NewWithFunctions(expressionPart, defaultFunctions)
		if err != nil {
			return nil, err
		}
		tq.columns = expr.Vars()
		tq.expr = expr
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

	return tq, nil
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
	// fields := strings.Join(tq.columns, ", ")

	if tq.strTime == "last" {
		ret = fmt.Sprintf(`SELECT round(to_timestamp(%s)/%d)*%d %s, stddev(value) value FROM %s
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
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.timeRange, tq.table, tq.tag,
			tq.table, tq.tag,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.strLimit,
		)
	} else if tq.strTime == "now" {
		ret = fmt.Sprintf(`SELECT round(to_timestamp(%s)/%d)*%d %s, stddev(value) value FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			GROUP BY %s
			ORDER BY %s
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.table,
			tq.tag,
			tq.baseTimeColumn, tq.timeRange,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.strLimit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT round(to_timestamp(%s)/%d)*%d %s, stddev(value) value FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN %s - %d AND %s
			GROUP BY %s
			ORDER BY %s
			LIMIT %s
			`,
			tq.baseTimeColumn, tq.timeGroup, tq.timeGroup, tq.baseTimeColumn, tq.table,
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
	fields := strings.Join(tq.columns, ", ")
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
			tq.baseTimeColumn, fields, tq.table,
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
			tq.baseTimeColumn, fields, tq.table,
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
			tq.baseTimeColumn, fields, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			tq.strTime, tq.timeRange, tq.strTime,
			tq.strLimit)
	}
	return ret
}
