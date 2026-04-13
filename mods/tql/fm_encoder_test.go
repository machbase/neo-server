package tql

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/stretchr/testify/require"
)

type templateRecorder struct {
	templates []string
}

func (r *templateRecorder) SetTemplate(templates ...string) {
	r.templates = templates
}

type timeLocationRecorder struct {
	tz *time.Location
}

func (r *timeLocationRecorder) SetTimeLocation(tz *time.Location) {
	r.tz = tz
}

type markAreaRecorder struct {
	from    any
	to      any
	label   string
	color   string
	opacity float64
}

func (r *markAreaRecorder) SetMarkAreaNameCoord(from any, to any, label string, color string, opacity float64) {
	r.from = from
	r.to = to
	r.label = label
	r.color = color
	r.opacity = opacity
}

type captureReceiver struct {
	last *Record
}

func (r *captureReceiver) Name() string { return "capture" }
func (r *captureReceiver) Receive(rec *Record) {
	r.last = rec
}

func TestNewEncoder(t *testing.T) {
	ret, err := newEncoder("json", time.UTC)
	require.NoError(t, err)
	require.Equal(t, "json", ret.format)
	require.Len(t, ret.opts, 1)
	tzRecorder := &timeLocationRecorder{}
	ret.opts[0](tzRecorder)
	require.Equal(t, time.UTC, tzRecorder.tz)

	cache := &CacheParam{}
	ret, err = newEncoder("json", cache)
	require.NoError(t, err)
	require.Same(t, cache, ret.cacheOption)

	_, err = newEncoder("markdown", cache)
	require.EqualError(t, err, "encoder 'markdown' does not support cache")

	_, err = newEncoder("json", "bad-option")
	require.EqualError(t, err, "encoder 'json' invalid option bad-option (string)")
}

func TestTemplateOption(t *testing.T) {
	opt, err := toTemplateOption("hello")
	require.NoError(t, err)
	require.NotNil(t, opt)
	recorder := &templateRecorder{}
	opt(recorder)
	require.Equal(t, []string{"hello"}, recorder.templates)

	dir := t.TempDir()
	path := filepath.Join(dir, "template.txt")
	require.NoError(t, os.WriteFile(path, []byte("file-template"), 0o600))
	opt, err = toTemplateOption(&FilePath{Path: "/template.txt", AbsPath: path})
	require.NoError(t, err)
	recorder = &templateRecorder{}
	opt(recorder)
	require.Equal(t, []string{"file-template"}, recorder.templates)

	_, err = toTemplateOption(&FilePath{Path: "/missing.txt"})
	require.EqualError(t, err, "template file '/missing.txt' not found")

	_, err = toTemplateOption(&FilePath{Path: "/missing.txt", AbsPath: filepath.Join(dir, "missing.txt")})
	require.ErrorContains(t, err, "template file '/missing.txt' not found")

	opt, err = toTemplateOption(123)
	require.NoError(t, err)
	require.Nil(t, opt)
}

func TestEncoderNodeFunctions(t *testing.T) {
	node := NewNode(NewTask())
	enc, err := node.fmChart()
	require.NoError(t, err)
	require.Equal(t, "echart", enc.format)

	enc, err = node.fmChartLine()
	require.NoError(t, err)
	require.Equal(t, "echart.line", enc.format)

	enc, err = node.fmChartScatter()
	require.NoError(t, err)
	require.Equal(t, "echart.scatter", enc.format)

	enc, err = node.fmChartBar()
	require.NoError(t, err)
	require.Equal(t, "echart.bar", enc.format)

	enc, err = node.fmChartLine3D()
	require.NoError(t, err)
	require.Equal(t, "echart.line3d", enc.format)

	enc, err = node.fmChartBar3D()
	require.NoError(t, err)
	require.Equal(t, "echart.bar3d", enc.format)

	enc, err = node.fmChartSurface3D()
	require.NoError(t, err)
	require.Equal(t, "echart.surface3d", enc.format)

	enc, err = node.fmChartScatter3D()
	require.NoError(t, err)
	require.Equal(t, "echart.scatter3d", enc.format)

	enc, err = node.fmDiscard()
	require.NoError(t, err)
	require.Equal(t, "discard", enc.format)

	dir := t.TempDir()
	path := filepath.Join(dir, "template.html")
	require.NoError(t, os.WriteFile(path, []byte("<p>{{ . }}</p>"), 0o600))
	enc, err = node.fmHtml(&FilePath{Path: "/template.html", AbsPath: path})
	require.NoError(t, err)
	require.Equal(t, "html", enc.format)
	require.Len(t, enc.opts, 1)

	enc, err = node.fmText("plain")
	require.NoError(t, err)
	require.Equal(t, "text", enc.format)
	require.Len(t, enc.opts, 1)
	tr := &templateRecorder{}
	enc.opts[0](tr)
	require.Equal(t, []string{"plain"}, tr.templates)

	_, err = node.fmHtml(&FilePath{Path: "/missing.html"})
	require.EqualError(t, err, "template file '/missing.html' not found")
}

func TestMarkAreaAndHttp(t *testing.T) {
	node := NewNode(NewTask())
	_, err := node.fmMarkArea("x1")
	require.EqualError(t, err, "f(markArea) invalid number of args; expect:2, actual:1")

	ret, err := node.fmMarkArea("x1", "x2", "label", "red", 0.25)
	require.NoError(t, err)
	option, ok := ret.(opts.Option)
	require.True(t, ok)
	area := &markAreaRecorder{}
	option(area)
	require.Equal(t, "x1", area.from)
	require.Equal(t, "x2", area.to)
	require.Equal(t, "label", area.label)
	require.Equal(t, "red", area.color)
	require.Equal(t, 0.25, area.opacity)

	_, err = node.fmMarkArea("x1", "x2", "label", "red", "bad")
	require.EqualError(t, err, "f(markArea) arg(4) should be opacity, but string")

	_, err = node.fmHttp()
	require.EqualError(t, err, "f(HTTP) invalid number of args; expect:1, actual:0")
	_, err = node.fmHttp([]int{10})
	require.EqualError(t, err, "f(HTTP) arg(0) should be content, but []int")
	_, err = node.fmHttp("GET http://example.com\ninvalid-header")
	require.ErrorContains(t, err, "HTTP parse error")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	node.next = &captureReceiver{}
	_, err = node.fmHttp("GET " + ts.URL)
	require.NoError(t, err)
	require.NotNil(t, node.next.(*captureReceiver).last)
}
