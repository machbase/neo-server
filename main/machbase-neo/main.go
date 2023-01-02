package main

import (
	"github.com/machbase/booter"
	_ "github.com/machbase/cemlib/banner"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/server"
)

func main() {
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mach.VersionString())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}
