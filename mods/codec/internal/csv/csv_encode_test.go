package csv_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal/csv"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/stretchr/testify/require"
)

func TestCsvEncoder(t *testing.T) {
	enc := csv.NewEncoder()
	require.Equal(t, "text/csv; charset=utf-8", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat("KITCHEN")
	enc.SetPrecision(3)
	enc.SetRownum(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeader(true)
	err := enc.Open()
	require.Nil(t, err)

	ts := time.Unix(1691800174, 123456789).UTC()
	i64 := int64(98765)
	sval := "text some"
	i16 := int16(16)
	enc.AddRow([]any{
		int8(1),
		float64(3.141592),
		sval,
		ts,
		i64,
		i16,
	})
	enc.AddRow([]any{
		int32(1),
		float32(3.141592),
		&sval,
		&ts,
		&i64,
		nil,
	})

	enc.Close()

	expects := []string{
		"ROWNUM,col1,col2,col3,col4,col5,col6",
		"1,1,3.142,text some,12:29:34AM,98765,16",
		"2,1,3.142,text some,12:29:34AM,98765,NULL",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}

func TestCsvEncoderNullValue(t *testing.T) {
	enc := csv.NewEncoder()
	require.Equal(t, "text/csv; charset=utf-8", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat("KITCHEN")
	enc.SetPrecision(2)
	enc.SetRownum(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeader(true)
	enc.SetSubstituteNull(1.234567)
	err := enc.Open()
	require.Nil(t, err)

	ts := time.Unix(1691800174, 123456789).UTC()
	i64 := int64(98765)
	sval := "text some"
	i16 := int16(16)
	enc.AddRow([]any{
		int8(1),
		float64(3.141592),
		sval,
		ts,
		i64,
		i16,
	})
	enc.AddRow([]any{
		int32(1),
		float32(3.141592),
		&sval,
		&ts,
		&i64,
		nil,
	})

	enc.Close()

	expects := []string{
		"ROWNUM,col1,col2,col3,col4,col5,col6",
		"1,1,3.14,text some,12:29:34AM,98765,16",
		"2,1,3.14,text some,12:29:34AM,98765,1.23",
		"\n",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}

func TestCsvTimeformat(t *testing.T) {
	result := runTimeformat(t, "ns")
	expects := []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174123456789,127.0.0.1,16",
		"1,3,text some,1691800174123456789,127.0.0.1,3",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "us")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174123456,127.0.0.1,16",
		"1,3,text some,1691800174123456,127.0.0.1,3",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "ms")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174123,127.0.0.1,16",
		"1,3,text some,1691800174123,127.0.0.1,3",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "s")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174,127.0.0.1,16",
		"1,3,text some,1691800174,127.0.0.1,3",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)
}

func runTimeformat(t *testing.T, format string) string {
	enc := csv.NewEncoder()

	require.Equal(t, "text/csv; charset=utf-8", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat(format)
	enc.SetPrecision(0)
	enc.SetRownum(false)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeader(true)
	err := enc.Open()
	require.Nil(t, err)

	ts := time.Unix(1691800174, 123456789).UTC()
	ip4 := net.ParseIP("127.0.0.1")
	sval := "text some"
	i16 := int16(16)
	ival := int(1)
	f32 := float32(3.141591)
	f64 := float64(3.141591)
	enc.AddRow([]any{
		&f32,
		float64(3.141592),
		ival,
		ts,
		net.ParseIP("127.0.0.1"),
		&i16,
	})
	enc.AddRow([]any{
		&ival,
		float32(3.141592),
		&sval,
		&ts,
		&ip4,
		&f64,
	})

	enc.Flush(false)
	enc.Close()

	return w.String()
}

type flushBuffer struct {
	bytes.Buffer
	flushed bool
}

func (f *flushBuffer) Flush() error {
	f.flushed = true
	return nil
}

func TestCsvEncoderSetterPaths(t *testing.T) {
	w := &flushBuffer{}
	enc := csv.NewEncoder()
	enc.SetOutputStream(w)
	enc.SetHeading(true)
	enc.SetDelimiter(";")
	enc.SetColumns("a", "b")
	enc.SetColumnTypes()
	require.NoError(t, enc.Open())
	require.NoError(t, enc.AddRow([]any{1, nil}))
	enc.Flush(true)
	enc.Close()
	require.True(t, w.flushed)
	require.Contains(t, w.String(), "a;b")
}

func TestCsvEncoderBinaryMode(t *testing.T) {
	t.Run("default hex", func(t *testing.T) {
		enc := csv.NewEncoder()
		w := &bytes.Buffer{}
		enc.SetOutputStream(w)
		enc.SetColumns("bin", "ptr_bin", "empty_bin", "nil_bin")
		enc.SetHeader(true)
		require.NoError(t, enc.Open())

		ptrBin := []byte{0x03, 0x04}
		var nilBin []byte
		require.NoError(t, enc.AddRow([]any{[]byte{0x01, 0x02}, &ptrBin, []byte{}, nilBin}))
		enc.Close()

		expects := []string{
			"bin,ptr_bin,empty_bin,nil_bin",
			"AQI=,AwQ=,,",
			"",
			"",
		}
		require.Equal(t, strings.Join(expects, "\n"), w.String())
	})

	t.Run("base64", func(t *testing.T) {
		enc := csv.NewEncoder()
		w := &bytes.Buffer{}
		enc.SetOutputStream(w)
		enc.SetColumns("bin", "ptr_bin", "empty_bin", "nil_bin")
		enc.SetHeader(true)
		enc.SetBinaryMode("BASE64")
		require.NoError(t, enc.Open())

		ptrBin := []byte{0x03, 0x04}
		var nilBin []byte
		require.NoError(t, enc.AddRow([]any{[]byte{0x01, 0x02}, &ptrBin, []byte{}, nilBin}))
		enc.Close()

		expects := []string{
			"bin,ptr_bin,empty_bin,nil_bin",
			"AQI=,AwQ=,,",
			"",
			"",
		}
		require.Equal(t, strings.Join(expects, "\n"), w.String())
	})

	t.Run("unknown mode falls back to hex", func(t *testing.T) {
		enc := csv.NewEncoder()
		w := &bytes.Buffer{}
		enc.SetOutputStream(w)
		enc.SetColumns("bin")
		enc.SetHeader(true)
		enc.SetBinaryMode("raw")
		require.NoError(t, enc.Open())

		require.NoError(t, enc.AddRow([]any{[]byte{0x0a, 0x0b}}))
		enc.Close()

		expects := []string{
			"bin",
			"0x0a0b",
			"",
			"",
		}
		require.Equal(t, strings.Join(expects, "\n"), w.String())
	})
}

func TestCsvEncoderSqlAndGeoTypes(t *testing.T) {
	enc := csv.NewEncoder()
	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat("KITCHEN")
	enc.SetSubstituteNull("N/A")
	enc.SetColumns("bool_v", "byte_v", "float_v", "int16_v", "int32_v", "int64_v", "string_v", "time_v", "float32_v", "ip_v", "latlon_v", "point_v")
	enc.SetHeader(true)
	require.NoError(t, enc.Open())

	ts := time.Unix(1691800174, 0).UTC()
	ip := net.ParseIP("127.0.0.1")
	require.NoError(t, enc.AddRow([]any{
		&sql.NullBool{Bool: true, Valid: true},
		&sql.NullByte{Byte: 7, Valid: true},
		&sql.NullFloat64{Float64: 3.5, Valid: true},
		&sql.NullInt16{Int16: 16, Valid: true},
		&sql.NullInt32{Int32: 32, Valid: true},
		&sql.NullInt64{Int64: 64, Valid: true},
		&sql.NullString{String: "text", Valid: true},
		&sql.NullTime{Time: ts, Valid: true},
		&sql.Null[float32]{V: float32(1.25), Valid: true},
		&sql.Null[net.IP]{V: ip, Valid: true},
		nums.NewLatLon(37.123, 127.456),
		nums.NewGeoPoint(nums.NewLatLon(38.321, 128.654), nil),
	}))
	require.NoError(t, enc.AddRow([]any{
		&sql.NullBool{Valid: false},
		&sql.NullByte{Valid: false},
		&sql.NullFloat64{Valid: false},
		&sql.NullInt16{Valid: false},
		&sql.NullInt32{Valid: false},
		&sql.NullInt64{Valid: false},
		&sql.NullString{Valid: false},
		&sql.NullTime{Valid: false},
		&sql.Null[float32]{Valid: false},
		&sql.Null[net.IP]{Valid: false},
		nil,
		nil,
	}))
	enc.Close()

	expects := []string{
		"bool_v,byte_v,float_v,int16_v,int32_v,int64_v,string_v,time_v,float32_v,ip_v,latlon_v,point_v",
		"true,7,3.5,16,32,64,text,12:29:34AM,1.25,127.0.0.1," +
			"\"[37.123,127.456]\",\"[38.321,128.654]\"",
		"N/A,N/A,N/A,N/A,N/A,N/A,N/A,N/A,N/A,N/A,N/A,N/A",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
}

func TestCsvEncoderTreatIntValueAsFloat(t *testing.T) {
	enc := csv.NewEncoder()
	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetPrecision(2)
	enc.SetColumns("int_v", "int8_v", "int16_v", "int32_v", "int64_v")
	enc.SetColumnTypes(api.DataTypeFloat64, api.DataTypeFloat32, api.DataTypeFloat64, api.DataTypeFloat32, api.DataTypeFloat64)
	enc.SetHeader(true)
	require.NoError(t, enc.Open())

	require.NoError(t, enc.AddRow([]any{int(1), int8(2), int16(3), int32(4), int64(5)}))
	enc.Close()

	expects := []string{
		"int_v,int8_v,int16_v,int32_v,int64_v",
		"1.00,2.00,3.00,4.00,5.00",
		"",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
}
