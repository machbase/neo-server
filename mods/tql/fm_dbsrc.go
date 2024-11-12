package tql

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (node *Node) fmSqlSelect(args ...any) (any, error) {
	ret, err := node.sqlbuilder("SQL_SELECT", args...)
	if err != nil {
		return nil, err
	}
	ret.version = 1

	tick := time.Now()
	var ds *DataGenMachbase
	var sqlText string
	defer func() {
		if ds != nil {
			node.task.LogTrace("╰─➤", ds.resultMsg, time.Since(tick).String())
		} else {
			node.task.LogTrace("SQL_SELECT dump:", sqlText)
		}
	}()

	if ret.dump == nil || !ret.dump.Flag {
		node.task.LogTrace("╭─", ret.ToSQL())
		ds = &DataGenMachbase{task: node.task, sqlText: ret.ToSQL()}
		ds.gen(node)
	} else {
		if ret.between != nil {
			if ret.between.HasPeriod() {
				sqlText = ret.toSqlGroup()
			} else {
				sqlText = ret.toSql()
			}
		}
		if ret.dump.Escape {
			sqlText = url.QueryEscape(sqlText)
		}
		NewRecord("SQLDUMP", sqlText).Tell(node.next)
		return nil, nil
	}
	return nil, nil
}

// QUERY('value', 'STDDEV(val)', from('example', 'sig.1'), range('last', '10s', '1s'), limit(100000) )
func (node *Node) fmQuery(args ...any) (any, error) {
	ret, err := node.sqlbuilder("QUERY", args...)
	if err != nil {
		return nil, err
	}
	tick := time.Now()
	var ds *DataGenMachbase
	var sqlText string
	defer func() {
		if ds != nil {
			node.task.LogTrace("╰─➤", ds.resultMsg, time.Since(tick).String())
		} else {
			node.task.LogTrace("QUERY dump:", sqlText)
		}
	}()

	if ret.dump == nil || !ret.dump.Flag {
		node.task.LogTrace("╭─", ret.ToSQL())
		ds = &DataGenMachbase{task: node.task, sqlText: ret.ToSQL()}
		ds.gen(node)
	} else {
		if ret.between != nil {
			if ret.between.HasPeriod() {
				sqlText = ret.toSqlGroup()
			} else {
				sqlText = ret.toSql()
			}
		}
		if ret.dump.Escape {
			sqlText = url.QueryEscape(sqlText)
		}
		NewRecord("SQLDUMP", sqlText).Tell(node.next)
		return nil, nil
	}
	return nil, nil
}

func (node *Node) sqlbuilder(name string, args ...any) (*querySource, error) {
	between, _ := node.fmBetween("last-1s", "last")
	ret := &querySource{
		columns: []string{},
		between: between,
		limit:   node.fmLimit(1000000),
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
			return nil, ErrArgs(name, i, fmt.Sprintf("unsupported args[%d] %T", i, tok))
		}
	}
	if ret.from == nil {
		return nil, ErrArgs(name, 0, "'from' should be specified")
	}

	return ret, nil
}

type querySource struct {
	version int
	columns []string
	from    *QueryFrom
	between *QueryBetween
	limit   *QueryLimit
	dump    *QueryDump
}

func (si *querySource) ToSQL() string {
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
	return ret
}

func (si *querySource) toSql() string {
	table := strings.ToUpper(si.from.Table)
	tag := si.from.Tag
	baseTime := si.from.BaseTime
	baseName := si.from.BaseName
	ret := ""
	columns := "value"
	if len(si.columns) > 0 {
		columns = strings.Join(si.columns, ", ")
	}
	aPart := si.between.BeginPart(table, tag)
	bPart := si.between.EndPart(table, tag)

	if si.version == 1 {
		ret = fmt.Sprintf(`SELECT %s FROM %s WHERE %s = '%s' AND %s BETWEEN %s AND %s LIMIT %d, %d`,
			columns, table,
			baseName, tag,
			baseTime, aPart, bPart,
			si.limit.Offset, si.limit.Limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s WHERE %s = '%s' AND %s BETWEEN %s AND %s LIMIT %d, %d`,
			baseTime, columns, table,
			baseName, tag,
			baseTime, aPart, bPart,
			si.limit.Offset, si.limit.Limit,
		)
	}

	return ret
}

func (si *querySource) toSqlGroup() string {
	table := strings.ToUpper(si.from.Table)
	tag := si.from.Tag
	baseTime := si.from.BaseTime
	baseName := si.from.BaseName
	ret := ""
	columns := "value"
	if si.version == 1 {
		if len(si.columns) > 0 {
			arr := make([]string, len(si.columns))
			for i, c := range si.columns {
				if c == baseTime {
					arr[i] = fmt.Sprintf("from_timestamp(round(to_timestamp(%s)/%d)*%d) %s",
						baseTime, si.between.Period(), si.between.Period(), baseTime)
				} else {
					arr[i] = c
				}
			}
			columns = strings.Join(arr, ", ")
		}
	} else {
		if len(si.columns) > 0 {
			columns = strings.Join(si.columns, ", ")
		}
	}
	aPart := si.between.BeginPart(table, tag)
	bPart := si.between.EndPart(table, tag)

	if si.version == 1 {
		ret = fmt.Sprintf(`SELECT %s FROM %s WHERE %s = '%s' AND %s BETWEEN %s AND %s GROUP BY %s ORDER BY %s LIMIT %d, %d`,
			columns, table,
			baseName, tag,
			baseTime, aPart, bPart,
			baseTime,
			baseTime,
			si.limit.Offset, si.limit.Limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s WHERE %s = '%s' AND %s BETWEEN %s AND %s GROUP BY %s ORDER BY %s LIMIT %d, %d`,
			baseTime, si.between.Period(), si.between.Period(), baseTime, columns, table,
			baseName, tag,
			baseTime, aPart, bPart,
			baseTime,
			baseTime,
			si.limit.Offset, si.limit.Limit,
		)
	}
	return ret
}

// SQL('select ....', arg1, arg2)
// SQL(bridge('sqlite'), 'SELECT * ...', arg1, arg2)
func (x *Node) fmSql(args ...any) (any, error) {
	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}
	tick := time.Now()
	switch v := args[0].(type) {
	case string:
		if dg, ok := parseDataGenCommands(v, x, args[1:]); ok {
			x.task.LogInfof("╭─ %s", v)
			dg.gen(x)
			x.task.LogInfof("╰─➤ %s", time.Since(tick).String())
		} else {
			ds := &DataGenMachbase{task: x.task, sqlText: v, params: args[1:]}
			x.task.LogInfof("╭─ %s", v)
			ds.gen(x)
			x.task.LogInfof("╰─➤ %s %s", ds.resultMsg, time.Since(tick).String())
		}
		return nil, nil
	case *bridgeName:
		if len(args) == 0 {
			return nil, ErrWrongTypeOfArgs("SQL", 1, "sql text", args[1])
		}
		if sqlText, ok := args[1].(string); ok {
			x.task.LogInfof("╭─ SQL(%s): %s", v.name, sqlText)
			ds := &DataGenBridge{task: x.task, name: v.name, sqlText: sqlText, params: args[2:]}
			ds.gen(x)
			x.task.LogInfof("╰─➤ Elapsed %s %s", ds.resultMsg, time.Since(tick).String())
			return nil, nil
		}
	}
	return nil, ErrWrongTypeOfArgs("SQL", 0, "sql text or bridge('name')", args[0])
}

type QueryFrom struct {
	Table    string
	Tag      string
	BaseTime string
	BaseName string
}

func (x *Node) fmFrom(table string, tag string, args ...string) *QueryFrom {
	ret := &QueryFrom{
		Table:    table,
		Tag:      tag,
		BaseTime: "time",
		BaseName: "name",
	}
	if len(args) > 0 {
		ret.BaseTime = args[0]
	}
	if len(args) > 1 {
		ret.BaseName = args[1]
	}
	return ret
}

type QueryLimit struct {
	Offset int
	Limit  int
}

// limit([offset ,] limit)
func (x *Node) fmLimit(args ...int) *QueryLimit {
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

func (x *Node) fmDump(args ...bool) *QueryDump {
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

func (x *Node) fmBetween(begin any, end any, period ...any) (*QueryBetween, error) {
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
		return nil, ErrWrongTypeOfArgs("between", 0, "time, 'now' or 'last", val)
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
		return nil, ErrWrongTypeOfArgs("between", 1, "time, 'now' or 'last", val)
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
		return nil, ErrWrongTypeOfArgs("between", 2, "duration", val)
	}
	return ret, nil
}
