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
		columns: []string{},
		between: &queryBetween{aStr: "last", aDur: time.Duration(-1000000000), bStr: "last", period: 0},
		limit:   &queryLimit{limit: 1000000},
	}
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.columns = append(ret.columns, tok)
		case *queryFrom:
			ret.from = tok
		case *queryBetween:
			ret.between = tok
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
	columns []string
	from    *queryFrom
	between *queryBetween
	limit   *queryLimit
	dump    *queryDump
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
	offset int
	limit  int
}

func (lm *queryLimit) String() string {
	return strconv.Itoa(lm.limit)
}

// len([offset ,] limit)
func srcf_limit(args ...any) (any, error) {
	lenArgs := len(args)
	if lenArgs != 1 && lenArgs != 2 {
		return nil, fmt.Errorf("f(limit) invalid number of args (n:%d)", len(args))
	}
	ret := &queryLimit{}
	idxArgs := 0
	if lenArgs == 2 {
		if d, ok := args[idxArgs].(float64); ok {
			ret.offset = int(d)
		} else {
			return nil, fmt.Errorf("f(range) offset should be int, but %T", args[1])
		}
		idxArgs++
	}
	if d, ok := args[idxArgs].(float64); ok {
		ret.limit = int(d)
	} else {
		return nil, fmt.Errorf("f(range) limit should be int, but %T", args[1])
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

type queryBetween struct {
	aStr   string
	aDur   time.Duration
	aTime  time.Time
	bStr   string
	bDur   time.Duration
	bTime  time.Time
	period time.Duration
}

func parseBetweenTime(str string) (string, time.Duration, error) {
	str = strings.TrimSpace(strings.ToLower(str))
	var dur time.Duration
	var err error
	if strings.HasPrefix(str, "now") {
		remain := strings.TrimSpace(str[3:])
		if len(remain) > 0 {
			dur, err = time.ParseDuration(remain)
			if err != nil {
				return "", 0, fmt.Errorf("f(between) %s", err.Error())
			}
		}
		return "now", dur, nil
	} else if strings.HasPrefix(str, "last") {
		remain := strings.TrimSpace(str[4:])
		if len(remain) > 0 {
			dur, err = time.ParseDuration(remain)
			if err != nil {
				return "", 0, fmt.Errorf("f(between) %s", err.Error())
			}
		}
		return "last", dur, nil
	} else {
		return "", 0, fmt.Errorf("f(between) invalid time expression")
	}
}

func srcf_between(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(between) invalid number of args (n:%d)", len(args))
	}
	ret := &queryBetween{}
	if str, ok := args[0].(string); ok {
		tok, dur, err := parseBetweenTime(str)
		if err != nil {
			return nil, err
		}
		ret.aStr = tok
		ret.aDur = dur
	} else {
		if num, ok := args[0].(float64); ok {
			ret.aTime = time.Unix(0, int64(num))
		} else {
			if ts, ok := args[0].(time.Time); ok {
				ret.aTime = ts
			} else {
				return nil, fmt.Errorf("f(between) 1st arg should be time or 'now', 'last', but %T", args[0])
			}
		}
	}

	if str, ok := args[1].(string); ok {
		tok, dur, err := parseBetweenTime(str)
		if err != nil {
			return nil, err
		}
		ret.bStr = tok
		ret.bDur = dur
	} else {
		if num, ok := args[1].(float64); ok {
			ret.bTime = time.Unix(0, int64(num))
		} else {
			if ts, ok := args[1].(time.Time); ok {
				ret.bTime = ts
			} else {
				return nil, fmt.Errorf("f(between) 2nd arg should be time or 'now', 'last', but %T", args[1])
			}
		}
	}

	if len(args) == 2 {
		return ret, nil
	}

	if str, ok := args[2].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.period = d
		} else {
			return nil, fmt.Errorf("f(between) 3rd arg should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.period = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(between) 3rd arg should be duration, but %T", args[1])
	}
	return ret, nil
}

func (si *querySrc) ToSQL() string {
	var ret string
	if si.from == nil {
		return "ERROR 'from()' missing"
	}
	if si.between != nil {
		if si.between.period == 0 {
			ret = si.toSql()
		} else {
			ret = si.toSqlGroup()
		}
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

func stringBetweenDuration(dur time.Duration) string {
	if dur == 0 {
		return ""
	} else if dur < 0 {
		return fmt.Sprintf("%d", dur)
	} else {
		return fmt.Sprintf("+%d", dur)
	}
}

func stringBetweenPart(str string, dur time.Duration, ts time.Time, table string, tag string) string {
	if str == "last" {
		return fmt.Sprintf("(SELECT MAX_TIME%s FROM V$%s_STAT WHERE name = '%s')", stringBetweenDuration(dur), table, tag)
	} else if str == "now" && dur == 0 {
		return "now"
	} else if str == "now" {
		return fmt.Sprintf("(now%s)", stringBetweenDuration(dur))
	} else {
		return fmt.Sprintf("%d", ts.UnixNano())
	}
}

func (si *querySrc) toSql() string {
	table := si.from.table
	tag := si.from.tag
	baseTime := si.from.baseTime
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	bw := si.between
	aPart := stringBetweenPart(bw.aStr, bw.aDur, bw.aTime, table, tag)
	bPart := stringBetweenPart(bw.bStr, bw.bDur, bw.bTime, table, tag)

	ret = fmt.Sprintf(`SELECT %s, %s FROM %s WHERE name = '%s' AND %s BETWEEN %s AND %s LIMIT %d, %d`,
		baseTime, columns, table,
		tag,
		baseTime, aPart, bPart,
		si.limit.offset, si.limit.limit,
	)

	return ret
}

func (si *querySrc) toSqlGroup() string {
	table := si.from.table
	tag := si.from.tag
	baseTime := si.from.baseTime
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	bw := si.between
	aPart := stringBetweenPart(bw.aStr, bw.aDur, bw.aTime, table, tag)
	bPart := stringBetweenPart(bw.bStr, bw.bDur, bw.bTime, table, tag)

	ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
		WHERE
			name = '%s'
		AND %s BETWEEN %s AND %s
		GROUP BY %s
		ORDER BY %s
		LIMIT %d, %d
		`,
		baseTime, bw.period, bw.period, baseTime, columns, table,
		tag,
		baseTime, aPart, bPart,
		baseTime,
		baseTime,
		si.limit.offset, si.limit.limit,
	)
	return ret
}
