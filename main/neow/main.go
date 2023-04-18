package main

import (
	"os"

	"github.com/getlantern/systray"
	"github.com/machbase/neo-server/main/neowin/icon"
)

func main() {
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
				mStart.Disable()
				mStop.Enable()
				systray.SetTooltip("Running...")
			case <-mStop.ClickedCh:
				mStart.Enable()
				mStop.Disable()
				systray.SetTooltip("Stopped")
			case <-mQuit.ClickedCh:
				if mStop.Disabled() {
					systray.Quit()
				} else {
					os.Exit(0)
				}
			}
		}
	}()
}

func onExit() {
	systray.Quit()
}
