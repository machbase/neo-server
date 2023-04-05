package json_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/json"
	"github.com/machbase/neo-server/mods/stream"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

func TestDecoder(t *testing.T) {
	input := bytes.NewBufferString(`{
		"table":"sometable", 
		"data":{ 
			"rows": [
				["name1", 1676528839, 0.1234],
				["name2", 1676528840, 0.2345]
		    ],
			"columns": ["name", "time", "value"]
		},
		"other": 999
	}`)

	decoderCtx := &spi.RowsDecoderContext{
		TableName: "test",
		Columns: []*spi.Column{
			{Name: "NAME", Type: "string", Size: 100, Length: 100},
			{Name: "TIME", Type: "datetime", Size: 8, Length: 8},
			{Name: "VALUE", Type: "double", Size: 8, Length: 8},
		},
		TimeFormat: "s",
		Reader:     &stream.ReaderInputStream{Reader: input},
	}
	dec := json.NewDecoder(decoderCtx)

	rec, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	rec, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(1676528840, 0), rec[1])
	require.Equal(t, 0.2345, rec[2])

	_, err = dec.NextRow()
	require.Equal(t, io.EOF, err)
}

func TestRowsOnlyDecoder(t *testing.T) {
	input := bytes.NewBufferString(`[
			["name1", 1676528839, 0.1234],
			["name2", 1676528840, 0.2345]
	]`)

	decoderCtx := &spi.RowsDecoderContext{
		TableName: "test",
		Columns: []*spi.Column{
			{Name: "NAME", Type: "string", Size: 100, Length: 100},
			{Name: "TIME", Type: "datetime", Size: 8, Length: 8},
			{Name: "VALUE", Type: "double", Size: 8, Length: 8},
		},
		TimeFormat: "s",
		Reader:     &stream.ReaderInputStream{Reader: input},
	}
	dec := json.NewDecoder(decoderCtx)

	rec, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	rec, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(1676528840, 0), rec[1])
	require.Equal(t, 0.2345, rec[2])

	_, err = dec.NextRow()
	require.Equal(t, io.EOF, err)
}

func TestSingleRowDecoder(t *testing.T) {
	input := bytes.NewBufferString(`["name1", 1676528839, 0.1234]`)

	decoderCtx := &spi.RowsDecoderContext{
		TableName: "test",
		Columns: []*spi.Column{
			{Name: "NAME", Type: "string", Size: 100, Length: 100},
			{Name: "TIME", Type: "datetime", Size: 8, Length: 8},
			{Name: "VALUE", Type: "double", Size: 8, Length: 8},
		},
		TimeFormat: "s",
		Reader:     &stream.ReaderInputStream{Reader: input},
	}
	dec := json.NewDecoder(decoderCtx)

	rec, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	_, err = dec.NextRow()
	require.Equal(t, io.EOF, err)
}
