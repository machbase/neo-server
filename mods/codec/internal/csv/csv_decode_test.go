package csv_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestCsvDecoder(t *testing.T) {
	data := []byte("json.data,1678147077000000000,1.234,2.3450,typeval,1234,2345,1111,pnameval,1,\"{\"\"name\"\":1234}\",192.168.1.100")

	dec := csv.NewDecoder()
	input := &stream.ReaderInputStream{Reader: (bytes.NewBuffer(data))}
	dec.SetInputStream(input)
	dec.SetDelimiter(",")
	dec.SetTimeformat("ns")
	dec.SetHeading(false)
	dec.SetColumnTypes(
		"string", "datetime", "double", "float", "string",
		"int", "int16", "int32", "string", "int64", "string",
		"ipv4")
	dec.Open()

	fields, err := dec.NextRow()
	require.Nil(t, err)

	ts := time.Unix(0, 1678147077000000000)
	require.Equal(t, "json.data", fields[0])
	require.Equal(t, ts, fields[1])
	require.Equal(t, 1.234, fields[2])
	require.Equal(t, float32(2.345), fields[3])
	require.Equal(t, "typeval", fields[4])
	require.Equal(t, int64(1234), fields[5])
	require.Equal(t, int64(2345), fields[6])
	require.Equal(t, int64(1111), fields[7])
	require.Equal(t, "pnameval", fields[8])
	require.Equal(t, int64(1), fields[9])
	require.Equal(t, "{\"name\":1234}", fields[10])
	require.Equal(t, "192.168.1.100", fields[11].(net.IP).String())
}

// func TestCsvDecoders(t *testing.T) {
// 	data := []byte("my-car,1670380342000000000,1.0001")

// 	r := csv.NewReader(bytes.NewBuffer(data))
// 	r.Comma = ','

// 	fields, err := r.Read()
// 	if err != nil {
// 		panic(err)
// 	}
// 	require.Equal(t, "my-car", fields[0])
// 	require.Equal(t, "1670380342000000000", fields[1])
// 	require.Equal(t, "1.0001", fields[2])

// }
