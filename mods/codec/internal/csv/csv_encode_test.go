package csv_test

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestCsvEncoder(t *testing.T) {
	enc := csv.NewEncoder()
	require.Equal(t, "text/csv", enc.ContentType())

	w := &bytes.Buffer{}
	out := &stream.WriterOutputStream{Writer: w}

	enc.SetOutputStream(out)
	enc.SetTimeformat("KITCHEN")
	enc.SetPrecision(3)
	enc.SetRownum(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeading(true)
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
		"1,1,3.142,text some,KITCHEN,98765,16",
		"2,1,3.142,text some,KITCHEN,98765,NULL",
		"",
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
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "us")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174123456,127.0.0.1,16",
		"1,3,text some,1691800174123456,127.0.0.1,3",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "ms")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174123,127.0.0.1,16",
		"1,3,text some,1691800174123,127.0.0.1,3",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "s")
	expects = []string{
		"col1,col2,col3,col4,col5,col6",
		"3,3,1,1691800174,127.0.0.1,16",
		"1,3,text some,1691800174,127.0.0.1,3",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)
}

func runTimeformat(t *testing.T, format string) string {
	enc := csv.NewEncoder()

	require.Equal(t, "text/csv", enc.ContentType())

	w := &bytes.Buffer{}
	out := &stream.WriterOutputStream{Writer: w}

	enc.SetOutputStream(out)
	enc.SetTimeformat(format)
	enc.SetPrecision(0)
	enc.SetRownum(false)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeading(true)
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
