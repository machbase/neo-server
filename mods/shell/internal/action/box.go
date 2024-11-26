package action

import (
	"io"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Boxer interface {
	NewBox(header []string) Box
	NewCompactBox(header []string) Box
}

type Box interface {
	AppendRow(row ...any)
	ResetRows()
	ResetHeaders()
	Render() string
}

func (ctx *ActionContext) NewCompactBox(header []string) Box {
	return ctx.newBox(header, true, true, "-", &util.NopCloseWriter{Writer: os.Stdout})
}

func (ctx *ActionContext) NewBox(header []string) Box {
	return ctx.newBox(header, false, true, "-", &util.NopCloseWriter{Writer: os.Stdout})
}

func (ctx *ActionContext) newBox(header []string, compact bool, heading bool, format string, mirror io.Writer) Box {
	b := &boxEnc{
		w:          table.NewWriter(),
		format:     format,
		timeformat: util.NewTimeFormatter(),
	}
	b.w.SetOutputMirror(mirror)
	b.timeformat.Set(util.Timeformat(ctx.PrefTimeformat()))
	b.timeformat.Set(util.TimeLocation(ctx.PrefTimeLocation()))

	style := table.StyleDefault
	switch ctx.Pref().BoxStyle().Value() {
	case "bold":
		style = table.StyleBold
	case "double":
		style = table.StyleDouble
	case "light":
		style = table.StyleLight
	case "round":
		style = table.StyleRounded
	}
	if compact {
		style.Options.SeparateColumns = false
		style.Options.DrawBorder = false
	} else {
		style.Options.SeparateColumns = true
		style.Options.DrawBorder = true
	}
	b.w.SetStyle(style)

	if heading {
		vs := make([]any, len(header))
		for i, h := range header {
			vs[i] = h
		}
		b.w.AppendHeader(table.Row(vs))
	}
	return b
}

type boxEnc struct {
	w          table.Writer
	format     string
	timeformat *util.TimeFormatter
}

func (b *boxEnc) AppendRow(row ...any) {
	for i, v := range row {
		if t, ok := v.(time.Time); ok {
			row[i] = b.timeformat.Format(t)
		}
	}
	b.w.AppendRow(row)
}

func (b *boxEnc) ResetRows() {
	b.w.ResetRows()
}

func (b *boxEnc) ResetHeaders() {
	b.w.ResetHeaders()
}

func (b *boxEnc) Render() string {
	if b.format == Formats.CSV {
		return b.w.RenderCSV()
	} else {
		return b.w.Render()
	}
}
