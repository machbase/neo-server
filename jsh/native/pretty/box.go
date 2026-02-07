package pretty

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/jedib0t/go-pretty/v6/table"
)

type TableOption struct {
	BoxStyle     string `json:"boxStyle"`
	Timeformat   string `json:"timeformat"`
	Tz           string `json:"tz"`
	Precision    int    `json:"precision"`
	Format       string `json:"format"`
	Header       bool   `json:"header"`
	Footer       bool   `json:"footer"`
	Pause        bool   `json:"pause"`
	Rownum       bool   `json:"rownum"`
	NullValue    string `json:"nullValue"`
	StringEscape bool   `json:"stringEscape"`
}

type TableWriter struct {
	table.Writer
	format       string
	timeformat   string
	tz           *time.Location
	precision    int
	headerRow    table.Row
	header       bool
	footer       bool
	pause        bool
	stringEscape bool
	rownum       bool
	rowCount     int64
	nullValue    string
	rawRows      []table.Row // to store raw rows for JSON, NDJSON rendering
	columnTypes  []string    // to store column types for JSON rendering
	renderCount  int         // count of render calls

	output               io.Writer
	nextPauseRow         int64
	pageHeight           int
	pageHeightSpaceLines int
	termSize             TermSize
}

func Table(opt TableOption) (table.Writer, error) {
	ret := &TableWriter{
		Writer:       table.NewWriter(),
		tz:           time.Local,
		precision:    opt.Precision,
		format:       strings.ToUpper(opt.Format),
		header:       opt.Header,
		footer:       opt.Footer,
		pause:        opt.Pause,
		rownum:       opt.Rownum,
		nullValue:    opt.NullValue,
		stringEscape: opt.StringEscape,
	}
	ret.SetBoxStyle(opt.BoxStyle)
	ret.SetFormat(opt.Format)
	ret.SetTimeformat(opt.Timeformat)
	if err := ret.SetTz(opt.Tz); err != nil {
		return nil, err
	}
	ret.SetAutoIndex(false)

	// initialize terminal size and page height
	if ret.pause && IsTerminal() {
		if ts, err := GetTerminalSize(); err == nil {
			ret.termSize = ts
			// set next pause row threshold
			ret.pageHeight = ts.Height - 1 // leave one line for prompt
			if ret.header {
				ret.pageHeight -= ret.pageHeightSpaceLines // leave lines for header
			}
			ret.nextPauseRow = int64(ret.pageHeight)
		} else {
			ret.pause = false
		}
	}
	return ret, nil
}

type BoxStyle struct {
	style          table.Style
	pageSpaceLines int
	option         func(*table.Style)
}

var boxStyles = map[string]BoxStyle{
	// Basic styles
	"LIGHT":   {table.StyleLight, 4, nil},
	"DOUBLE":  {table.StyleDouble, 4, nil},
	"BOLD":    {table.StyleBold, 4, nil},
	"ROUNDED": {table.StyleRounded, 4, nil},
	"ROUND":   {table.StyleRounded, 4, nil},
	"SIMPLE":  {table.StyleDefault, 4, nil},
	// Bright color styles
	"BRIGHT":         {table.StyleColoredBright, 1, nil},
	"BRIGHT_BLUE":    {table.StyleColoredBlackOnBlueWhite, 1, nil},
	"BRIGHT_CYAN":    {table.StyleColoredBlackOnCyanWhite, 1, nil},
	"BRIGHT_GREEN":   {table.StyleColoredBlackOnGreenWhite, 1, nil},
	"BRIGHT_MAGENTA": {table.StyleColoredBlackOnMagentaWhite, 1, nil},
	"BRIGHT_YELLOW":  {table.StyleColoredBlackOnYellowWhite, 1, nil},
	"BRIGHT_RED":     {table.StyleColoredBlackOnRedWhite, 1, nil},
	// Dark color styles
	"DARK":         {table.StyleColoredDark, 1, nil},
	"DARK_BLUE":    {table.StyleColoredBlueWhiteOnBlack, 1, nil},
	"DARK_CYAN":    {table.StyleColoredCyanWhiteOnBlack, 1, nil},
	"DARK_GREEN":   {table.StyleColoredGreenWhiteOnBlack, 1, nil},
	"DARK_MAGENTA": {table.StyleColoredMagentaWhiteOnBlack, 1, nil},
	"DARK_YELLOW":  {table.StyleColoredYellowWhiteOnBlack, 1, nil},
	"DARK_RED":     {table.StyleColoredRedWhiteOnBlack, 1, nil},
	// Compact style
	"COMPACT": {table.StyleLight, 2, func(s *table.Style) {
		s.Options.DrawBorder = false
		s.Options.SeparateColumns = false
	}},
}

func (tw *TableWriter) SetFormat(format string) {
	tw.format = strings.ToUpper(format)
	switch tw.format {
	case "HTML", "JSON":
		// force disable pause for HTML, JSON formats
		tw.pause = false
	case "NDJSON":
		tw.pageHeightSpaceLines = 0
	case "MD", "MARKDOWN":
		tw.pageHeightSpaceLines = 2
	case "CSV", "TSV":
		tw.pageHeightSpaceLines = 1
	}
}

func (tw *TableWriter) SetBoxStyle(style string) {
	if tw.format != "BOX" {
		return
	}
	styleUpper := strings.ToUpper(style)
	if o, ok := boxStyles[styleUpper]; ok {
		s := o.style
		if o.option != nil {
			o.option(&s)
		}
		tw.SetStyle(s)
		tw.pageHeightSpaceLines = o.pageSpaceLines
	} else {
		tw.SetStyle(table.StyleLight)
		tw.pageHeightSpaceLines = 4
	}
}

func (tw *TableWriter) SetTimeformat(format string) {
	switch strings.ToUpper(format) {
	case "DEFAULT", "":
		tw.timeformat = "2006-01-02 15:04:05.999"
	case "DATETIME":
		tw.timeformat = time.DateTime
	case "DATE":
		tw.timeformat = time.DateOnly
	case "TIME":
		tw.timeformat = time.TimeOnly
	case "RFC3339":
		tw.timeformat = time.RFC3339Nano
	case "RFC1123":
		tw.timeformat = time.RFC1123
	case "ANSIC":
		tw.timeformat = time.ANSIC
	case "KITCHEN":
		tw.timeformat = time.Kitchen
	case "STAMP":
		tw.timeformat = time.Stamp
	case "STAMPMILLI":
		tw.timeformat = time.StampMilli
	case "STAMPMICRO":
		tw.timeformat = time.StampMicro
	case "STAMPNANO":
		tw.timeformat = time.StampNano
	default:
		tw.timeformat = format
	}
}

func (tw *TableWriter) SetTz(tz string) error {
	switch strings.ToUpper(tz) {
	case "", "LOCAL":
		tw.tz = time.Local
	case "UTC":
		tw.tz = time.UTC
	default:
		if tz, err := time.LoadLocation(tz); err == nil {
			tw.tz = tz
		} else {
			return err
		}
	}
	return nil
}

func (tw *TableWriter) SetAutoIndex(autoIndex bool) {
	// always disable auto index feature favored over the rownum option
	tw.Writer.SetAutoIndex(false)
}

func (tw *TableWriter) SetOutput(o any) {
	var w io.Writer = io.Discard
	if m := o.(map[string]any); m != nil {
		// o is javascript object
		if writer, ok := m["writer"].(io.Writer); ok {
			w = writer
		} else {
			panic(fmt.Sprintf("SetOutput: invalid writer in object %+v", o))
		}
	} else if writer, ok := o.(io.Writer); ok {
		// o is io.Writer
		w = writer
	} else if file, ok := o.(*os.File); ok {
		// o is *os.File
		w = file
	}
	tw.output = w // be used for ndjson, and json rendering
	tw.Writer.SetOutputMirror(w)
}

func (tw *TableWriter) SetColumnConfigs(configs []table.ColumnConfig) {
	if tw.rownum {
		// insert ROWNUM column config at the beginning
		rc := table.ColumnConfig{
			Name: "ROWNUM",
		}
		configs = append([]table.ColumnConfig{rc}, configs...)
	}
	for i := range configs {
		configs[i].Number = i + 1
	}
	tw.Writer.SetColumnConfigs(configs)
}

func (tw *TableWriter) SetStringEscape(escape bool) {
	tw.stringEscape = escape
}

func (tw *TableWriter) SetPause(pause bool) {
	tw.pause = pause
}

func (tw *TableWriter) AppendHeader(v table.Row, configs ...table.RowConfig) {
	tw.headerRow = v // store header row
	if !tw.header {
		return
	}
	if tw.rownum {
		v = append(table.Row{"ROWNUM"}, v...)
	}
	tw.Writer.AppendHeader(v, configs...)
}

func (tw *TableWriter) SetColumnTypes(colTypes []string) {
	tw.columnTypes = colTypes
}

func (tw *TableWriter) SetCaption(format string, a ...interface{}) {
	if !tw.footer {
		return
	}
	tw.Writer.SetCaption(format, a...)
}

func (tw *TableWriter) Close() string {
	if tw.Writer.Length() > 0 {
		// remaining rows to render
		return tw.Render()
	}
	if tw.renderCount == 0 {
		// no rows rendered yet, render empty table
		return tw.Render()
	}
	return ""
}

func (tw *TableWriter) RequirePageRender() bool {
	if tw.pause {
		return tw.nextPauseRow > 0 && tw.rowCount == tw.nextPauseRow
	} else {
		return tw.rowCount%1000 == 0
	}
}

// PauseAndWait pauses the table rendering and waits for user input.
// Returns false if user pressed 'q' or 'Q' to quit, otherwise returns true.
func (tw *TableWriter) PauseAndWait() bool {
	if !tw.pause {
		tw.ResetRows()    // clear the table rows
		tw.ResetHeaders() // do not render header again
		return true
	}
	// set next pause row threshold
	tw.nextPauseRow += int64(tw.pageHeight)
	// wait for user input
	continued := PauseTerminal()
	// clear the table rows
	tw.ResetRows()
	return continued
}

func (tw *TableWriter) Append(v any, configs ...table.RowConfig) {
	switch v := v.(type) {
	case table.Row:
		tw.AppendRow(v, configs...)
	case []table.Row:
		tw.AppendRows(v, configs...)
	case []interface{}:
		tw.AppendRow(tw.Row(v...), configs...)
	default:
		return
	}
}

func (tw *TableWriter) AppendRows(rows []table.Row, configs ...table.RowConfig) {
	for _, row := range rows {
		tw.AppendRow(row, configs...)
	}
}

func (tw *TableWriter) AppendRow(row table.Row, configs ...table.RowConfig) {
	tw.rowCount++
	if tw.rownum {
		row = append(table.Row{tw.rowCount}, row...)
	}
	tw.rawRows = append(tw.rawRows, row) // store raw row for NDJSON rendering
	tw.Writer.AppendRow(row, configs...)
}

func (tw *TableWriter) ResetRows() {
	tw.rawRows = []table.Row{}
	tw.Writer.ResetRows()
}

func (tw *TableWriter) Row(values ...interface{}) table.Row {
	for i, value := range values {
		if value == nil {
			values[i] = tw.nullValue
			continue
		}
		switch val := value.(type) {
		case time.Time:
			switch tw.timeformat {
			case "ns":
				values[i] = val.In(tw.tz).UnixNano()
			case "us":
				values[i] = val.In(tw.tz).UnixMicro()
			case "ms":
				values[i] = val.In(tw.tz).UnixMilli()
			case "s":
				values[i] = val.In(tw.tz).Unix()
			default:
				values[i] = val.In(tw.tz).Format(tw.timeformat)
			}
		case float32:
			if tw.precision >= 0 {
				factor := math.Pow(10, float64(tw.precision))
				values[i] = float32(math.Round(float64(val)*factor) / factor)
			}
		case float64:
			if tw.precision >= 0 {
				factor := math.Pow(10, float64(tw.precision))
				values[i] = math.Round(val*factor) / factor
			}
		case string:
			if tw.stringEscape {
				var result strings.Builder
				for _, r := range val {
					if unicode.IsPrint(r) {
						result.WriteRune(r)
					} else {
						result.WriteString(fmt.Sprintf("\\u%04x", r))
					}
				}
				values[i] = result.String()
			} else {
				values[i] = val
			}
		default:
			values[i] = value
		}
	}
	tr := table.Row(values)
	return tr
}

func MakeRow(size int) []table.Row {
	rows := make([]table.Row, size)
	return rows
}

func (tw *TableWriter) Render() string {
	defer func() { tw.renderCount++ }()
	switch tw.format {
	case "CSV":
		return tw.Writer.RenderCSV()
	case "HTML":
		return tw.Writer.RenderHTML()
	case "MARKDOWN", "MD":
		return tw.Writer.RenderMarkdown()
	case "TSV":
		return tw.Writer.RenderTSV()
	case "NDJSON":
		return tw.RenderNDJSON()
	case "JSON":
		return tw.RenderJSON()
	default:
		return tw.Writer.Render()
	}
}

func (tw *TableWriter) RenderNDJSON() string {
	var out strings.Builder
	rows := tw.rawRows
	headers := []string{}
	if tw.rownum {
		headers = append(headers, "ROWNUM")
	}
	if len(tw.headerRow) > 0 {
		for _, h := range tw.headerRow {
			headers = append(headers, fmt.Sprint(h))
		}
	} else if len(rows) > 0 {
		for i := 0; i < len(rows[0]); i++ {
			headers = append(headers, fmt.Sprintf("C%d", i+1))
		}
	}
	for _, row := range rows {
		out.WriteRune('{')
		for i, col := range row {
			if i > 0 {
				out.WriteRune(',')
			}
			out.WriteString(fmt.Sprintf("\"%s\":", headers[i]))
			switch v := col.(type) {
			case string:
				out.WriteString(fmt.Sprintf("\"%s\"", v))
			default:
				out.WriteString(fmt.Sprint(v))
			}
		}
		out.WriteRune('}')
		out.WriteRune('\n')
	}
	ret := out.String()
	if tw.output != nil {
		tw.output.Write([]byte(ret))
	}
	return ret
}

func renderRowsJSON(out *strings.Builder, rows []table.Row) {
	for rIdx, row := range rows {
		if rIdx > 0 {
			out.WriteString(",")
		}
		out.WriteString("[")
		for i, col := range row {
			if i > 0 {
				out.WriteRune(',')
			}
			switch v := col.(type) {
			case string:
				out.WriteString(fmt.Sprintf("\"%s\"", v))
			default:
				out.WriteString(fmt.Sprint(v))
			}
		}
		out.WriteString("]")
	}
}

func (tw *TableWriter) RenderJSON() string {
	var out strings.Builder
	rows := tw.rawRows

	if tw.renderCount > 0 {
		renderRowsJSON(&out, rows)
		return out.String()
	}

	headers := []string{}
	types := []string{}

	if tw.rownum {
		headers = append(headers, "ROWNUM")
		types = append(types, "int64")
	}
	if len(tw.headerRow) > 0 {
		for _, h := range tw.headerRow {
			headers = append(headers, fmt.Sprint(h))
		}
	} else if len(rows) > 0 {
		for i := 0; i < len(rows[0]); i++ {
			headers = append(headers, fmt.Sprintf("C%d", i+1))
		}
	}
	for _, colType := range tw.columnTypes {
		types = append(types, colType)
	}
	out.WriteString("{")
	out.WriteString("\"columns\":[")
	for i, h := range headers {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(fmt.Sprintf("\"%s\"", h))
	}
	out.WriteString("],")
	if len(types) == len(headers) {
		out.WriteString("\"types\":[")
		for i, ct := range types {
			if i > 0 {
				out.WriteString(",")
			}
			out.WriteString(fmt.Sprintf("\"%s\"", ct))
		}
		out.WriteString("],")
	}
	out.WriteString("\"rows\":[")
	renderRowsJSON(&out, rows)
	out.WriteString("]")
	out.WriteString("}\n")
	ret := out.String()
	if tw.output != nil {
		tw.output.Write([]byte(ret))
	}
	return ret
}
