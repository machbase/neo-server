package shell

import (
	"runtime"
	"strings"

	mach "github.com/machbase/dbms-mach-go"
)

func (sess *Session) exec_show(line string) {
	toks := strings.Fields(line)
	if len(toks) == 1 {
		sess.Println("Usage: SHOW [VERSION | CONFIG]")
		return
	}
	if toks[0] != "SHOW" || len(toks) == 1 {
		sess.log.Errorf("invalid show command: %s", line)
		return
	}
	switch toks[1] {
	case "VERSION":
		v := mach.GetVersion()
		sess.Printf("Server v%d.%d.%d #%s", v.Major, v.Minor, v.Patch, v.GitSHA)
		sess.Printf("Engine %s", mach.LibMachLinkInfo)
	case "CONFIG":
		sess.Println(sess.server.GetConfig())
	case "RUNTIME":
		width := 15
		sess.Printf("%-*s %s %s", width, "os", runtime.GOOS, runtime.GOARCH)
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
}
