package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/machbase/neo-server/main/neow/res"
)

func PanicWindow(message string) {
	iconLogo := fyne.NewStaticResource("logo.png", res.Logo)
	a := app.NewWithID("com.machbase.neow")
	a.SetIcon(iconLogo)
	a.Settings().SetTheme(newAppTheme())

	window := a.NewWindow("machbase-neo")
	window.SetMaster()
	msgLabel := widget.NewLabel(message)
	titleLabel := widget.NewLabel("Error")
	closeButton := widget.NewButton("Close", func() {
		window.Close()
	})
	bottomBox := container.New(layout.NewCenterLayout(), closeButton)
	window.SetContent(container.New(layout.NewBorderLayout(titleLabel, bottomBox, nil, nil), titleLabel, msgLabel, bottomBox))
	window.ShowAndRun()
}
