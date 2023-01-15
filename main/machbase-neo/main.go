package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/machbase/booter"
	_ "github.com/machbase/cemlib/banner"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/server"
)

func main() {
	if len(os.Args) > 1 && strings.ToLower(os.Args[1]) == "gen-config" {
		fmt.Println(string(server.DefaultFallbackConfig))
		return
	}
	booter.SetFallbackConfig(server.DefaultFallbackConfig)
	booter.SetFallbackPname(server.DefaultFallbackPname)
	booter.SetVersionString(mods.VersionString() + " " + mods.EngineInfoString())
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}
