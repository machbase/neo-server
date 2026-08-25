package tql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/bridge"
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
			node.ensureRuntime().LogTrace("╰─➤", ds.resultMsg, time.Since(tick).String())
		} else {
			node.ensureRuntime().LogTrace("SQL_SELECT dump:", sqlText)
		}
	}()

	if ret.dump == nil || !ret.dump.Flag {
		node.ensureRuntime().LogTrace("╭─", ret.ToSQL())
		ds = &DataGenMachbase{sqlText: ret.ToSQL()}
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
		node.emit(NewRecord("SQLDUMP", sqlText))
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
			node.ensureRuntime().LogTrace("╰─➤", ds.resultMsg, time.Since(tick).String())
		} else {
			node.ensureRuntime().LogTrace("QUERY dump:", sqlText)
		}
	}()

	if ret.dump == nil || !ret.dump.Flag {
		node.ensureRuntime().LogTrace("╭─", ret.ToSQL())
		ds = &DataGenMachbase{sqlText: ret.ToSQL()}
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
		node.emit(NewRecord("SQLDUMP", sqlText))
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
	sqlText string
	params  []any

	resultMsg string
}

func (dc *DataGenMachbase) gen(node *Node) {
	runtime := node.ensureRuntime()
	conn, err := spi.Connect(runtime.Context(), runtime.ConsoleUser())
	if err != nil {
		node.emit(ErrorRecord(err))
		return
	}
	defer conn.Close()

	stmtType := spi.DetectSQLStatementType(dc.sqlText)
	if !stmtType.IsFetch() {
		runtime.SetResultColumns(client.Columns{
			client.MakeColumnRownum(),
			client.MakeColumnString("MESSAGE"),
		})
		result, err := conn.ExecContext(runtime.Context(), dc.sqlText, dc.params...)
		if err != nil {
			node.emit(ErrorRecord(err))
		} else {
			nrows, err := result.RowsAffected()
			if err != nil {
				node.emit(ErrorRecord(err))
			} else {
				dc.resultMsg = spi.MakeUserMessage(stmtType, nrows)
				node.emit(NewRecord(1, dc.resultMsg))
			}
		}
		return
	}

	rows, err := conn.QueryContext(runtime.Context(), dc.sqlText, dc.params...)
	if err != nil {
		node.emit(ErrorRecord(err))
		return
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		node.emit(ErrorRecord(err))
		return
	}
	cols := make([]*client.Column, len(columnTypes)+1)
	cols[0] = client.MakeColumnRownum()
	for i, col := range columnTypes {
		cols[i+1] = client.NewColumnWithType(col)
	}
	runtime.SetResultColumns(cols)

	nrow := int64(0)
	for rows.Next() {
		nrow++
		if runtime.ShouldStop() {
			break
		}
		values := spi.MakeBuffer(columnTypes)
		if err = rows.Scan(values...); err != nil {
			node.emit(ErrorRecord(err))
			break
		}
		for i := range values {
			values[i] = client.Unbox(values[i])
		}
		node.emit(NewRecord(nrow, values))
	}
	dc.resultMsg = spi.MakeUserMessage(stmtType, nrow)
}

type bridgeName struct {
	name string
}

// bridge('name')
func (x *Node) fmBridge(name string) *bridgeName {
	return &bridgeName{name: name}
}

type useDatabase struct {
	use string
}

// use('database')
func (x *Node) fmUse(dbname string) *useDatabase {
	return &useDatabase{use: dbname}
}

// SQL('select ....', arg1, arg2)
// SQL(bridge('sqlite'), 'SELECT * ...', arg1, arg2)
func (x *Node) fmSql(args ...any) (any, error) {
	if x.role == StageSink {
		return x.fmSqlSink(args...)
	}
	runtime := x.ensureRuntime()

	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}
	tick := time.Now()
	var sqlText string
	var sqlParams []any

	var use string
	var bridge string

	var conn *sql.Conn
	var resultMsg string

loop:
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			sqlText = strings.TrimSuffix(strings.TrimSpace(v), ";")
			sqlParams = args[i+1:]
			break loop
		case *useDatabase:
			use = strings.TrimSpace(strings.ToUpper(v.use))
		case *bridgeName:
			bridge = v.name
		}
	}
	if len(sqlText) == 0 {
		return nil, fmt.Errorf("f(SQL) Empty SQL text")
	}

	if bridge == "" {
		if c, err := spi.Connect(runtime.Context(), runtime.ConsoleUser()); err != nil {
			return nil, err
		} else {
			conn = c
		}
		defer conn.Close()
	} else {
		dbm, err := connector.Database(bridge)
		if err != nil {
			return nil, err
		}
		conn, err = dbm.Conn(runtime.Context())
		if err != nil {
			return nil, err
		}
		defer conn.Close()
	}
	if conn == nil {
		return nil, errors.New("f(SQL) failed to connect to database")
	}

	if use != "" {
		_, err := conn.ExecContext(runtime.Context(), fmt.Sprintf("USE %s", use))
		if err != nil {
			return nil, fmt.Errorf("f(SQL) failed to use database %s: %v", use, err)
		}
	}

	prompt := "SQL("
	flags := []string{}
	if bridge != "" {
		for _, prefix := range []string{"sqlite,", "mysql,", "mssql,", "postgres,"} {
			if strings.HasPrefix(bridge, prefix) {
				flags = append(flags, "BRIDGE "+strings.TrimSuffix(bridge, ","))
				break
			}
		}
	}
	if use != "" {
		flags = append(flags, "USE "+use)
	}
	if len(flags) > 0 {
		prompt = prompt + strings.Join(flags, ", ") + ")"
	} else {
		prompt = prompt + ")"
	}

	stmtType := spi.DetectSQLStatementType(sqlText)
	runtime.LogInfo("╭─", prompt, sqlText)
	switch {
	case stmtType == spi.SQLStatementTypeShow:
		resultMsg = sqlShow(x, conn, sqlText)
	case stmtType == spi.SQLStatementTypeDescribe:
		sqlText = strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(sqlText)), "DESCRIBE")
		sqlText = strings.TrimPrefix(strings.TrimSpace(sqlText), "DESC")
		sqlText = "SHOW TABLE " + sqlText
		resultMsg = sqlShow(x, conn, sqlText)
	case stmtType == spi.SQLStatementTypeExplain:
		resultMsg = sqlExplain(x, conn, sqlText)
	case stmtType.IsFetch():
		resultMsg = sqlQuery(x, stmtType, conn, sqlText, sqlParams...)
	default:
		resultMsg = sqlExec(x, stmtType, conn, sqlText, sqlParams...)
	}
	runtime.LogInfo("╰─➤", resultMsg, time.Since(tick).String())
	return nil, nil
}

func sqlExec(node *Node, stmtType spi.SQLStatementType, conn *sql.Conn, sqlText string, sqlParams ...any) string {
	runtime := node.ensureRuntime()
	var userMsg string
	result, err := conn.ExecContext(runtime.Context(), sqlText, sqlParams...)
	if err != nil {
		userMsg = err.Error()
		node.emit(ErrorRecord(err))
	} else {
		nrows, _ := result.RowsAffected()
		userMsg = spi.MakeUserMessage(stmtType, nrows)
		runtime.SetResultColumns(client.Columns{
			client.MakeColumnRownum(),
			client.MakeColumnString("MESSAGE"),
		})
		node.emit(NewRecord(1, userMsg))
	}
	return userMsg
}

func sqlQuery(node *Node, stmtType spi.SQLStatementType, conn *sql.Conn, sqlText string, sqlParams ...any) string {
	runtime := node.ensureRuntime()
	var userMsg string
	rows, err := conn.QueryContext(runtime.Context(), sqlText, sqlParams...)
	if err != nil {
		userMsg = err.Error()
		node.emit(ErrorRecord(err))
	} else {
		defer rows.Close()
		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			userMsg = err.Error()
			node.emit(ErrorRecord(err))
		} else {
			cols := make([]*client.Column, len(columnTypes)+1)
			cols[0] = client.MakeColumnRownum()
			for i, col := range columnTypes {
				cols[i+1] = client.NewColumnWithType(col)
			}
			runtime.SetResultColumns(cols)
			nrow := int64(0)
			for rows.Next() {
				nrow++
				if runtime.ShouldStop() {
					userMsg = spi.MakeUserMessage(stmtType, nrow) + ", cancelled"
					break
				}
				values := spi.MakeBuffer(columnTypes)
				if err := rows.Scan(values...); err != nil {
					userMsg = err.Error()
					node.emit(ErrorRecord(err))
					break
				}
				for i := range values {
					values[i] = client.Unbox(values[i])
				}
				node.emit(NewRecord(nrow, values))
			}
			userMsg = spi.MakeUserMessage(stmtType, nrow)
		}
	}
	return userMsg
}

type Explainer interface {
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func sqlExplain(node *Node, conn *sql.Conn, sqlText string) string {
	runtime := node.ensureRuntime()
	explainTokens, explainSqlText, err := splitExplainSQLText(sqlText)
	if err != nil {
		node.emit(ErrorRecord(err))
		return err.Error()
	}
	resultMsg := ""
	conn.Raw(func(driverConn any) error {
		if c, ok := driverConn.(Explainer); ok {
			// Use the Explainer interface if available
			plan, err := c.Explain(runtime.Context(), explainSqlText, explainHasFullFlag(explainTokens))
			if err != nil {
				node.emit(ErrorRecord(err))
				resultMsg = err.Error()
			} else {
				runtime.SetResultColumns(client.Columns{
					client.MakeColumnRownum(),
					client.MakeColumnString("PLAN"),
				})
				for n, line := range strings.Split(plan, "\n") {
					node.emit(NewRecord(n, []any{line}))
				}
				resultMsg = "plan generated."
			}
		} else {
			err := fmt.Errorf("database driver does not support Explain interface")
			node.emit(ErrorRecord(err))
			resultMsg = err.Error()
		}
		return nil
	})
	return resultMsg
}

func sqlShow(node *Node, conn *sql.Conn, text string) string {
	runtime := node.ensureRuntime()
	trimmed := strings.TrimSuffix(strings.TrimSpace(text), ";")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		err := fmt.Errorf("f(SQL) Empty SQL text")
		node.emit(ErrorRecord(err))
		return err.Error()
	}
	if !strings.EqualFold(fields[0], "show") {
		err := fmt.Errorf("f(SQL) invalid SHOW statement")
		node.emit(ErrorRecord(err))
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
				node.emit(ErrorRecord(err))
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
		node.emit(ErrorRecord(err))
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

	var err error
	switch command {
	case "info":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowInfo())
		}
	case "license":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowLicense(runtime.Context(), conn))
		}
	case "ports":
		err = validateNoAll()
		portType := ""
		if err == nil {
			if len(args) > 1 {
				err = fmt.Errorf("f(SQL) show ports expects at most 1 argument, got %d", len(args))
			} else if len(args) == 1 {
				portType = args[0]
			}
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowPorts(portType))
		}
	case "users":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowUsers(runtime.Context(), conn))
		}
	case "databases":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowDatabases(runtime.Context(), conn))
		}
	case "tables":
		err = validateArgs(command, 0)
		if err == nil {
			return yieldResultSet(node, spi.ShowTables(runtime.Context(), conn, showAll))
		}
	case "meta-tables":
		err = validateArgs(command, 0)
		if err == nil {
			return yieldResultSet(node, spi.ShowMetaTables(runtime.Context(), conn))
		}
	case "virtual-tables":
		err = validateArgs(command, 0)
		if err == nil {
			return yieldResultSet(node, spi.ShowVirtualTables(runtime.Context(), conn))
		}
	case "table":
		err = validateArgs(command, 1)
		if err == nil {
			return yieldResultSet(node, spi.ShowTable(runtime.Context(), conn, "MACHBASEDB", runtime.ConsoleUser(), args[0], showAll))
		}
	case "indexes":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowIndexes(runtime.Context(), conn))
		}
	case "index":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 1)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowIndex(runtime.Context(), conn, args[0]))
		}
	case "lsm":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowLsm(runtime.Context(), conn))
		}
	case "tags":
		err = validateNoAll()
		if err == nil && len(args) < 1 {
			err = fmt.Errorf("f(SQL) show tags expects at least 1 argument, got %d", len(args))
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowTags(runtime.Context(), conn, "MACHBASEDB", runtime.ConsoleUser(), args[0], args[1:]...))
		}
	case "indexgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowIndexGap(runtime.Context(), conn))
		}
	case "tagindexgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowTagIndexGap(runtime.Context(), conn))
		}
	case "rollupgap":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowRollupGap(runtime.Context(), conn))
		}
	case "sessions":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowSessions(runtime.Context(), conn))
		}
	case "statements":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowStatements(runtime.Context(), conn))
		}
	case "storage":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowStorage(runtime.Context(), conn))
		}
	case "table-usage":
		err = validateNoAll()
		if err == nil {
			err = validateArgs(command, 0)
		}
		if err == nil {
			return yieldResultSet(node, spi.ShowTableUsage(runtime.Context(), conn))
		}
	default:
		err = fmt.Errorf("f(SQL) unsupported show command %q", command)
	}

	node.emit(ErrorRecord(err))
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
	runtime := node.ensureRuntime()
	runtime.SetResultColumns(append(client.Columns{client.MakeColumnRownum()}, nfo.Columns()...))
	if err := nfo.Err(); err != nil {
		node.emit(ErrorRecord(err))
		return err.Error()
	}
	nrow := int64(0)
	nfo.Iter(func(values []any) bool {
		nrow++
		if err := nfo.Err(); err != nil {
			node.emit(ErrorRecord(err))
			return false
		}
		if runtime.ShouldStop() {
			return false
		}
		node.emit(NewRecord(nrow, values))
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

type Table struct {
	Name string
}

func (x *Node) fmTable(tableName string) *Table {
	return &Table{Name: tableName}
}

type Tag struct {
	Name   string
	Column string
}

func (x *Node) fmTag(name string, column ...string) *Tag {
	if len(column) == 0 {
		return &Tag{Name: name, Column: "name"}
	} else {
		return &Tag{Name: name, Column: column[0]}
	}
}

func (x *Node) fmInsert(args ...any) (*insert, error) {
	ret := &insert{}
	for _, arg := range args {
		switch v := arg.(type) {
		case *bridgeName:
			ret.bridge = v
		case string:
			ret.columns = append(ret.columns, v)
		case *Table:
			ret.table = v
		case *Tag:
			ret.tag = v
		}
	}
	if ret.table == nil {
		return nil, ErrArgs("INSERT", 0, "table is not specified")
	}
	if ret.bridge == nil && ret.tag != nil {
		ret.columns = append([]string{ret.tag.Column}, ret.columns...)
	}
	ret.node = x
	return ret, nil
}

type insert struct {
	conn      *sql.Conn
	ctx       context.Context
	ctxCancel context.CancelFunc

	rowsAffected int64
	lastInsertId int64

	node    *Node
	bridge  *bridgeName
	columns []string

	table *Table
	tag   *Tag
}

func (ins *insert) Open(runtime *executionRuntime) error {
	ins.ctx, ins.ctxCancel = context.WithCancel(runtime.Context())
	if conn, err := spi.Connect(ins.ctx, runtime.ConsoleUser()); err != nil {
		return err
	} else {
		ins.conn = conn
	}
	return nil
}

func (ins *insert) Close() (string, error) {
	ins.conn.Close()
	ins.ctxCancel()

	unit := "rows"
	if ins.rowsAffected <= 1 {
		unit = "row"
	}
	return fmt.Sprintf("%d %s inserted.", ins.rowsAffected, unit), nil
}

func (ins *insert) AddRow(values []any) error {
	if ins.bridge != nil {
		return ins._addRowBridge(values)
	} else {
		return ins._addRow(values)
	}

}
func (ins *insert) _addRowBridge(values []any) error {
	br, err := bridge.GetSqlBridge(ins.bridge.name)
	if err != nil {
		return err
	}

	placeHolders := []string{}
	for idx := range ins.columns {
		placeHolders = append(placeHolders, br.ParameterMarker(idx))
	}
	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)",
		ins.table.Name,
		strings.Join(ins.columns, ","),
		strings.Join(placeHolders, ","))
	conn, err := br.Connect(ins.node.runtime.Context())
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ins.node.runtime.Context(), sqlText, values...)
	if err != nil {
		return fmt.Errorf("%s, %s", err, sqlText)
	}
	if br.SupportLastInsertId() {
		lastInsertId, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("%s, %s", err, sqlText)
		}
		ins.lastInsertId = lastInsertId
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s, %s", err, sqlText)
	}
	ins.rowsAffected = rowsAffected
	return nil
}

func (ins *insert) _addRow(values []any) error {
	placeHolders := []string{}
	for range ins.columns {
		placeHolders = append(placeHolders, "?")
	}
	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)",
		ins.table.Name,
		strings.Join(ins.columns, ","),
		strings.Join(placeHolders, ","))
	if ins.tag == nil {
		if _, err := ins.conn.ExecContext(ins.ctx, sqlText, values...); err != nil {
			return err
		}
	} else {
		if _, err := ins.conn.ExecContext(ins.ctx, sqlText, append([]any{ins.tag.Name}, values...)...); err != nil {
			return err
		}
	}
	ins.rowsAffected++
	return nil
}

func (x *Node) fmAppend(args ...any) (*appender, error) {
	ret := &appender{}
	for i, arg := range args {
		switch v := arg.(type) {
		case *Table:
			ret.table = v
		case *bridgeName:
			return nil, ErrArgs("APPEND", i, "cannot use with a bridge")
		}
	}
	if ret.table == nil {
		return nil, ErrArgs("APPEND", 0, "table is not specified")
	}
	return ret, nil
}

type appender struct {
	nrows      int
	dbAppender *spi.AppendWorker
	dbColumns  client.Columns
	table      *Table
}

func (app *appender) Open(runtime *executionRuntime) (err error) {
	aw, err := spi.GetAppendWorker(runtime.Context(), app.table.Name)
	if err != nil {
		return
	}
	app.dbAppender = aw
	return
}

func (app *appender) Close() (string, error) {
	var succ, fail int64
	var err error
	if app.dbAppender != nil {
		succ, fail, err = app.dbAppender.Close()
	}
	_ = succ
	if err != nil {
		return fmt.Sprintf("append fail, %s", err.Error()), err
	} else {
		unit := "rows"
		if app.nrows <= 1 {
			unit = "row"
		}
		// since we are using api.AppendWraper, success is always nrows
		return fmt.Sprintf("append %d %s (success %d, fail %d)", app.nrows, unit, app.nrows, fail), nil
	}
}

func (app *appender) AddRow(values []any) error {
	if app.dbAppender == nil {
		return errors.New("f(APPEND) no appender exists")
	}
	if app.dbColumns == nil {
		app.dbColumns = app.dbAppender.Columns()
	}

	var timeformat string = "ns"
	var timeLocation *time.Location
	for idx, col := range app.dbColumns {
		if idx >= len(values) {
			return fmt.Errorf("missing value for column %s", col.Name)
		}
		if values[idx] == nil {
			continue
		}
		val, err := col.DataType.Apply(values[idx], timeformat, timeLocation)
		if err != nil {
			return fmt.Errorf("invalid value for column %s: %v, error: %s", col.Name, values[idx], err.Error())
		} else {
			values[idx] = val
		}
	}

	err := app.dbAppender.Append(values...)
	if err == nil {
		app.nrows++
	}
	return err
}

func (x *Node) fmSqlSink(args ...any) (*sqlSink, error) {
	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}

	ret := &sqlSink{}
	var paramStart int

	switch v := args[0].(type) {
	case string:
		ret.sqlText = strings.TrimSuffix(strings.TrimSpace(v), ";")
		paramStart = 1
	case *bridgeName:
		ret.bridge = v.name
		if len(args) < 2 {
			return nil, ErrInvalidNumOfArgs("SQL", 2, len(args))
		}
		sqlText, ok := args[1].(string)
		if !ok {
			return nil, ErrWrongTypeOfArgs("SQL", 1, "sql text", args[1])
		}
		ret.sqlText = strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
		paramStart = 2
	default:
		return nil, ErrWrongTypeOfArgs("SQL", 0, "sql text or bridge('name')", args[0])
	}

	if len(ret.sqlText) == 0 {
		return nil, fmt.Errorf("f(SQL) Empty SQL text")
	}
	ret.stmtType = spi.DetectSQLStatementType(ret.sqlText)
	if err := validateSqlVerbForSink(ret.sqlText); err != nil {
		return nil, err
	}

	ret.rawParams = make([]any, 0, len(args)-paramStart)
	for i := paramStart; i < len(args); i++ {
		ret.rawParams = append(ret.rawParams, args[i])
	}
	return ret, nil
}

type sqlSink struct {
	sqlText   string
	stmtType  spi.SQLStatementType
	rawParams []any
	bridge    string

	ctx       context.Context
	ctxCancel context.CancelFunc
	conn      *sql.Conn

	affectedRows int64
	resultMsg    string
}

func (s *sqlSink) Open(runtime *executionRuntime) error {
	s.ctx, s.ctxCancel = context.WithCancel(runtime.Context())
	if s.bridge == "" {
		conn, err := spi.Connect(s.ctx, runtime.ConsoleUser())
		if err != nil {
			s.ctxCancel()
			return err
		}
		s.conn = conn
		return nil
	}
	db, err := connector.Database(s.bridge)
	if err != nil {
		s.ctxCancel()
		return err
	}
	conn, err := db.Conn(s.ctx)
	if err != nil {
		s.ctxCancel()
		return err
	}
	s.conn = conn
	return nil
}

func (s *sqlSink) Close() (string, error) {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	return spi.MakeUserMessage(s.stmtType, s.affectedRows), nil
}

func (s *sqlSink) AddRow(values []any) error {
	params := make([]any, 0, len(s.rawParams))
	for _, p := range s.rawParams {
		switch v := p.(type) {
		case *recordValueRef:
			if v == nil {
				params = append(params, nil)
				continue
			}
			if v.index < 0 || v.index >= len(values) {
				return fmt.Errorf("f(SQL) value(%d) is out of range of input tuple(len:%d)", v.index, len(values))
			}
			params = append(params, values[v.index])
		default:
			params = append(params, p)
		}
	}
	result, err := s.conn.ExecContext(s.ctx, s.sqlText, params...)
	if err != nil {
		return err
	}
	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	s.resultMsg = spi.MakeUserMessage(s.stmtType, affectedRows)
	if n, ok := parseRowsAffectedFromMessage(s.resultMsg); ok {
		s.affectedRows += n
	} else {
		s.affectedRows++
	}
	return nil
}

func validateSqlVerbForSink(sqlText string) error {
	stmtType := spi.DetectSQLStatementType(sqlText)
	if stmtType.IsFetch() {
		verb := strings.ToUpper(strings.Fields(sqlText)[0])
		return fmt.Errorf("f(SQL) sink does not allow fetch verb %q", verb)
	}
	return nil
}

func parseRowsAffectedFromMessage(msg string) (int64, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(msg))
	if trimmed == "" {
		return 0, false
	}
	if strings.HasPrefix(trimmed, "a row ") {
		return 1, true
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 3 {
		return 0, false
	}
	if fields[1] != "row" && fields[1] != "rows" {
		return 0, false
	}
	var n int64
	if _, err := fmt.Sscan(fields[0], &n); err != nil {
		return 0, false
	}
	return n, true
}
