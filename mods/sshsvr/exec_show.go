package shell

import (
	"runtime"
	"strings"

	"github.com/machbase/neo-server/mods"
)

func (sess *Session) exec_show(line string) {
	toks := strings.Fields(line)
	if len(toks) == 1 {
		sess.Println("Usage: SHOW [TABLES | VERSION]")
		return
	}
	if toks[0] != "SHOW" || len(toks) == 1 {
		sess.log.Errorf("invalid show command: %s", line)
		return
	}
	switch toks[1] {
	case "VERSION":
		v := mods.GetVersion()
		sess.Printf("Server v%d.%d.%d #%s", v.Major, v.Minor, v.Patch, v.GitSHA)
		sess.Printf("Engine %s", mods.EngineInfoString())
	case "TABLES":
		sess.exec_show_tables()
	case "CONFIG":
		sess.Println(sess.server.GetConfig())
	case "RUNTIME":
		sess.exec_show_runtime()
	}
}

func (sess *Session) exec_show_tables() {
	rows, err := sess.db.Query("select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
	if err != nil {
		sess.log.Errorf("select m$sys_tables fail; %s", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var typ int
		var flg int
		rows.Scan(&name, &typ, &flg)
		desc := tableTypeDesc(typ, flg)
		sess.Printf("%-24s %s", name, desc)
	}
}

func (sess *Session) exec_show_runtime() {
	width := 15
	sess.Printf("%-*s %s %s", width, "os", runtime.GOOS, runtime.GOARCH)
	sess.Printf("%-*s %s", width, "version", mods.VersionString())
	sess.Printf("%-*s %s", width, "engine", mods.EngineInfoString())
	sess.Printf("%-*s %d", width, "processes", runtime.GOMAXPROCS(-1))

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	// sess.Printf("%-*s %d", width, "mem alloc", mem.Alloc)
	// sess.Printf("%-*s %d", width, "mem frees", mem.Frees)
	sess.Printf("%-*s %d", width, "mem in-use span", mem.HeapInuse/1024/1024)
	sess.Printf("%-*s %d", width, "mem idle span", mem.HeapIdle/1024/1024)
	// total bytes of memory obtained from the OS
	// Sys measures the virtual address space reserved
	// by the Go runtime for the heap, stacks, and other internal data structures.
	sess.Printf("%-*s %d MB", width, "mem sys", mem.Sys/1024/1024)
	// bytes of allocated for heap objects.
	sess.Printf("%-*s %d MB", width, "mem heap alloc", mem.HeapAlloc/1024/1024)
}
