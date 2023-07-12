package echart

import (
	"github.com/machbase/neo-server/mods/stream/spec"
)

type ChartBase struct {
	output spec.OutputStream

	title    string
	subtitle string
	theme    string
	width    string
	height   string

	assetHost    string
	toJsonOutput bool
}

func (ex *ChartBase) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *ChartBase) SetSize(width, height string) {
	ex.width = width
	ex.height = height
}

func (ex *ChartBase) SetTheme(theme string) {
	ex.theme = theme
}

func (ex *ChartBase) SetTitle(title string) {
	ex.title = title
}

func (ex *ChartBase) SetSubtitle(subtitle string) {
	ex.subtitle = subtitle
}

func (ex *ChartBase) SetAssetHost(path string) {
	ex.assetHost = path
}

func (ex *ChartBase) SetChartJson(flag bool) {
	ex.toJsonOutput = flag
}

func (ex *ChartBase) Theme() string {
	if ex.theme == "" {
		if ex.toJsonOutput {
			return "-" // client choose 'white' or 'dark'
		} else {
			return "white" // echarts default
		}
	} else {
		return ex.theme
	}
}
