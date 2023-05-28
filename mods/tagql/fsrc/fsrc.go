package fsrc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
)

var Functions = map[string]expression.Function{
	"range": srcf_range,
	"limit": srcf_limit,
	"INPUT": srcf_INPUT,
}

type Range struct {
	ts       string
	tsTime   time.Time
	duration time.Duration
	groupBy  time.Duration
}

func srcf_range(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(range) invalid number of args (n:%d)", len(args))
	}
	ret := &Range{}
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

	ret := NewSrcInput()
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.Columns = append(ret.Columns, tok)
		case *Range:
			ret.Range = tok
		case *Limit:
			ret.Limit = tok
		default:
			return nil, fmt.Errorf("f(INPUT) unknown type of args[%d], %T", i, tok)
		}
	}
	return ret, nil
}

type SrcInput struct {
	Table          string
	Tag            string
	BaseTimeColumn string

	Columns []string
	Range   *Range
	Limit   *Limit
}

func NewSrcInput() *SrcInput {
	return &SrcInput{
		Columns: []string{},
		Range:   &Range{ts: "last", duration: time.Second, groupBy: 0},
		Limit:   &Limit{limit: 1000000},
	}
}

func (si *SrcInput) ToSQL() string {
	if si.Range == nil || si.Range.groupBy == 0 {
		return si.toSql()
	} else {
		return si.toSqlGroup()
	}
}

func (si *SrcInput) toSqlGroup() string {
	ret := ""
	rng := si.Range
	columns := strings.Join(si.Columns, ", ")
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
			si.BaseTimeColumn, rng.groupBy, rng.groupBy, si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn,
			rng.duration, si.Table, si.Tag,
			si.Table, si.Tag,
			si.BaseTimeColumn,
			si.BaseTimeColumn,
			si.Limit.limit,
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
			si.BaseTimeColumn, rng.groupBy, rng.groupBy, si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn, rng.duration,
			si.BaseTimeColumn,
			si.BaseTimeColumn,
			si.Limit.limit,
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
			si.BaseTimeColumn, rng.groupBy, rng.groupBy, si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn,
			rng.tsTime.UnixNano(), rng.duration, rng.tsTime.UnixNano(),
			si.BaseTimeColumn,
			si.BaseTimeColumn,
			si.Limit.limit,
		)
	}
	return ret
}

func (si *SrcInput) toSql() string {
	ret := ""
	columns := strings.Join(si.Columns, ", ")
	if si.Range.ts == "last" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN 
					(SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			LIMIT %d
			`,
			si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn,
			si.Range.duration, si.Table, si.Tag,
			si.Table, si.Tag,
			si.Limit.limit,
		)
	} else if si.Range.ts == "now" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			LIMIT %d
			`,
			si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn, si.Range.duration,
			si.Limit.limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s 
			WHERE
				name = '%s'
			AND %s
				BETWEEN %d - %d AND %d
			LIMIT %d`,
			si.BaseTimeColumn, columns, si.Table,
			si.Tag,
			si.BaseTimeColumn,
			si.Range.tsTime.UnixNano(), si.Range.duration, si.Range.tsTime.UnixNano(),
			si.Limit.limit)
	}
	return ret
}
