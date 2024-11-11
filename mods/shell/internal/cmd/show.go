package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/shell/internal/action"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
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
	sqlText := `select 
		b.name as TABLE_NAME, 
		c.name as INDEX_NAME, 
		a.table_end_rid - a.end_rid as GAP
	from
		v$storage_dc_table_indexes a,
		m$sys_tables b, m$sys_indexes c
	where
		a.id = c.id 
	and c.table_id = b.id 
	order by 3 desc`

	doShowByQuery0(ctx, sqlText, true)
}

func doShowLsm(ctx *action.ActionContext) {
	sqlText := `select 
		b.name as TABLE_NAME,
		c.name as INDEX_NAME,
		a.level as LEVEL,
		a.end_rid - a.begin_rid as COUNT
	from
		v$storage_dc_lsmindex_levels a,
		m$sys_tables b, m$sys_indexes c
	where
		c.id = a.index_id 
	and b.id = a.table_id
	order by 1, 2, 3`

	doShowByQuery0(ctx, sqlText, true)
}

func doShowUsers(ctx *action.ActionContext) {
	sqlText := "select name USER_NAME from m$sys_users"
	doShowByQuery0(ctx, sqlText, true)
}

func doShowIndexes(ctx *action.ActionContext) {
	list, err := api.ListIndexes(ctx.Ctx, ctx.Conn)
	if err != nil {
		ctx.Println("unable to find indexes; ERR", err.Error())
		return
	}
	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "USER_NAME", "DB", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE"})
	for _, nfo := range list {
		nrow++
		box.AppendRow(nrow, nfo.User, nfo.Database, nfo.Table, nfo.Cols[0], nfo.Name, nfo.Type)
	}
	box.Render()
}

func doShowIndex(ctx *action.ActionContext, args []string) {
	if len(args) != 1 {
		ctx.Println("missing index name")
		ctx.Println("Usage: show index <index>")
		return
	}
	sqlText := `select 
		a.name as TABLE_NAME,
		c.name as COLUMN_NAME,
		b.name as INDEX_NAME,
		case b.type
			when 1 then 'BITMAP'
			when 2 then 'KEYWORD'
			when 5 then 'REDBLACK'
			when 6 then 'LSM'
			when 8 then 'REDBLACK'
			when 9 then 'KEYWORD_LSM'
			else 'LSM' end 
		as INDEX_TYPE,
		case b.key_compress
			when 0 then 'UNCOMPRESSED'
			else 'COMPRESSED' end 
		as KEY_COMPRESS,
		b.max_level as MAX_LEVEL,
		b.part_value_count as PART_VALUE_COUNT,
		case b.bitmap_encode
			when 0 then 'EQUAL'
			else 'RANGE' end 
		as BITMAP_ENCODE
	from
		m$sys_tables a,
		m$sys_indexes b,
		m$sys_index_columns c
	where
		a.id = b.table_id 
	and b.id = c.index_id
	and b.name = '%s'`
	sqlText = fmt.Sprintf(sqlText, args[0])
	doShowByQuery0(ctx, sqlText, true)
}

func doShowLicense(ctx *action.ActionContext) {
	sqlText := "select ID, TYPE, CUSTOMER, PROJECT, COUNTRY_CODE, INSTALL_DATE from v$license_info"
	doShowByQuery0(ctx, sqlText, true)
}

func doShowStatements(ctx *action.ActionContext) {
	sqlText := "SELECT ID USER_ID, SESS_ID SESSION_ID, QUERY FROM V$STMT"
	doShowByQuery0(ctx, sqlText, true)
}

func doShowStorage(ctx *action.ActionContext) {
	sqlText := `select
		a.table_name as TABLE_NAME,
		a.data_size as DATA_SIZE,
		case b.index_size 
			when b.index_size then b.index_size 
			else 0 end 
		as INDEX_SIZE,
		case a.data_size + b.index_size 
			when a.data_size + b.index_size then a.data_size + b.index_size 
			else a.data_size end 
		as TOTAL_SIZE
	from
		(select
			a.name as table_name,
			sum(b.storage_usage) as data_size
		from
			m$sys_tables a,
			v$storage_tables b
		where a.id = b.id
		group by a.name
		) as a LEFT OUTER JOIN
		(select
			a.name,
			sum(b.disk_file_size) as index_size
		from
			m$sys_tables a,
			v$storage_dc_table_indexes b
		where a.id = b.table_id
		group by a.name) as b
	on a.table_name = b.name
	order by a.table_name`
	doShowByQuery0(ctx, sqlText, true)
}

func doShowTableUsage(ctx *action.ActionContext) {
	sqlText := `SELECT
		a.NAME as TABLE_NAME,
		t.STORAGE_USAGE as STORAGE_USAGE
	FROM
		M$SYS_TABLES a,
		M$SYS_USERS u,
		V$STORAGE_TABLES t
	WHERE
		a.user_id = u.user_id
	AND t.ID = a.id
	ORDER BY a.NAME`
	doShowByQuery0(ctx, sqlText, true)
}

func doShowRollupGap(ctx *action.ActionContext) {
	sqlText := `SELECT
		C.SOURCE_TABLE AS SRC_TABLE,
		C.ROLLUP_TABLE,
		B.TABLE_END_RID AS SRC_END_RID,
		C.END_RID AS ROLLUP_END_RID,
		B.TABLE_END_RID - C.END_RID AS GAP,
		C.LAST_ELAPSED_MSEC AS LAST_TIME
	FROM
		M$SYS_TABLES A,
		V$STORAGE_TAG_TABLES B,
		V$ROLLUP C
	WHERE
		A.ID=B.ID
	AND A.NAME=C.SOURCE_TABLE
	ORDER BY SRC_TABLE`
	doShowByQuery0(ctx, sqlText, true)
}

func doShowTagIndexGap(ctx *action.ActionContext) {
	sqlText := `SELECT
		ID,
		INDEX_STATE AS STATUS,
		TABLE_END_RID - DISK_INDEX_END_RID AS DISK_GAP,
		TABLE_END_RID - MEMORY_INDEX_END_RID AS MEMORY_GAP
	FROM
		V$STORAGE_TAG_TABLES
	ORDER BY 1`
	doShowByQuery0(ctx, sqlText, true)
}

func doShowTags(ctx *action.ActionContext, args []string) {
	if len(args) != 1 {
		ctx.Println("missing table name")
		ctx.Println("Usage: show tags <table>")
		return
	}

	tableName := args[0]
	tableType, err := api.QueryTableType(ctx.Ctx, ctx.Conn, tableName)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if tableType != api.TableTypeTag {
		ctx.Println("ERR", fmt.Sprintf("'%s' is not a tag table", tableName))
		return
	}

	desc, err := api.DescribeTable(ctx.Ctx, ctx.Conn, tableName, false)
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
	api.ListTagsWalk(ctx.Ctx, ctx.Conn, strings.ToUpper(args[0]), func(tag *api.TagInfo, err error) bool {
		if err != nil {
			ctx.Println("ERR", err.Error())
			return false
		}
		nrow++
		if summarized {
			stat, err := api.TagStat(ctx.Ctx, ctx.Conn, tableName, tag.Name)
			if err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			t.AppendRow(nrow, tag.Id, tag.Name, stat.RowCount,
				stat.MinTime, stat.MaxTime, stat.RecentRowTime,
				stat.MinValue, stat.MinValueTime,
				stat.MaxValue, stat.MaxValueTime)
		} else {
			stat, err := api.TagStat(ctx.Ctx, ctx.Conn, tableName, tag.Name)
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

	t := ctx.NewBox([]string{"NAME", "VALUE"})
	stat, err := api.TagStat(ctx.Ctx, ctx.Conn, args[0], args[1])
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
	var output spec.OutputStream
	output, err := stream.NewOutputStream("-")
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	defer output.Close()

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
	if err := query.Execute(ctx.Ctx, ctx.Conn, sqlText); err != nil {
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

	desc, err := api.DescribeTable(ctx.Ctx, ctx.Conn, table, showAll)
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
	t := ctx.NewBox([]string{"ROWNUM", "DB", "USER", "NAME", "ID", "TYPE"})
	nrow := 0
	api.ListTablesWalk(ctx.Ctx, ctx.Conn, showAll, func(ti *api.TableInfo, err error) bool {
		if err != nil {
			ctx.Println("ERR", err.Error())
			return false
		}
		// if !showAll && strings.HasPrefix(ti.Name, "_") {
		// 	return true
		// }
		if ctx.Actor.Username() != "sys" && ti.Database != "MACHBASEDB" {
			return true
		}
		nrow++
		desc := api.TableTypeDescription(ti.Type, ti.Flag)
		t.AppendRow(nrow, ti.Database, ti.User, ti.Name, ti.Id, desc)
		return true
	})
	t.Render()
}

func doShowMVTables(ctx *action.ActionContext, tablesTable string) {
	rows, err := ctx.Conn.Query(ctx.Ctx, fmt.Sprintf("select NAME, TYPE, FLAG, ID from %s order by ID", tablesTable))
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
