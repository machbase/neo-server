package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "show",
		PcFunc: pcShow,
		Action: doShow,
		Desc:   "Display information",
		Usage:  helpShow,
	})
}

const helpShow = `  show [options] <object>
  objects:
    info                show server info
    ports               show service ports
    users               list users
    tables [-a]         list tables
    table [-a] <table>  describe the table
    meta-tables         list meta tables
    virtual-tables      list virtual tables
    statements          list statements
    indexes             list indexes
    index <index>       describe the index
    storage             show storage info
    table-usage         show table usage
    lsm                 LSM status
    indexgap            index gap status
    rollupgap           rollup gap status
    tagindexgap         tag index gap status
    tags <table>        tag list of the table
  options:
    -a,--all            includes all hidden tables/columns
`

type ShowCmd struct {
	Object  string   `arg:""`
	Args    []string `arg:"" optional:""`
	ShowAll bool     `name:"all" short:"a"`
	Help    bool     `kong:"-"`
}

func pcShow() readline.PrefixCompleterInterface {
	return readline.PcItem("show",
		readline.PcItem("info"),
		readline.PcItem("ports"),
		readline.PcItem("users"),
		readline.PcItem("tables"),
		readline.PcItem("table"),
		readline.PcItem("meta-tables"),
		readline.PcItem("virtual-tables"),
		readline.PcItem("statements"),
		readline.PcItem("indexes"),
		readline.PcItem("index"),
		readline.PcItem("storage"),
		readline.PcItem("table-usage"),
		readline.PcItem("lsm"),
		readline.PcItem("indexgap"),
		readline.PcItem("rollupgap"),
		readline.PcItem("tagindexgap"),
		readline.PcItem("tags"),
	)
}

func doShow(ctx *client.ActionContext) {
	cmd := &ShowCmd{}

	parser, err := client.Kong(cmd, func() error { ctx.Println(helpShow); cmd.Help = true; return nil })
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
	case "sessions":
		doShowSessions(ctx)
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
	default:
		ctx.Println(helpShow)
		return
	}
}

func doShowIndexGap(ctx *client.ActionContext) {
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

	doShowByQuery0(ctx, sqlText)
}

func doShowLsm(ctx *client.ActionContext) {
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

	doShowByQuery0(ctx, sqlText)
}

func doShowUsers(ctx *client.ActionContext) {
	sqlText := "select name USER_NAME from m$sys_users"
	doShowByQuery0(ctx, sqlText)
}

func doShowIndexes(ctx *client.ActionContext) {
	sqlText := `select 
		u.name as USER_NAME,
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
	when 11 then 'TAG'
	else 'LSM' end as INDEX_TYPE
	from
		m$sys_tables a, 
		m$sys_indexes b, 
		m$sys_index_columns c, 
		m$sys_users u
	where
		a.id = b.table_id
	and b.id = c.index_id
	and a.user_id = u.user_id`
	doShowByQuery0(ctx, sqlText)
}

func doShowIndex(ctx *client.ActionContext, args []string) {
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
	doShowByQuery0(ctx, sqlText)
}

func doShowSessions(ctx *client.ActionContext) {
	sqlText := "select ID, USER_ID, LOGIN_TIME, MAX_QPX_MEM from v$session"
	doShowByQuery0(ctx, sqlText)
}

func doShowLicense(ctx *client.ActionContext) {
	sqlText := "select INSTALL_DATE, ISSUE_DATE, TYPE from v$license_info"
	doShowByQuery0(ctx, sqlText)
}

func doShowStatements(ctx *client.ActionContext) {
	sqlText := "SELECT ID USER_ID, SESS_ID SESSION_ID, QUERY FROM V$STMT"
	doShowByQuery0(ctx, sqlText)
}

func doShowStorage(ctx *client.ActionContext) {
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
	doShowByQuery0(ctx, sqlText)
}

func doShowTableUsage(ctx *client.ActionContext) {
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
	doShowByQuery0(ctx, sqlText)
}

func doShowRollupGap(ctx *client.ActionContext) {
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
	doShowByQuery0(ctx, sqlText)
}

func doShowTagIndexGap(ctx *client.ActionContext) {
	sqlText := `SELECT
		ID,
		INDEX_STATE AS STATUS,
		TABLE_END_RID - DISK_INDEX_END_RID AS DISK_GAP,
		TABLE_END_RID - MEMORY_INDEX_END_RID AS MEMORY_GAP
	FROM
		V$STORAGE_TAG_TABLES
	ORDER BY 1`
	doShowByQuery0(ctx, sqlText)
}

func doShowTags(ctx *client.ActionContext, args []string) {
	if len(args) != 1 {
		ctx.Println("missing table name")
		ctx.Println("Usage: show tags <table>")
		return
	}

	sqlText := fmt.Sprintf("select name from _%s_META order by name", strings.ToUpper(args[0]))
	doShowByQuery0(ctx, sqlText)
}

func doShowByQuery0(ctx *client.ActionContext, sqlText string) {
	var output spi.OutputStream
	output, err := stream.NewOutputStream("-")
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	defer output.Close()

	encoder := codec.NewEncoderBuilder(codec.BOX).
		SetOutputStream(output).
		SetRownum(true).
		SetHeading(true).
		SetBoxStyle(ctx.Pref().BoxStyle().Value()).
		Build()

	queryCtx := &do.QueryContext{
		DB: ctx.DB,
		OnFetchStart: func(cols spi.Columns) {
			encoder.Open(cols)
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := encoder.AddRow(values)
			if err != nil {
				ctx.Println("ERR", err.Error())
			}
			return true
		},
		OnFetchEnd: func() {
			encoder.Close()
		},
	}
	msg, err := do.Query(queryCtx, sqlText)
	if err != nil {
		ctx.Println("ERR", err.Error())
	} else {
		ctx.Println(msg)
	}
}

func doShowTable(ctx *client.ActionContext, args []string, showAll bool) {
	if len(args) != 1 {
		ctx.Println("missing table name")
		ctx.Println("Usage: show table [-a] <table>")
		return
	}

	table := args[0]

	_desc, err := do.Describe(ctx.DB, table, showAll)
	if err != nil {
		ctx.Println("unable to describe", table, "; ERR", err.Error())
		return
	}
	desc := _desc.(*do.TableDescription)

	nrow := 0
	box := ctx.NewBox([]string{"ROWNUM", "NAME", "TYPE", "LENGTH"})
	for _, col := range desc.Columns {
		nrow++
		colType := spi.ColumnTypeString(col.Type)
		box.AppendRow(nrow, col.Name, colType, col.Length)
	}

	box.Render()
}

func doShowTables(ctx *client.ActionContext, showAll bool) {
	t := ctx.NewBox([]string{"ROWNUM", "DB", "USER", "NAME", "TYPE"})
	nrow := 0
	do.Tables(ctx.DB, func(ti *do.TableInfo, err error) bool {
		if err != nil {
			ctx.Println("ERR", err.Error())
			return false
		}
		if !showAll && strings.HasPrefix(ti.Name, "_") {
			return true
		}
		nrow++
		desc := do.TableTypeDescription(spi.TableType(ti.Type), ti.Flag)
		t.AppendRow(nrow, ti.Database, ti.User, ti.Name, desc)
		return true
	})
	t.Render()
}

func doShowMVTables(ctx *client.ActionContext, tablesTable string) {
	rows, err := ctx.DB.Query(fmt.Sprintf("select NAME, TYPE, FLAG, ID from %s order by ID", tablesTable))
	if err != nil {
		ctx.Printfln("ERR select %s fail; %s", tablesTable, err.Error())
		return
	}
	defer rows.Close()

	t := ctx.NewBox([]string{"ROWNUM", "ID", "NAME", "TYPE"})

	nrow := 0
	for rows.Next() {
		var name string
		var typ int
		var flg int
		var id int
		err := rows.Scan(&name, &typ, &flg, &id)
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		nrow++

		desc := do.TableTypeDescription(spi.TableType(typ), flg)
		t.AppendRow(nrow, id, name, desc)
	}
	t.Render()
}

func doShowInfo(ctx *client.ActionContext) {
	nfo, err := ctx.DB.GetServerInfo()
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
	box.AppendRow("runtime.goroutines", nfo.Runtime.Goroutines)

	box.AppendRow("mem.sys", util.BytesUnit(nfo.Runtime.MemSys, ctx.Lang))
	box.AppendRow("mem.heap.sys", util.BytesUnit(nfo.Runtime.MemHeapSys, ctx.Lang))
	box.AppendRow("mem.heap.alloc", util.BytesUnit(nfo.Runtime.MemHeapAlloc, ctx.Lang))
	box.AppendRow("mem.heap.in-use", util.BytesUnit(nfo.Runtime.MemHeapInUse, ctx.Lang))
	box.AppendRow("mem.stack.sys", util.BytesUnit(nfo.Runtime.MemStackSys, ctx.Lang))
	box.AppendRow("mem.stack.in-use", util.BytesUnit(nfo.Runtime.MemStackInUse, ctx.Lang))

	box.Render()
}

func doShowPorts(ctx *client.ActionContext) {
	ports, err := ctx.DB.GetServicePorts("")
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	box := ctx.NewBox([]string{"SERVICE", "PORT"})
	for _, p := range ports {
		box.AppendRow(p.Service, p.Address)
	}
	box.Render()
}
