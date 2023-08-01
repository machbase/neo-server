package maps

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/tql/conv"
)

type QueryFrom struct {
	Table    string
	Tag      string
	BaseTime string
}

func ToFrom(table string, tag string, baseTime ...string) *QueryFrom {
	ret := &QueryFrom{
		Table:    table,
		Tag:      tag,
		BaseTime: "time",
	}
	if len(baseTime) > 0 {
		ret.BaseTime = baseTime[0]
	}
	return ret
}

type QueryLimit struct {
	Offset int
	Limit  int
}

// limit([offset ,] limit)
func ToLimit(args ...int) *QueryLimit {
	ret := &QueryLimit{}
	if len(args) == 2 {
		ret.Offset = args[0]
		ret.Limit = args[1]
	} else {
		ret.Limit = args[0]
	}
	return ret
}

type QueryDump struct {
	Flag   bool
	Escape bool
}

func ToDump(args ...bool) *QueryDump {
	ret := &QueryDump{}
	if len(args) == 0 {
		return ret
	}
	if len(args) >= 1 {
		ret.Flag = args[0]
	}
	if len(args) >= 2 {
		ret.Escape = args[1]
	}
	return ret
}

type QueryBetween struct {
	aStr   string
	aDur   time.Duration
	aTime  time.Time
	bStr   string
	bDur   time.Duration
	bTime  time.Time
	period time.Duration
}

func (qb *QueryBetween) HasPeriod() bool {
	return qb.period > 0
}

func (qb *QueryBetween) Period() time.Duration {
	return qb.period
}

func (qb *QueryBetween) BeginPart(table string, tag string) string {
	return stringBetweenPart(qb.aStr, qb.aDur, qb.aTime, table, tag)
}

func (qb *QueryBetween) EndPart(table string, tag string) string {
	return stringBetweenPart(qb.bStr, qb.bDur, qb.bTime, table, tag)
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

func parseBetweenTime(str string) (string, time.Duration, error) {
	str = strings.TrimSpace(strings.ToLower(str))
	var dur time.Duration
	var err error
	if strings.HasPrefix(str, "now") {
		remain := strings.TrimSpace(str[3:])
		if len(remain) > 0 {
			dur, err = time.ParseDuration(remain)
			if err != nil {
				return "", 0, err
			}
		}
		return "now", dur, nil
	} else if strings.HasPrefix(str, "last") {
		remain := strings.TrimSpace(str[4:])
		if len(remain) > 0 {
			dur, err = time.ParseDuration(remain)
			if err != nil {
				return "", 0, err
			}
		}
		return "last", dur, nil
	} else {
		return "", 0, fmt.Errorf("invalid between expression")
	}
}

func ToBetween(begin any, end any, period ...any) (*QueryBetween, error) {
	ret := &QueryBetween{}
	switch val := begin.(type) {
	case string:
		tok, dur, err := parseBetweenTime(val)
		if err != nil {
			return nil, err
		}
		ret.aStr = tok
		ret.aDur = dur
	case float64:
		ret.aTime = time.Unix(0, int64(val))
	case time.Time:
		ret.aTime = val
	default:
		return nil, conv.ErrWrongTypeOfArgs("between", 0, "time, 'now' or 'last", val)
	}
	switch val := end.(type) {
	case string:
		tok, dur, err := parseBetweenTime(val)
		if err != nil {
			return nil, err
		}
		ret.bStr = tok
		ret.bDur = dur
	case float64:
		ret.bTime = time.Unix(0, int64(val))
	case time.Time:
		ret.bTime = val
	default:
		return nil, conv.ErrWrongTypeOfArgs("between", 1, "time, 'now' or 'last", val)
	}
	if len(period) == 0 {
		return ret, nil
	}
	switch val := period[0].(type) {
	case string:
		if d, err := time.ParseDuration(val); err == nil {
			ret.period = d
		} else {
			return nil, err
		}
	case float64:
		ret.period = time.Duration(int64(val))
	default:
		return nil, conv.ErrWrongTypeOfArgs("between", 2, "duration", val)
	}
	return ret, nil
}

// QUERY('value', 'STDDEV(val)', from('example', 'sig.1'), range('last', '10s', '1s'), limit(100000) )
func ToQuery(args ...any) (*Query, error) {
	between, _ := ToBetween("last-1s", "last")
	ret := &Query{
		columns: []string{},
		between: between,
		limit:   ToLimit(1000000),
	}
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.columns = append(ret.columns, tok)
		case *QueryFrom:
			ret.from = tok
		case *QueryBetween:
			ret.between = tok
		case *QueryLimit:
			ret.limit = tok
		case *QueryDump:
			ret.dump = tok
		default:
			return nil, conv.ErrArgs("QUERY", i, fmt.Sprintf("unsupported args[%d] %T", i, tok))
		}
	}
	if ret.from == nil {
		return nil, conv.ErrArgs("QUERY", 0, "'from' should be specified")
	}
	return ret, nil
}

type Query struct {
	columns []string
	from    *QueryFrom
	between *QueryBetween
	limit   *QueryLimit
	dump    *QueryDump
}

func (si *Query) ToSQL() string {
	var ret string
	if si.from == nil {
		return "ERROR 'from()' missing"
	}
	if si.between != nil {
		if si.between.HasPeriod() {
			ret = si.toSqlGroup()
		} else {
			ret = si.toSql()
		}
	}
	if si.dump != nil && si.dump.Flag {
		if si.dump.Escape {
			fmt.Printf("\n%s\n", url.QueryEscape(ret))
		} else {
			fmt.Printf("\n%s\n", ret)
		}
	}
	return ret
}

func (si *Query) toSql() string {
	table := strings.ToUpper(si.from.Table)
	tag := si.from.Tag
	baseTime := si.from.BaseTime
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	aPart := si.between.BeginPart(table, tag)
	bPart := si.between.EndPart(table, tag)

	ret = fmt.Sprintf(`SELECT %s, %s FROM %s WHERE name = '%s' AND %s BETWEEN %s AND %s LIMIT %d, %d`,
		baseTime, columns, table,
		tag,
		baseTime, aPart, bPart,
		si.limit.Offset, si.limit.Limit,
	)

	return ret
}

func (si *Query) toSqlGroup() string {
	table := strings.ToUpper(si.from.Table)
	tag := si.from.Tag
	baseTime := si.from.BaseTime
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	aPart := si.between.BeginPart(table, tag)
	bPart := si.between.EndPart(table, tag)

	ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
		WHERE
			name = '%s'
		AND %s BETWEEN %s AND %s
		GROUP BY %s
		ORDER BY %s
		LIMIT %d, %d
		`,
		baseTime, si.between.Period(), si.between.Period(), baseTime, columns, table,
		tag,
		baseTime, aPart, bPart,
		baseTime,
		baseTime,
		si.limit.Offset, si.limit.Limit,
	)
	return ret
}

type Sql struct {
	text string
}

// SQL('select ....')
func ToSql(text string) *Sql {
	return &Sql{text: text}
}

func (s *Sql) ToSQL() string {
	return s.text
}
