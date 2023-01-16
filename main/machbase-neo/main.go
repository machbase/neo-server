package main

import (
	"github.com/machbase/booter"
	_ "github.com/machbase/cemlib/banner"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
)

func main() {
	booter.SetConfiFileSuffix(".conf")
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + mods.EngineInfoString())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}
