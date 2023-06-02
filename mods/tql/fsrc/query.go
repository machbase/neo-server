package fsrc

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type dbSource interface {
	ToSQL() string
}

type sqlSrc struct {
	text string
}

var _ dbSource = &sqlSrc{}

// SQL('select ....')
func src_SQL(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(SQL) invalid number of args (n:%d)", len(args))
	}
	if str, ok := args[0].(string); ok {
		return &sqlSrc{text: str}, nil
	} else {
		return nil, errors.New("f(SQL) sql select statement should be specified in string")
	}
}

func (s *sqlSrc) ToSQL() string {
	return s.text
}

// QUERY('value', 'STDDEV(val)', from('example', 'sig.1'), range('last', '10s', '1s'), limit(100000) )
func srcf_QUERY(args ...any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("f(QUERY) invalid number of args (n:%d)", len(args))
	}
	ret := &querySrc{
		columns:   []string{},
		timeRange: &timeRange{ts: "last", duration: time.Second, period: 0},
		limit:     &queryLimit{limit: 1000000},
	}
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.columns = append(ret.columns, tok)
		case *queryFrom:
			ret.from = tok
		case *timeRange:
			ret.timeRange = tok
		case *queryLimit:
			ret.limit = tok
		case *queryDump:
			ret.dump = tok
		default:
			return nil, fmt.Errorf("f(QUERY) unsupported type of args[%d], %T", i, tok)
		}
	}
	if ret.from == nil {
		return nil, errors.New("f(QUERY) 'from' should be specified")
	}
	return ret, nil
}

type querySrc struct {
	columns   []string
	from      *queryFrom
	timeRange *timeRange
	limit     *queryLimit
	dump      *queryDump
}

var _ dbSource = &querySrc{}

type queryDump struct {
	flag   bool
	escape bool
}

func srcf_dump(args ...any) (any, error) {
	if len(args) == 0 {
		return &queryDump{flag: true}, nil
	} else if len(args) == 1 {
		ret := &queryDump{flag: true}
		if b, ok := args[0].(bool); ok {
			ret.escape = b
		} else {
			return nil, fmt.Errorf("f(dump) arg should be boolean, but %T", args[1])
		}
		return ret, nil
	} else {
		return nil, fmt.Errorf("f(dump) invalid number of args (n:%d)", len(args))
	}
}

type queryLimit struct {
	limit int
}

func (lm *queryLimit) String() string {
	return strconv.Itoa(lm.limit)
}

func srcf_limit(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(limit) invalid number of args (n:%d)", len(args))
	}
	ret := &queryLimit{}
	if d, ok := args[0].(float64); ok {
		ret.limit = int(d)
	} else {
		return nil, fmt.Errorf("f(range) arg should be int, but %T", args[1])
	}
	return ret, nil
}

type queryFrom struct {
	table    string
	tag      string
	baseTime string
}

// from( 'table', 'tag' [, 'base_time_column'] )
func srcf_from(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(from) invalid number of args (n:%d)", len(args))
	}
	ret := &queryFrom{}
	if str, ok := args[0].(string); ok {
		ret.table = strings.ToUpper(str)
	} else {
		return nil, fmt.Errorf("f(from) 1st arg should be string of table, but %T", args[0])
	}
	if str, ok := args[1].(string); ok {
		ret.tag = str
	} else {
		return nil, fmt.Errorf("f(from) 2nd arg should be string of tag name, but %T", args[1])
	}
	if len(args) == 3 {
		if str, ok := args[2].(string); ok {
			ret.baseTime = str
		} else {
			return nil, fmt.Errorf("f(from) 2nd arg should be name of base time column, but %T", args[2])
		}
	} else {
		ret.baseTime = "time"
	}
	return ret, nil
}

func (si *querySrc) ToSQL() string {
	var ret string
	if si.timeRange == nil || si.timeRange.period == 0 {
		ret = si.toSql()
	} else {
		ret = si.toSqlGroup()
	}
	if si.dump != nil && si.dump.flag {
		if si.dump.escape {
			fmt.Printf("\n%s\n", url.QueryEscape(ret))
		} else {
			fmt.Printf("\n%s\n", ret)
		}
	}
	return ret
}

func (si *querySrc) toSqlGroup() string {
	table := si.from.table
	tag := si.from.tag
	baseTime := si.from.baseTime
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
			baseTime, rng.period, rng.period, baseTime, columns, table,
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
			baseTime, rng.period, rng.period, baseTime, columns, table,
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
			baseTime, rng.period, rng.period, baseTime, columns, table,
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

func (si *querySrc) toSql() string {
	if si.from == nil {
		return "ERROR 'from' function missing"
	}
	table := si.from.table
	tag := si.from.tag
	baseTime := si.from.baseTime
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
