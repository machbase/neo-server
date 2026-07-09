package tql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/spi"
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

type DataGen interface {
	gen(*Node)
}

var _ DataGen = (*DataGenMachbase)(nil)

type DataGenMachbase struct {
	task    *Task
	sqlText string
	params  []any

	resultMsg string
}

func (dc *DataGenMachbase) gen(node *Node) {
	dbm, err := dc.task.SqlDatabase()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	conn, err := dbm.Conn(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()

	stmtType := spi.DetectSQLStatementType(dc.sqlText)
	if !stmtType.IsFetch() {
		dc.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			api.MakeColumnString("MESSAGE"),
		})
		result, err := conn.ExecContext(node.task.ctx, dc.sqlText, dc.params...)
		if err != nil {
			ErrorRecord(err).Tell(node.next)
		} else {
			nrows, err := result.RowsAffected()
			if err != nil {
				ErrorRecord(err).Tell(node.next)
			} else {
				dc.resultMsg = spi.MakeUserMessage(stmtType, nrows)
				NewRecord(1, dc.resultMsg).Tell(node.next)
			}
		}
		return
	}

	rows, err := conn.QueryContext(node.task.ctx, dc.sqlText, dc.params...)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	cols := make([]*api.Column, len(columnTypes)+1)
	cols[0] = api.MakeColumnRownum()
	for i, col := range columnTypes {
		cols[i+1] = &api.Column{Name: col.Name(), DataType: spi.SqlColumnTypeToDataType(col)}
	}
	dc.task.SetResultColumns(cols)

	nrow := int64(0)
	for rows.Next() {
		nrow++
		if dc.task.shouldStop() {
			break
		}
		values := spi.MakeBuffer(columnTypes)
		if err = rows.Scan(values...); err != nil {
			ErrorRecord(err).Tell(node.next)
			break
		}
		NewRecord(nrow, values).Tell(node.next)
	}
	dc.resultMsg = spi.MakeUserMessage(stmtType, nrow)
}

type DatabaseProvider func() (*sql.DB, error)

// SQL('select ....', arg1, arg2)
// SQL(bridge('sqlite'), 'SELECT * ...', arg1, arg2)
func (x *Node) fmSql(args ...any) (any, error) {
	if x.Inflight() == nil {
		return x.fmSqlSink(args...)
	}

	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}
	tick := time.Now()
	var databaseProvider DatabaseProvider
	var sqlText string
	var sqlParams []any
	var prompt string
	var resultMsg string

	switch v := args[0].(type) {
	case string:
		databaseProvider = func() (*sql.DB, error) {
			return x.task.SqlDatabase()
		}
		sqlText = strings.TrimSuffix(strings.TrimSpace(v), ";")
		sqlParams = args[1:]
	case *bridgeName:
		databaseProvider = func() (*sql.DB, error) {
			return connector.Database(v.name)
		}
		if str, ok := args[1].(string); ok {
			sqlText = strings.TrimSuffix(strings.TrimSpace(str), ";")
		}
		sqlParams = args[2:]
		prompt = v.name
		for _, prefix := range []string{"sqlite,", "mysql,", "mssql,", "postgres,"} {
			if strings.HasPrefix(v.name, prefix) {
				prompt = strings.TrimSuffix(prefix, ",")
			}
		}
		prompt = fmt.Sprintf("SQL(%s):", prompt)
	default:
		return nil, ErrWrongTypeOfArgs("SQL", 0, "sql text or bridge('name')", args[0])
	}
	if len(sqlText) == 0 {
		return nil, fmt.Errorf("f(SQL) Empty SQL text")
	}

	dbm, err := databaseProvider()
	if err != nil {
		return nil, err
	}
	conn, err := dbm.Conn(x.task.ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	stmtType := spi.DetectSQLStatementType(sqlText)
	x.task.LogInfo("╭─", prompt, sqlText)
	switch {
	case stmtType == spi.SQLStatementTypeShow:
		resultMsg = sqlShow(x, databaseProvider, sqlText)
	case stmtType == spi.SQLStatementTypeDescribe:
		sqlText = strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(sqlText)), "DESCRIBE")
		sqlText = strings.TrimPrefix(strings.TrimSpace(sqlText), "DESC")
		sqlText = "SHOW TABLE " + sqlText
		resultMsg = sqlShow(x, databaseProvider, sqlText)
	case stmtType == spi.SQLStatementTypeExplain:
		resultMsg = sqlExplain(x, databaseProvider, sqlText)
	case stmtType.IsFetch():
		resultMsg = sqlQuery(x, stmtType, databaseProvider, sqlText, sqlParams...)
	default:
		resultMsg = sqlExec(x, stmtType, databaseProvider, sqlText, sqlParams...)
	}
	x.task.LogInfo("╰─➤", resultMsg, time.Since(tick).String())
	return nil, nil
}

func sqlExec(node *Node, stmtType spi.SQLStatementType, dbProvider DatabaseProvider, sqlText string, sqlParams ...any) string {
	var userMsg string
	dbm, err := dbProvider()
	if err != nil {
		userMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
		return userMsg
	}
	conn, err := dbm.Conn(node.task.ctx)
	if err != nil {
		userMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
		return userMsg
	}
	defer conn.Close()

	result, err := conn.ExecContext(node.task.ctx, sqlText, sqlParams...)
	if err != nil {
		userMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
	} else {
		nrows, _ := result.RowsAffected()
		userMsg = spi.MakeUserMessage(stmtType, nrows)
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			api.MakeColumnString("MESSAGE"),
		})
		NewRecord(1, userMsg).Tell(node.next)
	}
	return userMsg
}

func sqlQuery(node *Node, stmtType spi.SQLStatementType, dbProvider DatabaseProvider, sqlText string, sqlParams ...any) string {
	var userMsg string
	dbm, err := dbProvider()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	conn, err := dbm.Conn(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	defer conn.Close()

	rows, err := conn.QueryContext(node.task.ctx, sqlText, sqlParams...)
	if err != nil {
		userMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
	} else {
		defer rows.Close()
		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			userMsg = err.Error()
			ErrorRecord(err).Tell(node.next)
		} else {
			cols := make([]*api.Column, len(columnTypes)+1)
			cols[0] = api.MakeColumnRownum()
			for i, col := range columnTypes {
				cols[i+1] = &api.Column{Name: col.Name(), DataType: spi.SqlColumnTypeToDataType(col)}
			}
			node.task.SetResultColumns(cols)
			nrow := int64(0)
			for rows.Next() {
				nrow++
				if node.task.shouldStop() {
					userMsg = spi.MakeUserMessage(stmtType, nrow) + ", cancelled"
					break
				}
				values := spi.MakeBuffer(columnTypes)
				if err := rows.Scan(values...); err != nil {
					userMsg = err.Error()
					ErrorRecord(err).Tell(node.next)
					break
				}
				NewRecord(nrow, values).Tell(node.next)
			}
			userMsg = spi.MakeUserMessage(stmtType, nrow)
		}
	}
	return userMsg
}

type Explainer interface {
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func sqlExplain(node *Node, dbProvider DatabaseProvider, sqlText string) string {
	explainTokens, explainSqlText, err := splitExplainSQLText(sqlText)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	dbm, err := dbProvider()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	conn, err := dbm.Conn(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	defer conn.Close()
	resultMsg := ""
	conn.Raw(func(driverConn any) error {
		if c, ok := driverConn.(Explainer); ok {
			// Use the Explainer interface if available
			plan, err := c.Explain(node.task.ctx, explainSqlText, explainHasFullFlag(explainTokens))
			if err != nil {
				ErrorRecord(err).Tell(node.next)
				resultMsg = err.Error()
			} else {
				node.task.SetResultColumns(api.Columns{
					api.MakeColumnRownum(),
					api.MakeColumnString("PLAN"),
				})
				for n, line := range strings.Split(plan, "\n") {
					NewRecord(n, []any{line}).Tell(node.next)
				}
				resultMsg = "plan generated."
			}
		} else {
			err := fmt.Errorf("database driver does not support Explain interface")
			ErrorRecord(err).Tell(node.next)
			resultMsg = err.Error()
		}
		return nil
	})
	return resultMsg
}

func sqlShow(node *Node, dbProvider DatabaseProvider, text string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(text), ";")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		err := fmt.Errorf("f(SQL) Empty SQL text")
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	if !strings.EqualFold(fields[0], "show") {
		err := fmt.Errorf("f(SQL) invalid SHOW statement")
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}

	showAll := false
	command := ""
	args := make([]string, 0, len(fields)-1)
	for _, raw := range fields[1:] {
		switch strings.ToLower(raw) {
		case "-a", "--all":
			showAll = true
		default:
			if strings.HasPrefix(raw, "-") {
				err := fmt.Errorf("f(SQL) unsupported show option %q", raw)
				ErrorRecord(err).Tell(node.next)
				return err.Error()
			}
			if command == "" {
				command = strings.ToLower(raw)
			} else {
				args = append(args, raw)
			}
		}
	}

	if command == "" {
		err := fmt.Errorf("f(SQL) missing show command")
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}

	validateArgs := func(want string, exact int) error {
		if len(args) != exact {
			return fmt.Errorf("f(SQL) show %s expects %d argument(s), got %d", want, exact, len(args))
		}
		return nil
	}
	validateNoAll := func() error {
		if showAll {
			return fmt.Errorf("f(SQL) show %s does not support -a/--all", command)
		}
		return nil
	}

	dbm, err := dbProvider()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	conn, err := dbm.Conn(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	defer conn.Close()
	apiConn := spi.WrapSqlConn(conn)

	switch command {
	case "info":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryServerInfo())
		}
	case "tables":
		err = validateArgs(command, 0)
		if err == nil {
			return yieldResultSet(node, spi.QueryTables(node.task.ctx, apiConn, showAll))
		}
	case "table":
		err = validateArgs(command, 1)
		if err == nil {
			return yieldResultSet(node, spi.QueryTable(node.task.ctx, apiConn, args[0], showAll))
		}
	case "indexes":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryIndexes(node.task.ctx, apiConn))
		}
	case "index":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 1)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryIndex(node.task.ctx, apiConn, args[0]))
		}
	case "lsm":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryLsmIndexes(node.task.ctx, apiConn))
		}
	case "tags":
		err = validateNoAll()
		if err == nil && len(args) < 1 {
			err = fmt.Errorf("f(SQL) show tags expects at least 1 argument, got %d", len(args))
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryTags(node.task.ctx, apiConn, args[0], args[1:]...))
		}
	case "indexgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryIndexGap(node.task.ctx, apiConn))
		}
	case "tagindexgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryTagIndexGap(node.task.ctx, apiConn))
		}
	case "rollupgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryRollupGap(node.task.ctx, apiConn))
		}
	case "sessions":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QuerySessions(node.task.ctx, apiConn))
		}
	case "statements":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryStatements(node.task.ctx, apiConn))
		}
	case "storage":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryStorage(node.task.ctx, apiConn))
		}
	case "table-usage":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryTableUsage(node.task.ctx, apiConn))
		}
	case "license":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.QueryLicense(node.task.ctx, apiConn))
		}
	default:
		err = fmt.Errorf("f(SQL) unsupported show command %q", command)
	}

	ErrorRecord(err).Tell(node.next)
	return err.Error()
}

func splitExplainSQLText(sqlText string) ([]string, string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, "", fmt.Errorf("f(SQL) Empty SQL text")
	}
	if !strings.EqualFold(fields[0], "explain") {
		return nil, trimmed, nil
	}

	tokens := make([]string, 0, len(fields))
	start := -1
	for i := 1; i < len(fields); i++ {
		tok := fields[i]
		if tok == "--" {
			if i+1 >= len(fields) {
				return nil, "", fmt.Errorf("f(SQL) missing statement after explain options")
			}
			start = i + 1
			break
		}
		if isSQLStatementStart(tok) {
			start = i
			break
		}
		tokens = append(tokens, tok)
	}
	if start == -1 {
		return nil, "", fmt.Errorf("f(SQL) missing statement after explain options")
	}
	return tokens, strings.Join(fields[start:], " "), nil
}

func explainHasFullFlag(tokens []string) bool {
	for _, tok := range tokens {
		if strings.EqualFold(tok, "full") || tok == "--full" || tok == "-f" {
			return true
		}
	}
	return false
}

func isSQLStatementStart(tok string) bool {
	switch spi.DetectSQLStatementType(tok) {
	case spi.SQLStatementTypeSelect,
		spi.SQLStatementTypeInsert,
		spi.SQLStatementTypeUpdate,
		spi.SQLStatementTypeDelete,
		spi.SQLStatementTypeCreate,
		spi.SQLStatementTypeDrop,
		spi.SQLStatementTypeAlter,
		spi.SQLStatementTypeDescribe,
		spi.SQLStatementTypeCommonTableExpression,
		spi.SQLStatementTypeShow:
		return true
	default:
		return false
	}
}

func yieldResultSet[T spi.ResultSet](node *Node, nfo T) string {
	node.task.SetResultColumns(append(api.Columns{api.MakeColumnRownum()}, nfo.Columns()...))
	if err := nfo.Err(); err != nil {
		ErrorRecord(err).Tell(node.next)
		return err.Error()
	}
	nrow := int64(0)
	nfo.Iter(func(values []any) bool {
		nrow++
		if err := nfo.Err(); err != nil {
			ErrorRecord(err).Tell(node.next)
			return false
		}
		if node.task.shouldStop() {
			return false
		}
		NewRecord(nrow, values).Tell(node.next)
		return true
	})
	return nfo.Message()
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
