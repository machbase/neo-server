package json_test

import (
	"bytes"
	"io"
	"regexp"
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
				["name1", 1676528839, 0.1234, 1234, 12345, 12340, 123456],
				["name2", 1676528840, 0.2345, 2345, 23456, 23450, 234567]
		    ],
			"columns": ["name", "time", "value", "count"]
		},
		"other": 999
	}`)

	dec := &json.Decoder{}
	dec.SetTableName("test")
	dec.SetTimeformat("s")
	dec.SetInputStream(&stream.ReaderInputStream{Reader: input})
	dec.SetColumnTypes("string", "datetime", "double", "int", "int16", "int32", "int64")
	dec.Open()

	rec, _, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])
	require.Equal(t, int32(1234), rec[3])
	require.Equal(t, int16(12345), rec[4])
	require.Equal(t, int32(12340), rec[5])
	require.Equal(t, int64(123456), rec[6])

	rec, _, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(1676528840, 0), rec[1])
	require.Equal(t, 0.2345, rec[2])
	require.Equal(t, int32(2345), rec[3])
	require.Equal(t, int16(23456), rec[4])
	require.Equal(t, int32(23450), rec[5])
	require.Equal(t, int64(234567), rec[6])

	_, _, err = dec.NextRow()
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

	rec, _, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(0, 1676432363333444555), rec[1])
	require.Equal(t, 0.1234, rec[2])

	rec, _, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(0, 1676432364666777888), rec[1])
	require.Equal(t, 0.2345, rec[2])

	_, _, err = dec.NextRow()
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

	rec, _, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	rec, _, err = dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name2", rec[0])
	require.Equal(t, time.Unix(1676528840, 0), rec[1])
	require.Equal(t, 0.2345, rec[2])

	_, _, err = dec.NextRow()
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

	rec, _, err := dec.NextRow()
	require.Nil(t, err)
	require.Equal(t, "name1", rec[0])
	require.Equal(t, time.Unix(1676528839, 0), rec[1])
	require.Equal(t, 0.1234, rec[2])

	_, _, err = dec.NextRow()
	require.Equal(t, io.EOF, err)
}

func TestEncoder(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["name","time","value","place"],"types":["string","datetime","double","string"],"rows":[["name1",1676432363333444555,0.1234,"Office"],["name2",1676432364666777888,0.2345,"Home"]]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)

}

func TestEncoderWithRownum(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRownum(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["ROWNUM","name","time","value","place"],"types":["int64","string","datetime","double","string"],"rows":[[1,"name1",1676432363333444555,0.1234,"Office"],[2,"name2",1676432364666777888,0.2345,"Home"]]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)

}

func TestEncoderTranspose(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRowsFlatten(true)
	enc.SetTranspose(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["name","time","value","place"],"types":["string","datetime","double","string"],"cols":[["name1","name2"],[1676432363333444555,1676432364666777888],[0.1234,0.2345],["Office","Home"]]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)
}

func TestEncoderTransposeWithRownum(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRowsFlatten(true)
	enc.SetTranspose(true)
	enc.SetRownum(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["name","time","value","place"],"types":["string","datetime","double","string"],"cols":[["name1","name2"],[1676432363333444555,1676432364666777888],[0.1234,0.2345],["Office","Home"]]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)
}

func TestEncoderRowsFlatten(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRowsFlatten(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["name","time","value","place"],"types":["string","datetime","double","string"],"rows":["name1",1676432363333444555,0.1234,"Office","name2",1676432364666777888,0.2345,"Home"]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)
}

func TestEncoderRowsFlattenWithRownum(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRowsFlatten(true)
	enc.SetRownum(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	result = result[0:strings.LastIndex(result, `,"elapse`)]
	expect := `{"data":{"columns":["ROWNUM","name","time","value","place"],"types":["int64","string","datetime","double","string"],"rows":[1,"name1",1676432363333444555,0.1234,"Office",2,"name2",1676432364666777888,0.2345,"Home"]},"success":true,"reason":"success"`
	require.Equal(t, expect, result)
}

func TestEncoderRowsArray(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRowsFlatten(true)
	enc.SetRowsArray(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	expect := `^{"data":{"columns":\["name","time","value","place"\],"types":\["string","datetime","double","string"\],"rows":\[{"name":"name1","place":"Office","time":1676432363333444555,"value":0.1234},{"name":"name2","place":"Home","time":1676432364666777888,"value":0.2345}\]},"success":true,"reason":"success","elapse":".+"}`
	reg := regexp.MustCompile(expect)
	if !reg.MatchString(result) {
		t.Log("Expect:", expect)
		t.Log("Actual:", result)
		t.Fail()
	}
}

func TestEncoderRowsArrayWithRownum(t *testing.T) {
	w := &bytes.Buffer{}

	enc := json.NewEncoder()
	enc.SetTimeformat("ns")
	enc.SetOutputStream(stream.NewOutputStreamWriter(w))
	enc.SetColumnTypes("string", "datetime", "double", "string")
	enc.SetColumns("name", "time", "value", "place")
	enc.SetRownum(true)
	enc.SetRowsFlatten(true)
	enc.SetRowsArray(true)
	require.Equal(t, enc.ContentType(), "application/json")

	enc.Open()
	enc.AddRow([]any{"name1", 1676432363333444555, 0.1234, "Office"})
	enc.AddRow([]any{"name2", 1676432364666777888, 0.2345, "Home"})
	enc.Close()

	result := w.String()
	expect := `^{"data":{"columns":\["ROWNUM","name","time","value","place"\],"types":\["int64","string","datetime","double","string"\],"rows":\[{"ROWNUM":1,"name":"name1","place":"Office","time":1676432363333444555,"value":0.1234},{"ROWNUM":2,"name":"name2","place":"Home","time":1676432364666777888,"value":0.2345}\]},"success":true,"reason":"success","elapse":".+"}`
	reg := regexp.MustCompile(expect)
	if !reg.MatchString(result) {
		t.Log("Expect:", expect)
		t.Log("Actual:", result)
		t.Fail()
	}
}
