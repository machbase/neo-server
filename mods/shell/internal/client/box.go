package client

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
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
	return ctx.newBox(header, true, true, "-", ctx.Stdout)
}

func (ctx *ActionContext) NewBox(header []string) Box {
	return ctx.newBox(header, false, true, "-", ctx.Stdout)
}

func (ctx *ActionContext) newBox(header []string, compact bool, heading bool, format string, mirror io.Writer) Box {
	b := &box{
		w:      table.NewWriter(),
		format: format,
	}
	b.w.SetOutputMirror(mirror)

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

type box struct {
	w      table.Writer
	format string
}

func (b *box) AppendRow(row ...any) {
	b.w.AppendRow(row)
}

func (b *box) ResetRows() {
	b.w.ResetRows()
}

func (b *box) ResetHeaders() {
	b.w.ResetHeaders()
}

func (b *box) Render() string {
	if b.format == Formats.CSV {
		return b.w.RenderCSV()
	} else {
		return b.w.Render()
	}
}
