package main

import (
	"github.com/machbase/booter"
	_ "github.com/machbase/cemlib/banner"
	mach "github.com/machbase/dbms-mach-go"
	"github.com/machbase/dbms-mach-go/server"
)

func main() {
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mach.VersionString())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}
