package tql

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector"
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
	conn, err := dc.task.ConnDatabase(node.task.ctx)
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer conn.Close()

	query := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			cols = append([]*api.Column{api.MakeColumnRownum()}, cols...)
			dc.task.SetResultColumns(cols)
		},
		Next: func(q *api.Query, nrow int64) bool {
			if dc.task.shouldStop() {
				return false
			}
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				ErrorRecord(err).Tell(node.next)
				return false
			}
			if err = q.Scan(values...); err != nil {
				ErrorRecord(err).Tell(node.next)
				return false
			}
			if len(values) > 0 {
				NewRecord(nrow, values).Tell(node.next)
			}
			return !dc.task.shouldStop()
		},
		End: func(q *api.Query) {
			dc.resultMsg = q.UserMessage()
			if !q.IsFetch() {
				dc.task.SetResultColumns(api.Columns{
					api.MakeColumnRownum(),
					api.MakeColumnString("MESSAGE"),
				})
				NewRecord(1, q.UserMessage()).Tell(node.next)
			}
		},
	}
	if err := query.Execute(node.task.ctx, conn, dc.sqlText, dc.params...); err != nil {
		dc.resultMsg = err.Error()
		ErrorRecord(err).Tell(node.next)
	}
}

// SQL('select ....', arg1, arg2)
// SQL(bridge('sqlite'), 'SELECT * ...', arg1, arg2)
func (x *Node) fmSql(args ...any) (any, error) {
	if len(args) == 0 {
		return nil, ErrInvalidNumOfArgs("SQL", 1, 0)
	}
	tick := time.Now()
	var databaseProvider func(ctx context.Context) (api.Conn, error)
	var sqlText string
	var sqlParams []any
	var sqlBridged bool
	var prompt string
	var resultMsg string

	switch v := args[0].(type) {
	case string:
		databaseProvider = func(ctx context.Context) (api.Conn, error) {
			return x.task.ConnDatabase(ctx)
		}
		sqlText = strings.TrimSuffix(strings.TrimSpace(v), ";")
		sqlParams = args[1:]
	case *bridgeName:
		sqlBridged = true
		databaseProvider = func(ctx context.Context) (api.Conn, error) {
			db, err := connector.New(v.name)
			if err != nil {
				return nil, err
			}
			return db.Connect(ctx)
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

	if sqlBridged && !api.DetectSQLStatementType(sqlText).IsFetch() {
		conn, err := databaseProvider(x.task.ctx)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		result := conn.Exec(x.task.ctx, sqlText, sqlParams...)
		if err := result.Err(); err != nil {
			resultMsg = err.Error()
			ErrorRecord(err).Tell(x.next)
			return nil, nil
		}
		resultMsg = result.Message()
		x.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			api.MakeColumnString("MESSAGE"),
		})
		NewRecord(1, resultMsg).Tell(x.next)
	} else {
		ch := api.CommandHandler{
			Database:        databaseProvider,
			FallbackVerb:    "sql --",
			SilenceUsage:    true,
			SilenceErrors:   true,
			ShowTables:      func(ti *api.TableInfo, nrow int64) bool { return yieldTableInfo(x, ti, nrow) },
			ShowIndexes:     func(ii *api.IndexInfo, nrow int64) bool { return yieldIndexesInfo(x, ii, nrow) },
			ShowIndex:       func(ii *api.IndexInfo) bool { return yieldIndexInfo(x, ii) },
			ShowLsmIndexes:  func(li *api.LsmIndexInfo, nrow int64) bool { return yieldLsmIndexesInfo(x, li, nrow) },
			DescribeTable:   func(td *api.TableDescription) { yieldTableDescription(x, td) },
			ShowTags:        func(tag *api.TagInfo, nrow int64) bool { return yieldTags(x, tag, nrow) },
			ShowIndexGap:    func(gap *api.IndexGapInfo, nrow int64) bool { return yieldIndexGap(x, gap, nrow) },
			ShowTagIndexGap: func(gap *api.IndexGapInfo, nrow int64) bool { return yieldTagIndexGap(x, gap, nrow) },
			ShowRollupGap:   func(gap *api.RollupGapInfo, nrow int64) bool { return yieldRollupGap(x, gap, nrow) },
			ShowSessions:    func(info *api.SessionInfo, nrow int64) bool { return yieldSessionsInfo(x, info, nrow) },
			ShowStatements:  func(info *api.StatementInfo, nrow int64) bool { return yieldStatementsInfo(x, info, nrow) },
			ShowStorage:     func(info *api.StorageInfo, nrow int64) bool { return yieldStorageInfo(x, info, nrow) },
			ShowTableUsage:  func(info *api.TableUsageInfo, nrow int64) bool { return yieldTableUsageInfo(x, info, nrow) },
			ShowLicense:     func(info *api.LicenseInfo) bool { return yieldLicenseInfo(x, info) },
			Explain:         func(sql string, err error) { yieldExplain(x, sql, err) },
			SqlQuery: func(q *api.Query, nrow int64) bool {
				if nrow == -1 {
					resultMsg = q.UserMessage()
				}
				return yieldSqlQuery(x, q, nrow)
			},
		}
		ch.PreExecute = func(args []string) {
			x.task.LogInfo("╭─", prompt, sqlText)
		}
		ch.PostExecute = func(args []string, message string, err error) {
			if err != nil {
				x.task.LogError("╰─➤", err.Error())
			} else {
				x.task.LogInfo("╰─➤", resultMsg, time.Since(tick).String())
			}
		}
		args := api.ParseCommandLine(sqlText)
		if !ch.IsKnownVerb(args[0]) {
			args = append([]string{"sql", "--"}, sqlText)
		}
		if err := ch.Exec(x.task.ctx, args, sqlParams...); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func withRownum(p interface{ Columns() api.Columns }) api.Columns {
	return append(api.Columns{api.MakeColumnRownum()}, p.Columns()...)
}

func yieldTableInfo(node *Node, info *api.TableInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(withRownum(info))
	}
	NewRecord(nrow, info.Values()).Tell(node.next)
	return true
}

func yieldIndexesInfo(node *Node, info *api.IndexInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(withRownum(info))
	}
	NewRecord(nrow, info.Values()).Tell(node.next)
	return info.Err() == nil
}

func yieldIndexInfo(node *Node, info *api.IndexInfo) bool {
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnRownum(),
		api.MakeColumnString("TABLE_NAME"),
		api.MakeColumnString("COLUMN_NAME"),
		api.MakeColumnString("INDEX_NAME"),
		api.MakeColumnString("INDEX_TYPE"),
		api.MakeColumnString("KEY_COMPRESS"),
		api.MakeColumnInt64("MAX_LEVEL"),
		api.MakeColumnInt64("PART_VALUE_COUNT"),
		api.MakeColumnString("BITMAP_ENCODE"),
	})
	NewRecord(1, []any{
		info.TableName,
		info.ColumnName,
		info.IndexName,
		info.IndexType,
		info.KeyCompress,
		info.MaxLevel,
		info.PartValueCount,
		info.BitMapEncode,
	}).Tell(node.next)
	return info.Err() == nil
}

func yieldLsmIndexesInfo(node *Node, info *api.LsmIndexInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "TABLE_NAME", DataType: api.DataTypeString},
			{Name: "INDEX_NAME", DataType: api.DataTypeString},
			{Name: "LEVEL", DataType: api.DataTypeInt64},
			{Name: "COUNT", DataType: api.DataTypeInt64},
		})
	}
	NewRecord(nrow, []any{
		info.TableName,
		info.IndexName,
		info.Level,
		info.Count,
	}).Tell(node.next)
	return true
}

func yieldStorageInfo(node *Node, info *api.StorageInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "TABLE_NAME", DataType: api.DataTypeString},
			{Name: "DATA_SIZE", DataType: api.DataTypeInt64},
			{Name: "INDEX_SIZE", DataType: api.DataTypeInt64},
			{Name: "TOTAL_SIZE", DataType: api.DataTypeInt64},
		})
	}
	NewRecord(nrow, []any{
		info.TableName,
		info.DataSize,
		info.IndexSize,
		info.TotalSize,
	}).Tell(node.next)
	return true
}

func yieldTableUsageInfo(node *Node, info *api.TableUsageInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "TABLE_NAME", DataType: api.DataTypeString},
			{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
		})
	}
	NewRecord(nrow, []any{
		info.TableName,
		info.StorageUsage,
	}).Tell(node.next)
	return true
}

func yieldTableDescription(node *Node, desc *api.TableDescription) {
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnRownum(),
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "LENGTH", DataType: api.DataTypeInt32},
		{Name: "FLAG", DataType: api.DataTypeString},
		{Name: "INDEX", DataType: api.DataTypeString},
	})
	for i, col := range desc.Columns {
		indexes := []string{}
		for _, idxDesc := range desc.Indexes {
			for _, colName := range idxDesc.Cols {
				if colName == col.Name {
					indexes = append(indexes, idxDesc.Name)
					break
				}
			}
		}
		values := []any{
			col.Name, col.Type.String(), col.Width(), col.Flag.String(), strings.Join(indexes, ","),
		}
		NewRecord(i+1, values).Tell(node.next)
	}
}

func yieldTags(node *Node, tag *api.TagInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "_ID", DataType: api.DataTypeInt64},
			{Name: "NAME", DataType: api.DataTypeString},
			{Name: "ROW_COUNT", DataType: api.DataTypeInt64},
			{Name: "MIN_TIME", DataType: api.DataTypeDatetime},
			{Name: "MAX_TIME", DataType: api.DataTypeDatetime},
			{Name: "RECENT_ROW_TIME", DataType: api.DataTypeDatetime},
			{Name: "MIN_VALUE", DataType: api.DataTypeFloat64},
			{Name: "MIN_VALUE_TIME", DataType: api.DataTypeDatetime},
			{Name: "MAX_VALUE", DataType: api.DataTypeFloat64},
			{Name: "MAX_VALUE_TIME", DataType: api.DataTypeDatetime},
		})
	}
	if tag.Stat != nil {
		if tag.Summarized {
			NewRecord(nrow, []any{tag.Id, tag.Name, tag.Stat.RowCount,
				tag.Stat.MinTime, tag.Stat.MaxTime, tag.Stat.RecentRowTime,
				tag.Stat.MinValue, tag.Stat.MinValueTime,
				tag.Stat.MaxValue, tag.Stat.MaxValueTime}).Tell(node.next)
		} else {
			NewRecord(nrow, []any{tag.Id, tag.Name, tag.Stat.RowCount,
				tag.Stat.MinTime, tag.Stat.MaxTime, tag.Stat.RecentRowTime,
				nil, nil, nil, nil}).Tell(node.next)
		}
	} else {
		NewRecord(nrow, []any{tag.Id, tag.Name, nil,
			nil, nil, nil,
			nil, nil, nil, nil}).Tell(node.next)
	}
	return true
}

func yieldRollupGap(node *Node, gap *api.RollupGapInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "SRC_TABLE", DataType: api.DataTypeString},
			{Name: "ROLLUP_TABLE", DataType: api.DataTypeString},
			{Name: "SRC_END_RID", DataType: api.DataTypeInt64},
			{Name: "ROLLUP_END_RID", DataType: api.DataTypeInt64},
			{Name: "GAP", DataType: api.DataTypeInt64},
			{Name: "LAST_ELAPSED", DataType: api.DataTypeString},
		})
	}

	NewRecord(nrow, []any{
		gap.SrcTable, gap.RollupTable, gap.SrcEndRID, gap.RollupEndRID, gap.Gap, gap.LastElapsed.String(),
	}).Tell(node.next)
	return true
}

func yieldIndexGap(node *Node, gap *api.IndexGapInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(withRownum(gap))
	}
	NewRecord(nrow, gap.Values()).Tell(node.next)
	return true
}

func yieldTagIndexGap(node *Node, gap *api.IndexGapInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(withRownum(gap))
	}
	NewRecord(nrow, gap.Values()).Tell(node.next)
	return true
}

func yieldSessionsInfo(node *Node, info *api.SessionInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(withRownum(info))
	}
	NewRecord(nrow, info.Values()).Tell(node.next)
	return true
}

func yieldStatementsInfo(node *Node, info *api.StatementInfo, nrow int64) bool {
	if nrow == 1 {
		node.task.SetResultColumns(api.Columns{
			api.MakeColumnRownum(),
			{Name: "ID", DataType: api.DataTypeInt64},
			{Name: "SESSION_ID", DataType: api.DataTypeInt64},
			{Name: "STATE", DataType: api.DataTypeString},
			{Name: "TYPE", DataType: api.DataTypeString},
			{Name: "RECORD_SIZE", DataType: api.DataTypeInt64},
			{Name: "APPEND_SUCCESS", DataType: api.DataTypeInt64},
			{Name: "APPEND_FAIL", DataType: api.DataTypeInt64},
			{Name: "QUERY", DataType: api.DataTypeString},
		})
	}
	if info.IsNeo {
		NewRecord(nrow, []any{
			info.ID, info.SessionID, info.State, "neo", nil, info.AppendSuccessCount, info.AppendFailCount, info.Query,
		}).Tell(node.next)
	} else {
		NewRecord(nrow, []any{
			info.ID, info.SessionID, info.State, "", info.RecordSize, nil, nil, info.Query,
		}).Tell(node.next)
	}
	return info.Err() == nil
}

func yieldLicenseInfo(node *Node, info *api.LicenseInfo) bool {
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnRownum(),
		api.MakeColumnString("ID"),
		api.MakeColumnString("TYPE"),
		api.MakeColumnString("CUSTOMER"),
		api.MakeColumnString("PROJECT"),
		api.MakeColumnString("COUNTRY_CODE"),
		api.MakeColumnString("INSTALL_DATE"),
		api.MakeColumnString("ISSUE_DATE"),
		api.MakeColumnString("STATUS"),
	})
	NewRecord(1, []any{
		info.Id, info.Type, info.Customer, info.Project, info.CountryCode, info.InstallDate, info.IssueDate, info.LicenseStatus,
	}).Tell(node.next)
	return true
}

func yieldExplain(node *Node, plan string, err error) {
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	node.task.SetResultColumns(api.Columns{
		api.MakeColumnString("PLAN"),
	})
	for _, line := range strings.Split(plan, "\n") {
		NewRecord(1, []any{line}).Tell(node.next)
	}
}

func yieldSqlQuery(node *Node, q *api.Query, nrow int64) bool {
	if node.task.shouldStop() {
		return false
	}
	if nrow == 0 { // Query.Begin
		cols := q.Columns()
		cols = append([]*api.Column{api.MakeColumnRownum()}, cols...)
		node.task.SetResultColumns(cols)
		return true
	} else if nrow == -1 { // Query.End
		if !q.IsFetch() {
			node.task.SetResultColumns(api.Columns{
				api.MakeColumnRownum(),
				api.MakeColumnString("MESSAGE"),
			})
			NewRecord(1, q.UserMessage()).Tell(node.next)
		}
		return false
	}
	// Query.Next
	values, err := q.Columns().MakeBuffer()
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return false
	}
	if err = q.Scan(values...); err != nil {
		ErrorRecord(err).Tell(node.next)
		return false
	}
	if len(values) > 0 {
		NewRecord(nrow, values).Tell(node.next)
	}
	return !node.task.shouldStop()
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
