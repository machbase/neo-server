package main

import (
	"os"

	"github.com/getlantern/systray"
	"github.com/machbase/neo-server/main/neowin/icon"
)

func main() {
	doServe()
}

func doServe() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTitle("machbase-neo")
	mStart := systray.AddMenuItem("Start", "Start machbase-neo")
	mStop := systray.AddMenuItem("Stop", "Stop machbase-neo")
	mStop.Disable()
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit machbase-neo")
	mQuit.SetIcon(icon.Data)
	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				// booter.SetConfiFileSuffix(".conf")
				// booter.SetFallbackConfig(server.DefaultFallbackConfig)
				// booter.SetFallbackPname(server.DefaultFallbackPname)
				// booter.SetVersionString(mods.VersionString() + " " + mach.LinkInfo())
				// booter.Startup()
				mStart.Disable()
				mStop.Enable()
				systray.SetTooltip("Running...")
				// booter.WaitSignal()
			case <-mStop.ClickedCh:
				mStart.Enable()
				mStop.Disable()
				//				booter.Shutdown()
				systray.SetTooltip("Stopped")
			case <-mQuit.ClickedCh:
				if mStop.Disabled() {
					systray.Quit()
				} else {
					// booter.ShutdownAndExit(0)
					os.Exit(0)
				}
			}
		}
	}()
}

func onExit() {
	systray.Quit()
}
