package json_test

import (
	"bytes"
	"io"
	"strings"
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
	dec.SetTableName("test")
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumnTypes("string", "datetime", "double")
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

func TestDecoderNano(t *testing.T) {
	input := bytes.NewBufferString(`{
		"table":"sometable", 
		"data":{ 
			"rows": [
				["name1", 1676432363333444555, 0.1234],
				["name2", 1676432364666777888, 0.2345]
		    ],
			"columns": ["name", "time", "value"]
		},
		"other": 999
	}`)

	dec := &json.Decoder{}
	dec.SetTableName("test")
	dec.SetTimeformat("ns")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumnTypes("string", "datetime", "double")
	dec.Open()

	rec, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(0, 1676432363333444555), rec[1])
	require.Equal(t, 0.1234, rec[2])

	rec, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(0, 1676432364666777888), rec[1])
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
	dec.SetTableName("test")
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumnTypes("string", "datetime", "double")
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
	dec.SetTableName("test")
	dec.SetColumnTypes("string", "datetime", "double")
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

func TestEncoder(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double")
	enc.SetColumns("name", "time", "value")
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234})
	enc.AddRow([]any{"name1", 1676432364666777888, 0.2345})
	enc.Close()

	result := w.String()
	expect := `{"data":{"columns":["name","time","value"],"types":["string","datetime","double"],"rows":[["name1",1676432363333444555,0.1234],["name1",1676432364666777888,0.2345]]}, "success":true, "reason":"success",`
	require.True(t, len(result) > len(expect))
	require.True(t, strings.HasPrefix(result, expect))
}
