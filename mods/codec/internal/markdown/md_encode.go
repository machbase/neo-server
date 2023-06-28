package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
)

type Exporter struct {
	htmlRender    bool
	brief         bool
	briefMaxCount int64
	rownum        int64
	mdLines       []string

	timeLocation *time.Location
	output       spec.OutputStream
	showRownum   bool
	timeformat   string
	precision    int

	heading  bool
	colNames []string

	closeOnce sync.Once
}

func NewEncoder() *Exporter {
	ret := &Exporter{
		precision:     -1,
		timeformat:    "ns",
		briefMaxCount: 5,
	}
	return ret
}

func (ex *Exporter) ContentType() string {
	if ex.htmlRender {
		return "application/xhtml+xml"
	} else {
		return "text/markdown"
	}
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformat = format
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeLocation = tz
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

func (ex *Exporter) SetHeading(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetColumns(labels []string, types []string) {
	ex.colNames = labels
}

func (ex *Exporter) SetHtmlRender(flag bool) {
	ex.htmlRender = flag
}

func (ex *Exporter) SetBrief(flag bool) {
	ex.brief = flag
}

func (ex *Exporter) Open() error {
	if ex.output == nil {
		return errors.New("no output is assigned")
	}
	if ex.showRownum && len(ex.colNames) > 0 {
		ex.colNames = append([]string{"ROWNUM"}, ex.colNames...)
	}
	return nil
}

func (ex *Exporter) Close() {
	ex.closeOnce.Do(func() {
		tailLines := []string{}
		if ex.brief && ex.rownum > ex.briefMaxCount {
			tailLines = append(tailLines, strings.Repeat("| ... ", len(ex.colNames))+"|\n")
			tailLines = append(tailLines, fmt.Sprintf("\n> *Total* %d *records*\n", ex.rownum))
		}
		if !ex.htmlRender {
			for _, line := range tailLines {
				ex.output.Write([]byte(line))
			}
			ex.output.Close()
			return
		}

		ex.mdLines = append(ex.mdLines, tailLines...)

		md := goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				&mermaid.Extender{NoScript: true},
				highlighting.NewHighlighting(
					highlighting.WithStyle("catppuccin-macchiato"),
					highlighting.WithFormatOptions(
						chromahtml.WithLineNumbers(true),
					),
				),
			),
			goldmark.WithRendererOptions(
				html.WithHardWraps(),
				html.WithXHTML(),
			),
		)
		md.Convert(bytes.NewBufferString(strings.Join(ex.mdLines, "")).Bytes(), ex.output)
		ex.output.Close()
	})
}

func (ex *Exporter) Flush(heading bool) {
	ex.output.Flush()
}

func (ex *Exporter) encodeTime(v time.Time) string {
	switch ex.timeformat {
	case "ns":
		return strconv.FormatInt(v.UnixNano(), 10)
	case "ms":
		return strconv.FormatInt(v.UnixMilli(), 10)
	case "us":
		return strconv.FormatInt(v.UnixMicro(), 10)
	case "s":
		return strconv.FormatInt(v.Unix(), 10)
	default:
		if ex.timeLocation == nil {
			ex.timeLocation = time.UTC
		}
		return v.In(ex.timeLocation).Format(ex.timeformat)
	}
}

func (ex *Exporter) encodeFloat64(v float64) string {
	if ex.precision < 0 {
		return fmt.Sprintf("%f", v)
	} else {
		return fmt.Sprintf("%.*f", ex.precision, v)
	}
}

func (ex *Exporter) AddRow(values []any) error {
	defer func() {
		o := recover()
		if o != nil {
			fmt.Println("PANIC (csvexporter)", o)
			debug.PrintStack()
		}
	}()

	ex.rownum++

	if ex.rownum == 1 && ex.heading {
		header := "|" + strings.Join(ex.colNames, "|") + "|\n"
		headerBorder := strings.Repeat("|:-----", len(ex.colNames)) + "|\n"
		if ex.htmlRender {
			ex.mdLines = append(ex.mdLines, header, headerBorder)
		} else {
			ex.output.Write([]byte(header))
			ex.output.Write([]byte(headerBorder))
		}
	}

	if ex.brief && ex.rownum > ex.briefMaxCount {
		return nil
	}

	var cols = make([]string, len(values))

	for i, r := range values {
		if r == nil {
			cols[i] = "NULL"
			continue
		}
		switch v := r.(type) {
		case *string:
			cols[i] = *v
		case string:
			cols[i] = v
		case *time.Time:
			cols[i] = ex.encodeTime(*v)
		case time.Time:
			cols[i] = ex.encodeTime(v)
		case *float64:
			cols[i] = ex.encodeFloat64(*v)
		case float64:
			cols[i] = ex.encodeFloat64(v)
		case *float32:
			cols[i] = ex.encodeFloat64(float64(*v))
		case float32:
			cols[i] = ex.encodeFloat64(float64(v))
		case *int:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int32:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int32:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int64:
			cols[i] = strconv.FormatInt(*v, 10)
		case int64:
			cols[i] = strconv.FormatInt(v, 10)
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}

	if ex.showRownum {
		cols = append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...)
	}

	line := "|" + strings.Join(cols, "|") + "|\n"

	if ex.htmlRender {
		ex.mdLines = append(ex.mdLines, line)
	} else {
		ex.output.Write([]byte(line))
	}
	return nil
}
