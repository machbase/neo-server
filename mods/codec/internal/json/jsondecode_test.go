package json_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/json"
	"github.com/machbase/neo-server/mods/stream"
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

	dec := &json.Decoder{}
	dec.SetTable("test")
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumns([]string{"NAME", "TIME", "VALUE"}, []string{"string", "datetime", "double"})
	dec.Open()

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

	dec := &json.Decoder{}
	dec.SetTable("test")
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumns([]string{"NAME", "TIME", "VALUE"}, []string{"string", "datetime", "double"})
	dec.Open()

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

	dec := &json.Decoder{}
	dec.SetTable("test")
	dec.SetColumns([]string{"NAME", "TIME", "VALUE"}, []string{"string", "datetime", "double"})
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.Open()

	rec, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	_, err = dec.NextRow()
	require.Equal(t, io.EOF, err)
}
