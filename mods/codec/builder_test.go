package codec

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/stretchr/testify/require"
)

type captureEncoder struct {
	colNames []string
	colTypes []api.DataType
}

func (c *captureEncoder) Open() error                      { return nil }
func (c *captureEncoder) Close()                           {}
func (c *captureEncoder) AddRow(values []any) error        { return nil }
func (c *captureEncoder) Flush(heading bool)               {}
func (c *captureEncoder) ContentType() string              { return "text/plain" }
func (c *captureEncoder) HttpHeaders() map[string][]string { return nil }
func (c *captureEncoder) SetColumns(names ...string)       { c.colNames = append([]string{}, names...) }
func (c *captureEncoder) SetColumnTypes(types ...api.DataType) {
	c.colTypes = append([]api.DataType{}, types...)
}

func TestNewEncoderAndDiscardSink(t *testing.T) {
	buf := &bytes.Buffer{}
	types := []string{
		BOX, CSV, JSON, NDJSON, MARKDOWN, HTML, TEXT,
		ECHART, ECHART_LINE, ECHART_SCATTER, ECHART_BAR,
		ECHART_LINE3D, ECHART_SURFACE3D, ECHART_SCATTER3D, ECHART_BAR3D,
		GEOMAP, DISCARD, "unknown",
	}
	for _, typ := range types {
		enc := NewEncoder(typ, opts.OutputStream(buf))
		require.NotNil(t, enc)
	}

	ds, ok := NewEncoder(DISCARD).(*DiscardSink)
	require.True(t, ok)
	require.NoError(t, ds.Open())
	require.NoError(t, ds.AddRow([]any{"x"}))
	ds.Flush(true)
	ds.Close()
	require.Equal(t, "text/plain", ds.ContentType())
	require.Nil(t, ds.HttpHeaders())
}

func TestNewDecoderReturnsExpectedDecoder(t *testing.T) {
	for _, typ := range []string{CSV, NDJSON, JSON, "unknown"} {
		dec := NewDecoder(typ, opts.InputStream(bytes.NewBufferString("{}")), opts.TableName("example"))
		require.NotNil(t, dec)
	}
}

func TestSetEncoderColumnsTimeLocation(t *testing.T) {
	enc := &captureEncoder{}
	cols := api.Columns{
		{Name: "name", DataType: api.DataTypeString},
		{Name: "ts", DataType: api.DataTypeDatetime},
	}

	SetEncoderColumns(enc, cols)
	require.Equal(t, []string{"name", "ts"}, enc.colNames)
	require.Equal(t, []api.DataType{api.DataTypeString, api.DataTypeDatetime}, enc.colTypes)

	SetEncoderColumnsTimeLocation(enc, cols, time.UTC)
	require.Equal(t, []string{"name", "ts(UTC)"}, enc.colNames)
	require.Equal(t, []api.DataType{api.DataTypeString, api.DataTypeDatetime}, enc.colTypes)
}
