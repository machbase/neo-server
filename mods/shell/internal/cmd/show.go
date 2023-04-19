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
    info             show server info
    indexgap         INDEX GAP status
	lsm              LSM status
    tables           list tables
      -a,--all       includes all hidden tables
    meta-tables      show meta tables
    virtual-tables   show virtual tables
`

type ShowCmd struct {
	Object  string `arg:""`
	ShowAll bool   `name:"all" short:"a"`
	Help    bool   `kong:"-"`
}

func pcShow() readline.PrefixCompleterInterface {
	return readline.PcItem("show",
		readline.PcItem("info"),
		readline.PcItem("indexgap"),
		readline.PcItem("lsm"),
		readline.PcItem("tables"),
		readline.PcItem("meta-tables"),
		readline.PcItem("virtual-tables"),
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
	case "indexgap":
		doShowIndexGap(ctx)
	case "lsm":
		doShowLsm(ctx)
	case "tables":
		doShowTables(ctx, cmd.ShowAll)
	case "meta-tables":
		doShowMVTables(ctx, "M$TABLES")
	case "virtual-tables":
		doShowMVTables(ctx, "V$TABLES")
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

func doShowTables(ctx *client.ActionContext, showAll bool) {
	sqlText := `SELECT
			j.DB_NAME as DB_NAME,
			u.NAME as USER_NAME,
			j.NAME as TABLE_NAME,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG
		from
			M$SYS_USERS u,
			(select
				a.NAME as NAME,
				a.USER_ID as USER_ID,
				a.TYPE as TYPE,
				a.FLAG as FLAG,
				case a.DATABASE_ID
					when -1 then 'MACHBASEDB'
					else d.MOUNTDB
				end as DB_NAME
			from M$SYS_TABLES a
				left join V$STORAGE_MOUNT_DATABASES d on a.DATABASE_ID = d.BACKUP_TBSID) as j
		where
			u.USER_ID = j.USER_ID
		order by j.NAME
		`

	rows, err := ctx.DB.Query(sqlText)
	if err != nil {
		ctx.Printfln("ERR show tables fail; %s", err.Error())
		return
	}
	defer rows.Close()

	t := ctx.NewBox([]string{"ROWNUM", "DB", "USER", "NAME", "TYPE"})

	nrow := 0
	for rows.Next() {
		var dbname string
		var user string
		var name string
		var typ int
		var flg int
		err := rows.Scan(&dbname, &user, &name, &typ, &flg)
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		if !showAll && strings.HasPrefix(name, "_") {
			continue
		}
		nrow++

		desc := do.TableTypeDescription(spi.TableType(typ), flg)
		t.AppendRow(nrow, dbname, user, name, desc)
	}
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
