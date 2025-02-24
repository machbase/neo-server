package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "show",
		PcFunc: pcShow,
		Action: doShow,
		Desc:   "Display information",
		Usage:  strings.ReplaceAll(helpShow, "\t", "    "),
	})
}

const helpShow = `  show [options] <object>
  objects:
    info                   show server info
    license                show license info
    ports                  show service ports
    users                  list users
    tables [-a]            list tables
    table [-a] <table>     describe the table
    meta-tables            list meta tables
    virtual-tables         list virtual tables
    sessions               list sessions
    statements             list statements
    indexes                list indexes
    index <index>          describe the index
    storage                show storage info
    table-usage            show table usage
    lsm                    LSM status
    indexgap               index gap status
    rollupgap              rollup gap status
    tagindexgap            tag index gap status
    tags <table>           tag list of the table
    tagstat <table> <tag>  show stat of the tag
  options:
    -a,--all               includes all hidden tables/columns`

type ShowCmd struct {
	Object  string   `arg:""`
	Args    []string `arg:"" optional:""`
	ShowAll bool     `name:"all" short:"a"`
	Help    bool     `kong:"-"`
}

func pcShow() action.PrefixCompleterInterface {
	return action.PcItem("show",
		action.PcItem("info"),
		action.PcItem("license"),
		action.PcItem("ports"),
		action.PcItem("users"),
		action.PcItem("tables"),
		action.PcItem("table"),
		action.PcItem("meta-tables"),
		action.PcItem("virtual-tables"),
		action.PcItem("statements"),
		action.PcItem("sessions"),
		action.PcItem("indexes"),
		action.PcItem("index"),
		action.PcItem("storage"),
		action.PcItem("table-usage"),
		action.PcItem("lsm"),
		action.PcItem("indexgap"),
		action.PcItem("rollupgap"),
		action.PcItem("tagindexgap"),
		action.PcItem("tags"),
		action.PcItem("tagstat"),
	)
}

func doShow(ctx *action.ActionContext) {
	cmd := &ShowCmd{}

	parser, err := action.Kong(cmd, func() error { ctx.Println(helpShow); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	switch strings.ToLower(cmd.Object) {
	case "info":
		doShowInfo(ctx)
	case "ports":
		doShowPorts(ctx)
	case "users":
		doShowUsers(ctx)
	case "tables":
		doShowTables(ctx, cmd.ShowAll)
	case "table":
		doShowTable(ctx, cmd.Args, cmd.ShowAll)
	case "meta-tables":
		doShowMVTables(ctx, "M$TABLES")
	case "virtual-tables":
		doShowMVTables(ctx, "V$TABLES")
	case "indexes":
		doShowIndexes(ctx)
	case "index":
		doShowIndex(ctx, cmd.Args)
	case "license":
		doShowLicense(ctx)
	case "sessions":
		doShowSessions(ctx)
	case "statements":
		doShowStatements(ctx)
	case "storage":
		doShowStorage(ctx)
	case "table-usage":
		doShowTableUsage(ctx)
	case "lsm":
		doShowLsm(ctx)
	case "indexgap":
		doShowIndexGap(ctx)
	case "rollupgap":
		doShowRollupGap(ctx)
	case "tagindexgap":
		doShowTagIndexGap(ctx)
	case "tags":
		doShowTags(ctx, cmd.Args)
	case "tagstat":
		doShowTagStat(ctx, cmd.Args)
	default:
		ctx.Println(helpShow)
		return
	}
}

func doShowIndexGap(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.IndexGapInfo{}).Columns().Names()...))
	nrow := 0
	api.ListIndexGapWalk(ctx.Ctx, conn, func(igi *api.IndexGapInfo) bool {
		if igi.Err() != nil {
			ctx.Println("ERR", igi.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, igi.Values()...)...)
		return true
	})
	box.Render()
}

func doShowTagIndexGap(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.IndexGapInfo{}).Columns().Names()...))
	nrow := 0
	api.ListTagIndexGapWalk(ctx.Ctx, conn, func(igi *api.IndexGapInfo) bool {
		if igi.Err() != nil {
			ctx.Println("ERR", igi.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, igi.Values()...)...)
		return true
	})
	box.Render()
}

func doShowLsm(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.LsmIndexInfo{}).Columns().Names()...))
	api.ListLsmIndexesWalk(ctx.Ctx, conn, func(nfo *api.LsmIndexInfo) bool {
		if nfo.Err() != nil {
			ctx.Println("ERR", nfo.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, nfo.Values()...)...)
		return true
	})
	box.Render()
}

func doShowUsers(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	list, err := api.GetUsers(ctx.Ctx, conn)
	if err != nil {
		ctx.Println("unable to find users; ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "USER_NAME"})
	for _, name := range list {
		nrow++
		box.AppendRow(nrow, name)
	}
	box.Render()
}

func doShowIndexes(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	list, err := api.ListIndexes(ctx.Ctx, conn)
	if err != nil {
		ctx.Println("unable to find indexes; ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "USER_NAME", "DB", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE"})
	for _, nfo := range list {
		nrow++
		box.AppendRow(nrow, nfo.User, nfo.Database, nfo.TableName, nfo.ColumnName, nfo.IndexName, nfo.IndexType)
	}
	box.Render()
}

func doShowIndex(ctx *action.ActionContext, args []string) {
	if len(args) != 1 {
		ctx.Println("missing index name")
		ctx.Println("Usage: show index <index>")
		return
	}
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nfo, err := api.DescribeIndex(ctx.Ctx, conn, args[0])
	if err != nil {
		ctx.Println("unable to describe index", args[0], "; ERR", err.Error())
		return
	}
	box := ctx.NewBox([]string{"ROWNUM", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"})
	box.AppendRow(1, nfo.TableName, nfo.ColumnName, nfo.IndexName, nfo.IndexType, nfo.KeyCompress, nfo.MaxLevel, nfo.PartValueCount, nfo.BitMapEncode)
	box.Render()
}

func doShowLicense(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nfo, err := api.GetLicenseInfo(ctx.Ctx, conn)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	box := ctx.NewBox([]string{"ID", "TYPE", "CUSTOMER", "PROJECT", "COUNTRY_CODE", "INSTALL_DATE", " ISSUE_DATE", "STATUS"})
	box.AppendRow(nfo.Id, nfo.Type, nfo.Customer, nfo.Project, nfo.CountryCode, nfo.InstallDate, nfo.IssueDate, nfo.LicenseStatus)
	box.Render()
}

func doShowSessions(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.SessionInfo{}).Columns().Names()...))
	nrow := 0
	api.ListSessionsWalk(ctx.Ctx, conn, func(si *api.SessionInfo) bool {
		if si.Err() != nil {
			ctx.Println("ERR", si.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, si.Values()...)...)
		return true
	})
	box.Render()
}

func doShowStatements(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	box := ctx.NewBox([]string{"ID", "SESSION_ID", "STATE", "TYPE", "RECORD_SIZE", "APPEND_SUCCESS", "APPEND_FAIL", "QUERY"})
	api.ListStatementsWalk(ctx.Ctx, conn, func(stmt *api.StatementInfo) bool {
		if stmt.Err() != nil {
			ctx.Println("ERR", stmt.Err().Error())
			return false
		}
		if stmt.IsNeo {
			box.AppendRow(stmt.ID, stmt.SessionID, stmt.State, "neo", "-", stmt.AppendSuccessCount, stmt.AppendFailCount, stmt.Query)
		} else {
			box.AppendRow(stmt.ID, stmt.SessionID, stmt.State, "", stmt.RecordSize, "-", "-", stmt.Query)
		}
		return true
	})
	box.Render()
}

func doShowStorage(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.StorageInfo{}).Columns().Names()...))
	api.ListStorageWalk(ctx.Ctx, conn, func(nfo *api.StorageInfo) bool {
		if nfo.Err() != nil {
			ctx.Println("unable to find storage; ERR", nfo.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, nfo.Values()...)...)
		return true
	})
	box.Render()
}

func doShowTableUsage(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.TableUsageInfo{}).Columns().Names()...))
	api.ListTableUsageWalk(ctx.Ctx, conn, func(nfo *api.TableUsageInfo) bool {
		if nfo.Err() != nil {
			ctx.Println("unable to find table usage; ERR", nfo.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(append([]any{nrow}, nfo.Values()...)...)
		return true
	})
	box.Render()
}

func doShowRollupGap(ctx *action.ActionContext) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox(append([]string{"ROWNUM"}, (&api.RollupGapInfo{}).Columns().Names()...))
	api.ListRollupGapWalk(ctx.Ctx, conn, func(nfo *api.RollupGapInfo) bool {
		if nfo.Err() != nil {
			ctx.Println("unable to find rollupgap; ERR", nfo.Err().Error())
			return false
		}
		nrow++
		box.AppendRow(nrow, nfo.SrcTable, nfo.RollupTable, nfo.SrcEndRID, nfo.RollupEndRID, nfo.Gap, nfo.LastElapsed.String())
		return true
	})
	box.Render()
}

func doShowTags(ctx *action.ActionContext, args []string) {
	if len(args) != 1 {
		ctx.Println("missing table name")
		ctx.Println("Usage: show tags <table>")
		return
	}

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	tableName := args[0]
	tableType, err := api.QueryTableType(ctx.Ctx, conn, tableName)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if tableType != api.TableTypeTag {
		ctx.Println("ERR", fmt.Sprintf("'%s' is not a tag table", tableName))
		return
	}

	desc, err := api.DescribeTable(ctx.Ctx, conn, tableName, false)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	summarized := false
	for _, c := range desc.Columns {
		if c.Flag&api.ColumnFlagSummarized > 0 {
			summarized = true
			break
		}
	}
	var t action.Box
	if summarized {
		t = ctx.NewBox([]string{"ROWNUM", "_ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"})
	} else {
		t = ctx.NewBox([]string{"ROWNUM", "_ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME"})
	}
	nrow := 0
	api.ListTagsWalk(ctx.Ctx, conn, strings.ToUpper(args[0]), func(tag *api.TagInfo) bool {
		if tag.Err != nil {
			ctx.Println("ERR", err.Error())
			return false
		}
		nrow++
		if summarized {
			stat, err := api.TagStat(ctx.Ctx, conn, tableName, tag.Name)
			if err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			t.AppendRow(nrow, tag.Id, tag.Name, stat.RowCount,
				stat.MinTime, stat.MaxTime, stat.RecentRowTime,
				stat.MinValue, stat.MinValueTime,
				stat.MaxValue, stat.MaxValueTime)
		} else {
			stat, err := api.TagStat(ctx.Ctx, conn, tableName, tag.Name)
			if err != nil {
				// tag exists in _table_meta, but not found in v$table_stat
				t.AppendRow(nrow, tag.Id, tag.Name, "", "", "", "")
			} else {
				t.AppendRow(nrow, tag.Id, tag.Name, stat.RowCount, stat.MinTime, stat.MaxTime, stat.RecentRowTime)
			}
		}
		return true
	})
	t.Render()
}

func doShowTagStat(ctx *action.ActionContext, args []string) {
	if len(args) != 2 {
		ctx.Println("missing table or tag name")
		ctx.Println("Usage: show tagstat <table> <tag>")
		return
	}

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	t := ctx.NewBox([]string{"NAME", "VALUE"})
	stat, err := api.TagStat(ctx.Ctx, conn, args[0], args[1])
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	tz := time.UTC
	if itm := ctx.Pref().TimeZone(); itm != nil {
		tz = itm.TimezoneValue()
	}
	timeformat := util.GetTimeformat("-")
	if itm := ctx.Pref().Timeformat(); itm != nil {
		timeformat = itm.Value()
	}
	tmf := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return fmt.Sprintf("%s (%s)", t.In(tz).Format(timeformat), tz.String())
	}

	t.AppendRow("NAME", stat.Name)
	t.AppendRow("ROW_COUNT", stat.RowCount)
	t.AppendRow("MIN_TIME", tmf(stat.MinTime))
	t.AppendRow("MAX_TIME", tmf(stat.MaxTime))
	t.AppendRow("MIN_VALUE", stat.MinValue)
	t.AppendRow("MIN_VALUE_TIME", tmf(stat.MinValueTime))
	t.AppendRow("MAX_VALUE", stat.MaxValue)
	t.AppendRow("MAX_VALUE_TIME", tmf(stat.MaxValueTime))
	t.AppendRow("RECENT_ROW_TIME", tmf(stat.RecentRowTime))
	t.Render()
}

func doShowByQuery0(ctx *action.ActionContext, sqlText string, showRownum bool) {
	var output io.Writer = &util.NopCloseWriter{Writer: os.Stdout}
	encoder := codec.NewEncoder(codec.BOX,
		opts.OutputStream(output),
		opts.Rownum(showRownum),
		opts.Heading(true),
		opts.BoxStyle(ctx.Pref().BoxStyle().Value()),
	)

	query := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		Next: func(q *api.Query, nrow int64) bool {
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			if err := q.Scan(values...); err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			if err := encoder.AddRow(values); err != nil {
				ctx.Println("ERR", err.Error())
			}
			return true
		},
		End: func(q *api.Query) {
			encoder.Close()
			ctx.Println(q.UserMessage())
		},
	}

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if err := query.Execute(ctx.Ctx, conn, sqlText); err != nil {
		ctx.Println("ERR", err.Error())
	}
}

func doShowTable(ctx *action.ActionContext, args []string, showAll bool) {
	if len(args) != 1 {
		ctx.Println("missing table name")
		ctx.Println("Usage: show table [-a] <table>")
		return
	}

	table := args[0]

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	desc, err := api.DescribeTable(ctx.Ctx, conn, table, showAll)
	if err != nil {
		ctx.Println("unable to describe", table, "; ERR", err.Error())
		return
	}

	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "LENGTH", "DESC"})
	for _, col := range desc.Columns {
		nrow++
		colType := col.Type.String()
		box.AppendRow(nrow, col.Name, colType, col.Width(), col.Flag.String())
	}

	box.Render()
}

func doShowTables(ctx *action.ActionContext, showAll bool) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	t := ctx.NewBox(append([]string{"ROWNUM"}, (&api.TableInfo{}).Columns().Names()...))
	nrow := 0
	api.ListTablesWalk(ctx.Ctx, conn, showAll, func(ti *api.TableInfo) bool {
		if ti.Err() != nil {
			ctx.Println("ERR", ti.Err())
			return false
		}
		if ctx.Actor.Username() != "sys" && ti.Database != "MACHBASEDB" {
			return true
		}
		nrow++
		t.AppendRow(append([]any{nrow}, ti.Values()...)...)
		return true
	})
	t.Render()
}

func doShowMVTables(ctx *action.ActionContext, tablesTable string) {
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	rows, err := conn.Query(ctx.Ctx, fmt.Sprintf("select NAME, TYPE, FLAG, ID from %s order by ID", tablesTable))
	if err != nil {
		ctx.Printfln("ERR select %s fail; %s", tablesTable, err.Error())
		return
	}
	defer rows.Close()

	t := ctx.NewBox([]string{"ROWNUM", "ID", "NAME", "TYPE"})

	nrow := 0
	for rows.Next() {
		var name string
		var typ api.TableType
		var flg api.TableFlag
		var id int
		err := rows.Scan(&name, &typ, &flg, &id)
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		nrow++

		desc := api.TableTypeDescription(api.TableType(typ), flg)
		t.AppendRow(nrow, id, name, desc)
	}
	t.Render()
}

func doShowInfo(ctx *action.ActionContext) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	nfo, err := mgmtClient.ServerInfo(ctx.Ctx, &mgmt.ServerInfoRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	uptime := time.Duration(nfo.Runtime.UptimeInSecond) * time.Second

	box := ctx.NewBox([]string{"NAME", "VALUE"})

	box.AppendRow("build.version", fmt.Sprintf("v%d.%d.%d", nfo.Version.Major, nfo.Version.Minor, nfo.Version.Patch))
	box.AppendRow("build.hash", fmt.Sprintf("#%s", nfo.Version.GitSHA))
	box.AppendRow("build.timestamp", nfo.Version.BuildTimestamp)
	box.AppendRow("build.engine", nfo.Version.Engine)

	box.AppendRow("runtime.os", nfo.Runtime.OS)
	box.AppendRow("runtime.arch", nfo.Runtime.Arch)
	box.AppendRow("runtime.pid", nfo.Runtime.Pid)
	box.AppendRow("runtime.uptime", util.HumanizeDurationWithFormat(uptime, util.HumanizeDurationFormatSimple))
	box.AppendRow("runtime.processors", nfo.Runtime.Processes)
	box.AppendRow("runtime.goroutines", nfo.Runtime.Goroutines)

	box.AppendRow("mem.mallocs", util.NumberFormat(nfo.Runtime.Mem["mallocs"]))
	box.AppendRow("mem.frees", util.NumberFormat(nfo.Runtime.Mem["frees"]))
	box.AppendRow("mem.lives", util.NumberFormat(nfo.Runtime.Mem["lives"]))
	box.AppendRow("mem.sys", util.BytesUnit(nfo.Runtime.Mem["sys"], ctx.Lang))
	box.AppendRow("mem.heap_sys", util.BytesUnit(nfo.Runtime.Mem["heap_sys"], ctx.Lang))
	box.AppendRow("mem.heap_alloc", util.BytesUnit(nfo.Runtime.Mem["heap_alloc"], ctx.Lang))
	box.AppendRow("mem.heap_in_use", util.BytesUnit(nfo.Runtime.Mem["heap_in_use"], ctx.Lang))
	box.AppendRow("mem.stack_sys", util.BytesUnit(nfo.Runtime.Mem["stack_sys"], ctx.Lang))
	box.AppendRow("mem.stack_in_use", util.BytesUnit(nfo.Runtime.Mem["stack_in_use"], ctx.Lang))
	box.Render()
}

func doShowPorts(ctx *action.ActionContext) {
	client, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	rsp, err := client.ServicePorts(ctx.Ctx, &mgmt.ServicePortsRequest{Service: ""})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	box := ctx.NewBox([]string{"SERVICE", "PORT"})
	for _, p := range rsp.Ports {
		box.AppendRow(p.Service, p.Address)
	}
	box.Render()
}
