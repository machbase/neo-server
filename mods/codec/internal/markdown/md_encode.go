package markdown

import (
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/mdconv"
)

type Exporter struct {
	htmlRender bool
	brief      int64
	rownum     int64
	mdLines    []string

	timeLocation *time.Location
	output       spec.OutputStream
	showRownum   bool
	timeformat   string
	precision    int

	colNames []string

	closeOnce sync.Once
}

func NewEncoder() *Exporter {
	ret := &Exporter{
		precision:  -1,
		timeformat: "ns",
		brief:      0,
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

func (ex *Exporter) SetColumns(labels ...string) {
	ex.colNames = labels
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
	if ex.showRownum && len(ex.colNames) > 0 {
		ex.colNames = append([]string{"ROWNUM"}, ex.colNames...)
	}

	headLines := []string{}
	headLines = append(headLines, "|"+strings.Join(ex.colNames, "|")+"|\n")
	headLines = append(headLines, strings.Repeat("|:-----", len(ex.colNames))+"|\n")

	if ex.htmlRender {
		ex.mdLines = append(ex.mdLines, headLines...)
	} else {
		for _, l := range headLines {
			ex.output.Write([]byte(l))
		}
	}

	return nil
}

func (ex *Exporter) Close() {
	ex.closeOnce.Do(func() {
		tailLines := []string{}
		if ex.brief > 0 && ex.rownum > ex.brief {
			tailLines = append(tailLines, strings.Repeat("| ... ", len(ex.colNames))+"|\n")
			tailLines = append(tailLines, fmt.Sprintf("\n> *Total* %s *records*\n", util.NumberFormat(ex.rownum)))
		} else if ex.rownum == 0 {
			tailLines = append(tailLines, "\n> *No record*\n")
		}
		if !ex.htmlRender {
			for _, line := range tailLines {
				ex.output.Write([]byte(line))
			}
			ex.output.Close()
			return
		}

		ex.mdLines = append(ex.mdLines, tailLines...)

		conv := mdconv.New(mdconv.WithDarkMode(false))
		ex.output.Write([]byte("<div>"))
		conv.ConvertString(strings.Join(ex.mdLines, ""), ex.output)
		ex.output.Write([]byte("</div>"))
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
	if ex.brief > 0 && ex.rownum > ex.brief {
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

	if ex.htmlRender {
		ex.mdLines = append(ex.mdLines, line)
	} else {
		ex.output.Write([]byte(line))
	}
	return nil
}
