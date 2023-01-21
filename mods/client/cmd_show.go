package client

import (
	"runtime"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods"
)

func (cli *client) pcShow() *readline.PrefixCompleter {
	return readline.PcItem("show",
		readline.PcItem("tables"),
		readline.PcItem("runtime"),
		readline.PcItem("version"),
	)
}

func (cli *client) doShow(obj string) {
	switch obj {
	case "version":
		cli.doShowVersion()
	case "tables":
		cli.doShowTables()
	case "runtime":
		cli.doShowRuntime()
	default:
		cli.Writef("unknown show '%s'", obj)
	}
}

func (cli *client) doShowTables() {
	rows, err := cli.db.Query("select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
	if err != nil {
		cli.Writef("ERR select m$sys_tables fail; %s", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var typ int
		var flg int
		rows.Scan(&name, &typ, &flg)
		desc := tableTypeDesc(typ, flg)
		cli.Writef("%-24s %s", name, desc)
	}
}

func (cli *client) doShowVersion() {
	v := mods.GetVersion()
	cli.Printf("Server v%d.%d.%d #%s\r\n", v.Major, v.Minor, v.Patch, v.GitSHA)
	cli.Printf("Engine %s\r\n", mods.EngineInfoString())
}

func (cli *client) doShowRuntime() {
	width := 15
	cli.Writef("%-*s %s %s", width, "os", runtime.GOOS, runtime.GOARCH)
	cli.Writef("%-*s %s", width, "version", mods.VersionString())
	cli.Writef("%-*s %s", width, "engine", mods.EngineInfoString())
	cli.Writef("%-*s %d", width, "processes", runtime.GOMAXPROCS(-1))

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	// sess.Printf("%-*s %d", width, "mem alloc", mem.Alloc)
	// sess.Printf("%-*s %d", width, "mem frees", mem.Frees)
	cli.Writef("%-*s %d", width, "mem in-use span", mem.HeapInuse/1024/1024)
	cli.Writef("%-*s %d", width, "mem idle span", mem.HeapIdle/1024/1024)
	// total bytes of memory obtained from the OS
	// Sys measures the virtual address space reserved
	// by the Go runtime for the heap, stacks, and other internal data structures.
	cli.Writef("%-*s %d MB", width, "mem sys", mem.Sys/1024/1024)
	// bytes of allocated for heap objects.
	cli.Writef("%-*s %d MB", width, "mem heap alloc", mem.HeapAlloc/1024/1024)
}
