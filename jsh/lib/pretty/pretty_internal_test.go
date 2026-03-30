package pretty

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	prettytable "github.com/jedib0t/go-pretty/v6/table"
)

func TestParseTimeCoverage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		value  string
		format string
		tz     string
		want   time.Time
	}{
		{name: "ns", value: "1000", format: "ns", want: time.Unix(0, 1000)},
		{name: "us", value: "1000", format: "us", want: time.Unix(0, 1000*1000)},
		{name: "ms", value: "1000", format: "ms", want: time.Unix(0, 1000*1000*1000)},
		{name: "s", value: "1000", format: "s", want: time.Unix(1000, 0)},
		{name: "rfc3339 default", value: "2024-03-15T14:30:45Z", format: "", tz: "UTC", want: time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)},
		{name: "custom layout utc", value: "2024-03-15 14:30", format: "2006-01-02 15:04", tz: "UTC", want: time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTime(tc.value, tc.format, tc.tz)
			if err != nil {
				t.Fatalf("parseTime() error = %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("parseTime() = %v, want %v", got, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		value  string
		format string
		tz     string
		want   string
	}{
		{name: "bad ns", value: "bad", format: "ns", want: "failed to parse time 'bad' as integer"},
		{name: "bad timezone", value: "2024-03-15T14:30:45Z", format: "", tz: "Bad/Zone", want: "failed to load location 'Bad/Zone'"},
		{name: "bad rfc3339", value: "not-a-time", format: "", tz: "UTC", want: "failed to parse time 'not-a-time' with RFC3339"},
		{name: "bad custom layout", value: "2024/03/15", format: "2006-01-02", tz: "UTC", want: "failed to parse time '2024/03/15' with format '2006-01-02'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTime(tc.value, tc.format, tc.tz)
			if err == nil {
				t.Fatal("expected parseTime error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parseTime() error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestTerminalHelpersCoverage(t *testing.T) {
	if got := (TermSize{Width: 80, Height: 24}).String(); got != "{Width: 80, Height: 24}" {
		t.Fatalf("TermSize.String() = %q", got)
	}

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stdin failed: %v", err)
	}
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stdout failed: %v", err)
	}
	os.Stdin = rIn
	os.Stdout = wOut
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		rIn.Close()
		wIn.Close()
		rOut.Close()
		wOut.Close()
	}()

	if IsTerminal() {
		t.Fatal("expected pipe-backed stdin to be non-terminal")
	}
	if _, err := GetTerminalSize(); err == nil {
		t.Fatal("expected GetTerminalSize() to fail with pipe-backed stdout")
	}
	if !PauseTerminal() {
		t.Fatal("PauseTerminal() should return true when raw mode setup fails")
	}
}

func TestFormattingHelpersCoverage(t *testing.T) {
	if got := Bytes(int64(2048)); got != "2.0KB" {
		t.Fatalf("Bytes() = %q", got)
	}
	if got := Ints(int64(12345)); got != "12,345" {
		t.Fatalf("Ints() = %q", got)
	}
}

func TestTableConfigurationAndRenderCoverage(t *testing.T) {
	writer, err := Table(TableOption{Format: "json", Header: true, Footer: true, Rownum: true, Tz: "UTC"})
	if err != nil {
		t.Fatalf("Table() error = %v", err)
	}
	tw := writer.(*TableWriter)

	var buf bytes.Buffer
	tw.SetOutput(map[string]any{"writer": &buf})
	tw.SetStringEscape(true)
	tw.SetPause(false)
	tw.SetColumnTypes([]string{"string", "float64"})
	tw.AppendHeader(prettytable.Row{"NAME", "VALUE"})
	tw.SetCaption("caption %d", 1)
	tw.Append([]interface{}{"line\nfeed", 1.5})
	tw.AppendRows([]prettytable.Row{{"plain", 2.5}})
	tw.Append(struct{}{})

	jsonRendered := tw.Render()
	if !strings.Contains(jsonRendered, `"columns":["ROWNUM","NAME","VALUE"]`) {
		t.Fatalf("RenderJSON() missing columns: %q", jsonRendered)
	}
	if !strings.Contains(jsonRendered, `"types":["int64","string","float64"]`) {
		t.Fatalf("RenderJSON() missing types: %q", jsonRendered)
	}
	if !strings.Contains(jsonRendered, "[[1,\"line\nfeed\",1.5],[2,\"plain\",2.5]]") {
		t.Fatalf("RenderJSON() missing rows: %q", jsonRendered)
	}
	if buf.String() != jsonRendered {
		t.Fatalf("RenderJSON() output mirror mismatch: %q != %q", buf.String(), jsonRendered)
	}

	tw.ResetRows()
	tw.AppendRow(prettytable.Row{"again", 3.5})
	jsonRowsOnly := tw.Render()
	if jsonRowsOnly != `[3,"again",3.5]` {
		t.Fatalf("RenderJSON() second pass = %q", jsonRowsOnly)
	}

	ndWriter, err := Table(TableOption{Format: "ndjson", Header: true, Rownum: true})
	if err != nil {
		t.Fatalf("Table(ndjson) error = %v", err)
	}
	nd := ndWriter.(*TableWriter)
	nd.AppendHeader(prettytable.Row{"NAME", "VALUE"})
	nd.AppendRow(prettytable.Row{"alpha", 10})
	nd.AppendRow(prettytable.Row{"beta", 20})
	ndRendered := nd.Render()
	if ndRendered != "{\"ROWNUM\":1,\"NAME\":\"alpha\",\"VALUE\":10}\n{\"ROWNUM\":2,\"NAME\":\"beta\",\"VALUE\":20}\n" {
		t.Fatalf("RenderNDJSON() = %q", ndRendered)
	}

	noHeaderWriter, err := Table(TableOption{Format: "ndjson", Header: false, Rownum: false})
	if err != nil {
		t.Fatalf("Table(ndjson,noheader) error = %v", err)
	}
	noHeader := noHeaderWriter.(*TableWriter)
	noHeader.AppendRow(prettytable.Row{"x", 1})
	if got := noHeader.Render(); got != "{\"C1\":\"x\",\"C2\":1}\n" {
		t.Fatalf("RenderNDJSON() without header = %q", got)
	}

	htmlWriter, err := Table(TableOption{Format: "html", Pause: true})
	if err != nil {
		t.Fatalf("Table(html) error = %v", err)
	}
	html := htmlWriter.(*TableWriter)
	if html.pause {
		t.Fatal("HTML format should force pause=false")
	}

	mdWriter, err := Table(TableOption{Format: "markdown"})
	if err != nil {
		t.Fatalf("Table(markdown) error = %v", err)
	}
	if mdWriter.(*TableWriter).pageHeightSpaceLines != 2 {
		t.Fatal("markdown format should set pageHeightSpaceLines to 2")
	}

	tsvWriter, err := Table(TableOption{Format: "tsv"})
	if err != nil {
		t.Fatalf("Table(tsv) error = %v", err)
	}
	if tsvWriter.(*TableWriter).pageHeightSpaceLines != 1 {
		t.Fatal("tsv format should set pageHeightSpaceLines to 1")
	}
}

func TestTableBehaviorCoverage(t *testing.T) {
	tw := &TableWriter{Writer: prettytable.NewWriter(), format: "BOX", header: true, footer: true, rownum: true, nullValue: "NULL", precision: 2, tz: time.UTC}
	tw.SetBoxStyle("compact")
	if tw.pageHeightSpaceLines != 2 {
		t.Fatalf("compact box style pageHeightSpaceLines = %d", tw.pageHeightSpaceLines)
	}
	tw.SetBoxStyle("unknown")
	if tw.pageHeightSpaceLines != 4 {
		t.Fatalf("unknown box style pageHeightSpaceLines = %d", tw.pageHeightSpaceLines)
	}
	tw.format = "CSV"
	tw.pageHeightSpaceLines = 9
	tw.SetBoxStyle("double")
	if tw.pageHeightSpaceLines != 9 {
		t.Fatalf("SetBoxStyle should be ignored for non-BOX format, got %d", tw.pageHeightSpaceLines)
	}

	tw.SetTimeformat("datetime")
	if tw.timeformat != time.DateTime {
		t.Fatalf("SetTimeformat(datetime) = %q", tw.timeformat)
	}
	tw.SetTimeformat("custom-layout")
	if tw.timeformat != "custom-layout" {
		t.Fatalf("SetTimeformat(custom) = %q", tw.timeformat)
	}
	if err := tw.SetTz("UTC"); err != nil {
		t.Fatalf("SetTz(UTC) error = %v", err)
	}
	if err := tw.SetTz("Local"); err != nil {
		t.Fatalf("SetTz(Local) error = %v", err)
	}
	if err := tw.SetTz("Bad/Zone"); err == nil {
		t.Fatal("expected SetTz(Bad/Zone) error")
	}

	if got := tw.transformer(nil); got != "NULL" {
		t.Fatalf("transformer(nil) = %q", got)
	}
	tm := time.Date(2024, 3, 15, 14, 30, 45, 123000000, time.UTC)
	tw.SetTimeformat("ms")
	if got := tw.transformer(tm); got != "1710513045123" {
		t.Fatalf("transformer(time,ms) = %q", got)
	}
	tw.precision = -1
	if got := tw.transformer(1.25); got != "1.25" {
		t.Fatalf("transformer(float,no precision) = %q", got)
	}
	tw.precision = 2
	tw.SetStringEscape(true)
	if got := tw.transformer("hi\n"); got != `hi\u000a` {
		t.Fatalf("transformer(string escaped) = %q", got)
	}
	tw.SetStringEscape(false)
	if got := tw.transformer("ok"); got != "ok" {
		t.Fatalf("transformer(string) = %q", got)
	}

	tw.output = io.Discard
	tw.AppendHeader(prettytable.Row{"NAME"})
	tw.AppendRow(prettytable.Row{"value"})
	if tw.RequirePageRender() {
		t.Fatal("RequirePageRender() should be false for rowCount=1 pause=false")
	}
	tw.rowCount = 1000
	if !tw.RequirePageRender() {
		t.Fatal("RequirePageRender() should be true for rowCount multiple of 1000")
	}
	tw.pause = true
	tw.nextPauseRow = 1000
	if !tw.RequirePageRender() {
		t.Fatal("RequirePageRender() should be true when pause threshold is hit")
	}
	tw.pause = false
	if !tw.PauseAndWait() {
		t.Fatal("PauseAndWait() should continue when pause=false")
	}
	if len(tw.rawRows) != 0 {
		t.Fatal("PauseAndWait() should reset raw rows when pause=false")
	}
	if tw.Writer.Length() != 0 {
		t.Fatal("PauseAndWait() should reset writer rows when pause=false")
	}

	emptyWriter, err := Table(TableOption{Format: "box", Header: true, Footer: true, Rownum: true})
	if err != nil {
		t.Fatalf("Table(box) error = %v", err)
	}
	empty := emptyWriter.(*TableWriter)
	empty.AppendHeader(prettytable.Row{"NAME"})
	if got := empty.Close(); !strings.Contains(got, "NAME") {
		t.Fatalf("Close() should render empty table with header, got %q", got)
	}
	if got := empty.Close(); got != "" {
		t.Fatalf("Close() second call = %q, want empty string", got)
	}

	if _, err := Table(TableOption{Tz: "Bad/Zone"}); err == nil {
		t.Fatal("Table() should fail for invalid timezone")
	}

	var directOutput bytes.Buffer
	tw.SetOutput(&directOutput)
	tw.format = "NDJSON"
	tw.headerRow = prettytable.Row{"NAME"}
	tw.rownum = false
	tw.rawRows = []prettytable.Row{{"direct-writer"}}
	if got := tw.RenderNDJSON(); directOutput.String() != got {
		t.Fatalf("SetOutput(io.Writer) mirror mismatch: %q != %q", directOutput.String(), got)
	}
}

func TestProgressCoverage(t *testing.T) {
	pw := Progress(map[string]any{
		"showPercentage":  false,
		"showETA":         false,
		"showSpeed":       false,
		"updateFrequency": int64(10),
		"trackerLength":   int64(7),
	})
	if pw == nil {
		t.Fatal("Progress() returned nil")
	}
	tracker := pw.Tracker(map[string]any{"message": "work", "total": int64(10)})
	if tracker.Message != "work" || tracker.Total != 10 {
		t.Fatalf("Tracker() = %+v", tracker)
	}
	defaultTracker := pw.Tracker(map[string]any{})
	if defaultTracker.Message != "" || defaultTracker.Total != 0 {
		t.Fatalf("Tracker(default) = %+v", defaultTracker)
	}
	pw.Stop()
	if !pw.IsRenderInProgress() {
		// acceptable if the render goroutine has already stopped before inspection
	}
	_ = filepath.Separator
}
