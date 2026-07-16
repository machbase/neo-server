package markdown

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/stretchr/testify/require"
)

type spyLogger struct {
	errorCount int
}

func (l *spyLogger) Logf(format string, args ...any)      {}
func (l *spyLogger) Log(args ...any)                      {}
func (l *spyLogger) LogDebugf(format string, args ...any) {}
func (l *spyLogger) LogDebug(args ...any)                 {}
func (l *spyLogger) LogWarnf(format string, args ...any)  {}
func (l *spyLogger) LogWarn(args ...any)                  {}
func (l *spyLogger) LogErrorf(format string, args ...any) {}
func (l *spyLogger) LogError(args ...any)                 { l.errorCount++ }

type flushCloseBuffer struct {
	bytes.Buffer
	flushCount int
	closeCount int
}

func (b *flushCloseBuffer) Flush() error {
	b.flushCount++
	return nil
}

func (b *flushCloseBuffer) Close() error {
	b.closeCount++
	return nil
}

func TestExporterOpenErrorPaths(t *testing.T) {
	ex := NewEncoder()
	err := ex.Open()
	require.EqualError(t, err, "no output is assigned")

	ex.SetOutputStream(&bytes.Buffer{})
	ex.SetTemplate("{{")
	err = ex.Open()
	require.Error(t, err)
}

func TestExporterFlushAndCloseOnce(t *testing.T) {
	out := &flushCloseBuffer{}
	ex := NewEncoder()
	ex.SetOutputStream(out)
	ex.SetColumns("v")
	require.NoError(t, ex.Open())
	ex.Flush(false)
	ex.Close()
	ex.Close()

	require.Equal(t, 1, out.flushCount)
	require.Equal(t, 1, out.closeCount)
}

func TestExporterSetBriefAndPrecision(t *testing.T) {
	out := &bytes.Buffer{}
	ex := NewEncoder()
	ex.SetOutputStream(out)
	ex.SetColumns("f")
	ex.SetPrecision(2)
	ex.SetBrief(true)
	require.NoError(t, ex.Open())

	for i := 0; i < 7; i++ {
		require.NoError(t, ex.AddRow([]any{1.2345}))
	}
	ex.Close()

	result := out.String()
	require.Contains(t, result, "|1.23|")
	require.Contains(t, result, "Total")
}

func TestExporterTemplateNoRowsAndTemplateExecuteError(t *testing.T) {
	t.Run("no rows template path", func(t *testing.T) {
		out := &bytes.Buffer{}
		ex := NewEncoder()
		ex.SetOutputStream(out)
		ex.SetColumns("name")
		ex.SetTemplate(`{{if .IsEmpty}}EMPTY{{end}}`)
		require.NoError(t, ex.Open())
		ex.Close()
		require.Equal(t, "EMPTY", out.String())
	})

	t.Run("template execute error logs", func(t *testing.T) {
		out := &bytes.Buffer{}
		logger := &spyLogger{}
		ex := NewEncoder()
		ex.SetOutputStream(out)
		ex.SetLogger(logger)
		ex.SetTemplate(`{{index .Values 3}}`)
		require.NoError(t, ex.Open())
		require.NoError(t, ex.AddRow([]any{"only-one"}))
		ex.Close()
		require.Greater(t, logger.errorCount, 0)
	})
}

func TestExporterAdditionalAddRowTypes(t *testing.T) {
	out := &bytes.Buffer{}
	ex := NewEncoder()
	ex.SetOutputStream(out)
	ex.SetColumns("u16", "u32", "u64", "json")
	require.NoError(t, ex.Open())

	require.NoError(t, ex.AddRow([]any{uint16(16), uint32(32), uint64(64), api.JSONString(`{"k":1}`)}))
	ex.Close()

	result := out.String()
	require.Contains(t, result, "|16|32|64|{\"k\":1}|")
}

func TestRecordHelpers(t *testing.T) {
	rec := &Record{
		Num:        7,
		columns:    []string{"c1", "c2"},
		values:     []any{"x", 10},
		showRownum: true,
	}

	require.Equal(t, "x", rec.Value(0))
	require.Nil(t, rec.Value(-1))
	require.Nil(t, rec.Value(9))
	require.Equal(t, "10", rec.ValueString(1))
	require.Equal(t, "x", string(rec.ValueHTML(0)))
	require.Equal(t, "x", string(rec.ValueHTMLAttr(0)))
	require.Equal(t, "x", string(rec.ValueCSS(0)))
	require.Equal(t, "x", string(rec.ValueJS(0)))
	require.Equal(t, "x", string(rec.ValueURL(0)))
	require.Equal(t, []any{7, "x", 10}, rec.Values())
	require.Equal(t, []string{"ROWNUM", "c1", "c2"}, rec.Columns())
	require.Equal(t, "ROWNUM", rec.Column(0))
	require.Equal(t, "", rec.Column(10))

	m := rec.V()
	require.Equal(t, 7, m["ROWNUM"])
	require.Equal(t, "x", m["c1"])
	require.Equal(t, 10, m["c2"])

	recNoRow := &Record{columns: []string{"a"}, values: []any{1}, showRownum: false}
	require.Equal(t, []any{1}, recNoRow.Values())
	require.Equal(t, []string{"a"}, recNoRow.Columns())
}

func TestTemplateFuncs(t *testing.T) {
	fm := templateFuncs()

	toUpper, ok := fm["toUpper"].(func(string) string)
	require.True(t, ok)
	require.Equal(t, "ABC", toUpper("Abc"))

	toLower, ok := fm["toLower"].(func(string) string)
	require.True(t, ok)
	require.Equal(t, "abc", toLower("AbC"))

	formatFn, ok := fm["format"].(func(string, any) string)
	require.True(t, ok)
	require.Equal(t, "v=3", formatFn("v=%d", 3))

	timeformatFn, ok := fm["timeformat"].(func(any, any, any) string)
	require.True(t, ok)

	require.Contains(t, timeformatFn("2006", "Invalid/TZ", time.Now()), "Invalid timezone")
	require.Contains(t, timeformatFn("2006", "UTC", errors.New("not time")), "Invalid time")
	valid := timeformatFn("2006-01-02", "UTC", time.Date(2023, 8, 22, 0, 0, 0, 0, time.UTC))
	require.Equal(t, "2023-08-22", strings.TrimSpace(valid))
}
