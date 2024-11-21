package markdown

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

type Exporter struct {
	internal.RowsEncoderBase
	logger     facility.Logger
	htmlRender bool
	brief      int64
	rownum     int64
	mdLines    []string

	output        io.Writer
	showRownum    bool
	precision     int
	timeformatter *util.TimeFormatter

	headerNames []string
	closeOnce   sync.Once
}

func NewEncoder() *Exporter {
	ret := &Exporter{
		precision:     -1,
		timeformatter: util.NewTimeFormatter(),
		brief:         0,
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

func (ex *Exporter) SetLogger(l facility.Logger) {
	ex.logger = l
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetColumns(colNames ...string) {
	ex.headerNames = colNames
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformatter.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeformatter.Set(util.TimeLocation(tz))
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

func (ex *Exporter) SetHtml(flag bool) {
	ex.htmlRender = flag
}

func (ex *Exporter) SetBrief(flag bool) {
	if flag {
		ex.SetBriefCount(5)
	} else {
		ex.SetBriefCount(0)
	}
}

func (ex *Exporter) SetBriefCount(count int) {
	ex.brief = int64(count)
}

func (ex *Exporter) Open() error {
	if ex.output == nil {
		return errors.New("no output is assigned")
	}
	return nil
}

func (ex *Exporter) Close() {
	ex.closeOnce.Do(func() {
		if ex.showRownum && len(ex.headerNames) > 0 {
			ex.headerNames = append([]string{"ROWNUM"}, ex.headerNames...)
		}
		headLines := []string{}
		headLines = append(headLines, "|"+strings.Join(ex.headerNames, "|")+"|\n")
		headLines = append(headLines, strings.Repeat("|:-----", len(ex.headerNames))+"|\n")

		tailLines := []string{}
		if ex.brief > 0 && ex.rownum > ex.brief {
			tailLines = append(tailLines, strings.Repeat("| ... ", len(ex.headerNames))+"|\n")
			tailLines = append(tailLines, fmt.Sprintf("\n> *Total* %s *records*\n", util.NumberFormat(ex.rownum)))
		} else if ex.rownum == 0 {
			tailLines = append(tailLines, "\n> *No record*\n")
		}

		if ex.htmlRender {
			ex.mdLines = append(headLines, ex.mdLines...)
			ex.mdLines = append(ex.mdLines, tailLines...)
			conv := mdconv.New(mdconv.WithDarkMode(false))
			ex.output.Write([]byte("<div>\n"))
			conv.ConvertString(strings.Join(ex.mdLines, ""), ex.output)
			ex.output.Write([]byte("</div>"))
		} else {
			for _, l := range headLines {
				ex.output.Write([]byte(l))
			}
			for _, l := range ex.mdLines {
				ex.output.Write([]byte(l))
			}
			for _, line := range tailLines {
				ex.output.Write([]byte(line))
			}
		}
		if closer, ok := ex.output.(io.Closer); ok {
			closer.Close()
		}
	})
}

func (ex *Exporter) Flush(heading bool) {
	if flusher, ok := ex.output.(api.Flusher); ok {
		flusher.Flush()
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
			if ex.logger != nil {
				ex.logger.LogError("PANIC (csvexporter)", o)
			}
			debug.PrintStack()
		}
	}()

	ex.rownum++
	if ex.brief > 0 && ex.rownum > ex.brief {
		return nil
	}

	if len(ex.headerNames) != len(values) {
		ex.headerNames = make([]string, len(values))
		for i := range ex.headerNames {
			ex.headerNames[i] = fmt.Sprintf("column%d", i)
		}
	}
	var cols = make([]string, len(values))

	for i, r := range values {
		if r == nil {
			cols[i] = "NULL"
			continue
		}
		switch v := r.(type) {
		case *bool:
			cols[i] = strconv.FormatBool(*v)
		case bool:
			cols[i] = strconv.FormatBool(v)
		case *string:
			cols[i] = *v
		case string:
			cols[i] = v
		case *time.Time:
			cols[i] = ex.timeformatter.Format(*v)
		case time.Time:
			cols[i] = ex.timeformatter.Format(v)
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
		case *net.IP:
			cols[i] = v.String()
		case net.IP:
			cols[i] = v.String()
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}

	if ex.showRownum {
		cols = append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...)
	}

	line := "|" + strings.Join(cols, "|") + "|\n"
	ex.mdLines = append(ex.mdLines, line)

	return nil
}
