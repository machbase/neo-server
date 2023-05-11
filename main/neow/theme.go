package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/machbase/neo-server/main/neow/res"
)

type appTheme struct {
	base        fyne.Theme
	defaultFont fyne.Resource
}

func newAppTheme() fyne.Theme {
	fontDefault := fyne.NewStaticResource("default_font", res.D2Coding)
	return &appTheme{base: fyne.CurrentApp().Settings().Theme(), defaultFont: fontDefault}
}

// Color fixes a bug < 2.1 where theme.DarkTheme() would not override user preference.
func (th *appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case termOverlay:
		if c := th.Color("fynedeskPanelBackground", variant); c != color.Transparent {
			return c
		}
		if variant == theme.VariantLight {
			return color.NRGBA{R: 0xaa, G: 0xaa, B: 0xaa, A: 0xf6}
		}
		return color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0a, A: 0xf6}
	}
	// variant = theme.VariantLight, theme.VariantDark
	return th.base.Color(name, variant)
}

func (th *appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return th.defaultFont
}
func (th *appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return th.base.Icon(name)
}
func (th *appTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 12
	}

	return th.base.Size(name)
}

const termOverlay = fyne.ThemeColorName("termOver")
