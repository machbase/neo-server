package fsrc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
)

func Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, functions)
}

type Source interface {
	ToSQL(table string, tag string, baseTime string) string
}

func Compile(text string) (Source, error) {
	// validates the syntax
	expr, err := Parse(text)
	if err != nil {
		return nil, err
	}
	ret, err := expr.Eval(nil)
	if err != nil {
		return nil, err
	}
	input, ok := ret.(*srcInput)
	if !ok {
		return nil, fmt.Errorf("compile error, %v", input)
	}
	return input, nil
}

var functions = map[string]expression.Function{
	"range": srcf_range,
	"limit": srcf_limit,
	"INPUT": srcf_INPUT,
}

type timeRange struct {
	ts       string
	tsTime   time.Time
	duration time.Duration
	groupBy  time.Duration
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
			ret.groupBy = d
		} else {
			return nil, fmt.Errorf("f(range) 3rd args should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.groupBy = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 3rd args should be duration, but %T", args[1])
	}
	if ret.duration <= ret.groupBy {
		return nil, fmt.Errorf("f(range) 3rd args should be smaller than 2nd")
	}

	return ret, nil
}

type Limit struct {
	limit int
}

func (lm *Limit) String() string {
	return strconv.Itoa(lm.limit)
}

func srcf_limit(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(limit) invalid number of args (n:%d)", len(args))
	}
	ret := &Limit{}
	if d, ok := args[0].(float64); ok {
		ret.limit = int(d)
	} else {
		return nil, fmt.Errorf("f(range) arg should be int, but %T", args[1])
	}
	return ret, nil
}

// src=INPUT('value', 'STDDEV(val)', range('last', '10s', '1s'), limit(100000) )
func srcf_INPUT(args ...any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("f(INPUT) invalid number of args (n:%d)", len(args))
	}

	ret := NewSource().(*srcInput)
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.columns = append(ret.columns, tok)
		case *timeRange:
			ret.timeRange = tok
		case *Limit:
			ret.limit = tok
		default:
			return nil, fmt.Errorf("f(INPUT) unknown type of args[%d], %T", i, tok)
		}
	}
	return ret, nil
}

type srcInput struct {
	columns   []string
	timeRange *timeRange
	limit     *Limit
}

func NewSource() Source {
	return &srcInput{
		columns:   []string{},
		timeRange: &timeRange{ts: "last", duration: time.Second, groupBy: 0},
		limit:     &Limit{limit: 1000000},
	}
}

func (si *srcInput) ToSQL(table string, tag string, baseTime string) string {
	if si.timeRange == nil || si.timeRange.groupBy == 0 {
		return si.toSql(table, tag, baseTime)
	} else {
		return si.toSqlGroup(table, tag, baseTime)
	}
}

func (si *srcInput) toSqlGroup(table string, tag string, baseTime string) string {
	ret := ""
	rng := si.timeRange
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}

	if rng.ts == "last" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN
				    (SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			baseTime, rng.groupBy, rng.groupBy, baseTime, columns, table,
			tag,
			baseTime,
			rng.duration, table, tag,
			table, tag,
			baseTime,
			baseTime,
			si.limit.limit,
		)
	} else if rng.ts == "now" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			baseTime, rng.groupBy, rng.groupBy, baseTime, columns, table,
			tag,
			baseTime, rng.duration,
			baseTime,
			baseTime,
			si.limit.limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN %d - %d AND %d
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			baseTime, rng.groupBy, rng.groupBy, baseTime, columns, table,
			tag,
			baseTime,
			rng.tsTime.UnixNano(), rng.duration, rng.tsTime.UnixNano(),
			baseTime,
			baseTime,
			si.limit.limit,
		)
	}
	return ret
}

func (si *srcInput) toSql(table string, tag string, baseTime string) string {
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	if si.timeRange.ts == "last" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN 
					(SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			LIMIT %d
			`,
			baseTime, columns, table,
			tag,
			baseTime,
			si.timeRange.duration, table, tag,
			table, tag,
			si.limit.limit,
		)
	} else if si.timeRange.ts == "now" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			LIMIT %d
			`,
			baseTime, columns, table,
			tag,
			baseTime, si.timeRange.duration,
			si.limit.limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s 
			WHERE
				name = '%s'
			AND %s
				BETWEEN %d - %d AND %d
			LIMIT %d`,
			baseTime, columns, table,
			tag,
			baseTime,
			si.timeRange.tsTime.UnixNano(), si.timeRange.duration, si.timeRange.tsTime.UnixNano(),
			si.limit.limit)
	}
	return ret
}
