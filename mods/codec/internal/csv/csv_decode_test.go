package csv_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/util/charset"
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
	require.Equal(t, int(1234), fields[5])
	require.Equal(t, int16(2345), fields[6])
	require.Equal(t, int32(1111), fields[7])
	require.Equal(t, "pnameval", fields[8])
	require.Equal(t, int64(1), fields[9])
	require.Equal(t, "{\"name\":1234}", fields[10])
	require.Equal(t, "192.168.1.100", fields[11].(net.IP).String())
}

func TestCsvDecoderCharset(t *testing.T) {
	// big endian
	// 0000000 f8cd d1cd b5a4 eca4 c6a4 ada4 bfa4 b8ca
	// 0000010 fabb b3a5 bca1 312c 3037 3931 3331 3831
	// 0000020 2c32 2e33 3431 3531 3239 000a
	data := []byte{
		0xf8, 0xcd, 0xd1, 0xcd, 0xb5, 0xa4, 0xec, 0xa4, 0xc6, 0xa4, 0xad, 0xa4, 0xbf, 0xa4, 0xb8, 0xca,
		0xfa, 0xbb, 0xb3, 0xa5, 0xbc, 0xa1, 0x31, 0x2c, 0x30, 0x37, 0x39, 0x31, 0x33, 0x31, 0x38, 0x31,
		0x2c, 0x32, 0x2e, 0x33, 0x34, 0x31, 0x35, 0x31, 0x32, 0x39, 0x00, 0x0a,
	}
	// convert to little endian
	for i := 0; i < len(data); i += 2 {
		if len(data) > i+1 {
			data[i], data[i+1] = data[i+1], data[i]
		}
	}

	input := &stream.ReaderInputStream{Reader: bytes.NewBuffer(data)}

	dec := csv.NewDecoder()
	dec.SetInputStream(input)
	eucjp, _ := charset.Encoding("EUC-JP")
	dec.SetCharsetEncoding(eucjp)
	dec.SetDelimiter(",")
	dec.SetHeading(false)
	dec.SetColumnTypes(
		"string", "string", "string")
	dec.Open()
	fields, err := dec.NextRow()

	require.Nil(t, err)
	require.Equal(t, "利用されてきた文字コー", fields[0])
	require.Equal(t, "1701913182", fields[1])
	require.Equal(t, "3.141592", fields[2])
}
