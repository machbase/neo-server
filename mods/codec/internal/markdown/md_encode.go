package markdown

import (
	"errors"
	"fmt"
	htmlTemplate "html/template"
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	txtTemplate "text/template"
	"time"

	"github.com/machbase/neo-client/api"
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

	output          io.Writer
	showRownum      bool
	precision       int
	timeformatter   *util.TimeFormatter
	binaryFormatter *util.BinaryFormatter
	templates       []string
	tmpl            *txtTemplate.Template
	record          *Record

	headerNames []string
	closeOnce   sync.Once
}

func NewEncoder() *Exporter {
	ret := &Exporter{
		precision:       -1,
		timeformatter:   util.NewTimeFormatter(),
		binaryFormatter: util.NewBinaryFormatter("hex"),
		brief:           0,
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

func (ex *Exporter) SetBinaryformat(format string) {
	ex.binaryFormatter.SetFormat(format)
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

func (ex *Exporter) SetTemplate(templates ...string) {
	ex.templates = append(ex.templates, templates...)
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
	if len(ex.templates) > 0 {
		tmpl := txtTemplate.New("markdown_layout").Funcs(templateFuncs())
		for _, content := range ex.templates {
			if _, err := tmpl.Parse(content); err != nil {
				return err
			}
		}
		ex.tmpl = tmpl
	}
	return nil
}

func (ex *Exporter) Close() {
	ex.closeOnce.Do(func() {
		if ex.tmpl != nil {
			ex.closeTemplatePath()
			if closer, ok := ex.output.(io.Closer); ok {
				closer.Close()
			}
			return
		}

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

func (ex *Exporter) closeTemplatePath() {
	if ex.record != nil {
		ex.record.IsLast = true
		ex.executeTemplate(ex.record)
	} else if ex.rownum == 0 {
		executeRecord := &Record{
			Num:        0,
			IsFirst:    true,
			IsLast:     true,
			IsEmpty:    true,
			columns:    ex.headerNames,
			showRownum: ex.showRownum,
		}
		ex.executeTemplate(executeRecord)
	}

	content := strings.Join(ex.mdLines, "")
	if ex.htmlRender {
		conv := mdconv.New(mdconv.WithDarkMode(false))
		ex.output.Write([]byte("<div>\n"))
		conv.ConvertString(content, ex.output)
		ex.output.Write([]byte("</div>"))
		return
	}
	ex.output.Write([]byte(content))
}

func (ex *Exporter) executeTemplate(record *Record) {
	if ex.tmpl == nil {
		return
	}
	b := &strings.Builder{}
	if err := ex.tmpl.Execute(b, record); err != nil {
		if ex.logger != nil {
			ex.logger.LogError("markdown template execute", err)
		}
		return
	}
	ex.mdLines = append(ex.mdLines, b.String())
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

	if ex.tmpl != nil {
		if ex.record != nil {
			ex.executeTemplate(ex.record)
		}
		for i, val := range values {
			values[i] = api.Unbox(val)
		}
		ex.record = &Record{
			values:     values,
			Num:        int(ex.rownum),
			IsFirst:    ex.rownum == 1,
			IsEmpty:    len(values) == 0,
			columns:    ex.headerNames,
			showRownum: ex.showRownum,
		}
		return nil
	}
	var cols = make([]string, len(values))

	var nullAlt string = "NULL"

	for i, r := range values {
		if r == nil {
			cols[i] = nullAlt
			continue
		}
		switch v := api.Unbox(r).(type) {
		case bool:
			cols[i] = strconv.FormatBool(v)
		case string:
			cols[i] = v
		case time.Time:
			cols[i] = ex.timeformatter.Format(v)
		case float64:
			cols[i] = ex.encodeFloat64(v)
		case float32:
			cols[i] = ex.encodeFloat64(float64(v))
		case []byte:
			cols[i] = ex.binaryFormatter.Format(v)
		case int:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case int8:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case int16:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case uint16:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case int32:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case uint32:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case int64:
			cols[i] = strconv.FormatInt(v, 10)
		case uint64:
			cols[i] = strconv.FormatUint(v, 10)
		case net.IP:
			cols[i] = v.String()
		case api.JSONString:
			cols[i] = string(v)
		default:
			cols[i] = fmt.Sprintf("%#v", v)
		}
	}

	if ex.showRownum {
		cols = append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...)
	}

	line := "|" + strings.Join(cols, "|") + "|\n"
	ex.mdLines = append(ex.mdLines, line)

	return nil
}

type Record struct {
	Num        int
	IsFirst    bool
	IsLast     bool
	IsEmpty    bool
	columns    []string
	values     []any
	showRownum bool
	v          map[string]any
}

func (to *Record) Value(idx int) any {
	if idx < 0 || idx >= len(to.values) {
		return nil
	}
	return to.values[idx]
}

func (to *Record) ValueString(idx int) string {
	return fmt.Sprint(to.Value(idx))
}

func (to *Record) ValueHTML(idx int) htmlTemplate.HTML {
	return htmlTemplate.HTML(to.ValueString(idx))
}

func (to *Record) ValueHTMLAttr(idx int) htmlTemplate.HTMLAttr {
	return htmlTemplate.HTMLAttr(to.ValueString(idx))
}

func (to *Record) ValueCSS(idx int) htmlTemplate.CSS {
	return htmlTemplate.CSS(to.ValueString(idx))
}

func (to *Record) ValueJS(idx int) htmlTemplate.JS {
	return htmlTemplate.JS(to.ValueString(idx))
}

func (to *Record) ValueURL(idx int) htmlTemplate.URL {
	return htmlTemplate.URL(to.ValueString(idx))
}

func (to *Record) Values() []any {
	if !to.showRownum {
		return to.values
	}
	vals := make([]any, 0, len(to.values)+1)
	vals = append(vals, to.Num)
	vals = append(vals, to.values...)
	return vals
}

func (to *Record) V() map[string]any {
	if to.v == nil {
		to.v = make(map[string]any)
		if to.showRownum {
			to.v["ROWNUM"] = to.Num
		}
		for i := 0; i < len(to.values) && i < len(to.columns); i++ {
			to.v[to.columns[i]] = to.values[i]
		}
	}
	return to.v
}

func (to *Record) Columns() []string {
	if to.columns == nil {
		return nil
	}
	if !to.showRownum {
		cols := make([]string, len(to.columns))
		copy(cols, to.columns)
		return cols
	}
	cols := make([]string, 0, len(to.columns)+1)
	cols = append(cols, "ROWNUM")
	cols = append(cols, to.columns...)
	return cols
}

func (to *Record) Column(idx int) string {
	cols := to.Columns()
	if cols == nil || idx < 0 || idx >= len(cols) {
		return ""
	}
	return cols[idx]
}

func templateFuncs() txtTemplate.FuncMap {
	return txtTemplate.FuncMap{
		"timeformat": func(s any, z any, v any) string {
			tz, err := util.ParseTimeLocation(fmt.Sprint(z), time.Local)
			if err != nil {
				return fmt.Sprintf("Invalid timezone: %v", err)
			}
			ts, err := util.ToTime(v)
			if err != nil {
				return fmt.Sprintf("Invalid time: %v", err)
			}
			tf := util.NewTimeFormatter(util.Timeformat(fmt.Sprint(s)), util.TimeLocation(tz))
			return tf.Format(ts)
		},
		"format": func(format string, v any) string {
			return fmt.Sprintf(format, v)
		},
		"toUpper": func(s string) string {
			return strings.ToUpper(s)
		},
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
	}
}
