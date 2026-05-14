package box_test

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/internal/box"
	"github.com/stretchr/testify/require"
)

func TestBox1(t *testing.T) {
	enc := box.NewEncoder()

	require.Equal(t, "text/plain", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat("KITCHEN")
	enc.SetPrecision(3)
	enc.SetRownum(true)
	enc.SetBoxStyle("light")
	enc.SetBoxSeparateColumns(true)
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
		float32(0.1400),
		&sval,
		&ts,
		&i64,
		nil,
	})

	enc.Close()

	expects := []string{
		"┌────────┬──────┬───────┬───────────┬────────────┬───────┬──────┐",
		"│ ROWNUM │ COL1 │ COL2  │ COL3      │ COL4       │ COL5  │ COL6 │",
		"├────────┼──────┼───────┼───────────┼────────────┼───────┼──────┤",
		"│      1 │ 1    │ 3.142 │ text some │ 12:29:34AM │ 98765 │ 16   │",
		"│      2 │ 1    │ 0.140 │ text some │ 12:29:34AM │ 98765 │ NULL │",
		"└────────┴──────┴───────┴───────────┴────────────┴───────┴──────┘",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}

func TestNano(t *testing.T) {
	result := runTimeformat(t, "ns")
	expects := []string{
		"+------+------+-----------+---------------------+-----------+------+",
		"| COL1 | COL2 | COL3      | COL4                | COL5      | COL6 |",
		"+------+------+-----------+---------------------+-----------+------+",
		"| 3    | 3    | 1         | 1691800174123456789 | 127.0.0.1 | 16   |",
		"| 1    | 3    | text some | 1691800174123456789 | 127.0.0.1 | 3    |",
		"+------+------+-----------+---------------------+-----------+------+",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "us")
	expects = []string{
		"+------+------+-----------+------------------+-----------+------+",
		"| COL1 | COL2 | COL3      | COL4             | COL5      | COL6 |",
		"+------+------+-----------+------------------+-----------+------+",
		"| 3    | 3    | 1         | 1691800174123456 | 127.0.0.1 | 16   |",
		"| 1    | 3    | text some | 1691800174123456 | 127.0.0.1 | 3    |",
		"+------+------+-----------+------------------+-----------+------+",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "ms")
	expects = []string{
		"+------+------+-----------+---------------+-----------+------+",
		"| COL1 | COL2 | COL3      | COL4          | COL5      | COL6 |",
		"+------+------+-----------+---------------+-----------+------+",
		"| 3    | 3    | 1         | 1691800174123 | 127.0.0.1 | 16   |",
		"| 1    | 3    | text some | 1691800174123 | 127.0.0.1 | 3    |",
		"+------+------+-----------+---------------+-----------+------+",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)

	result = runTimeformat(t, "s")
	expects = []string{
		"+------+------+-----------+------------+-----------+------+",
		"| COL1 | COL2 | COL3      | COL4       | COL5      | COL6 |",
		"+------+------+-----------+------------+-----------+------+",
		"| 3    | 3    | 1         | 1691800174 | 127.0.0.1 | 16   |",
		"| 1    | 3    | text some | 1691800174 | 127.0.0.1 | 3    |",
		"+------+------+-----------+------------+-----------+------+",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), result)
}

func TestBoxFloat(t *testing.T) {
	enc := box.NewEncoder()

	require.Equal(t, "text/plain", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetRownum(true)
	enc.SetBoxStyle("double")
	enc.SetBoxSeparateColumns(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeading(true)
	err := enc.Open()
	require.Nil(t, err)

	enc.AddRow([]any{
		0.0,
		1.234000,
		float32(1.234000),
		-1.234000,
		float32(-1.234000),
		math.Pi,
	})
	enc.Close()

	expects := []string{
		"╔════════╦══════╦═══════╦═══════╦════════╦════════╦═══════════════════╗",
		"║ ROWNUM ║ COL1 ║ COL2  ║ COL3  ║ COL4   ║ COL5   ║ COL6              ║",
		"╠════════╬══════╬═══════╬═══════╬════════╬════════╬═══════════════════╣",
		"║      1 ║ 0    ║ 1.234 ║ 1.234 ║ -1.234 ║ -1.234 ║ 3.141592653589793 ║",
		"╚════════╩══════╩═══════╩═══════╩════════╩════════╩═══════════════════╝",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}

func TestBoxFloat2(t *testing.T) {
	enc := box.NewEncoder()

	require.Equal(t, "text/plain", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetRownum(true)
	enc.SetPrecision(2)
	enc.SetBoxStyle("bold")
	enc.SetBoxSeparateColumns(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeading(true)
	err := enc.Open()
	require.Nil(t, err)

	enc.AddRow([]any{
		0.0,
		1.234000,
		float32(1.234000),
		-1.234000,
		float32(-1.234000),
		math.Pi,
	})
	enc.AddRow([]any{
		0.005,
		1.235000,
		float32(1.235000),
		-1.235000,
		float32(-1.235000),
		math.Pi,
	})
	enc.Close()

	expects := []string{
		"┏━━━━━━━━┳━━━━━━┳━━━━━━┳━━━━━━┳━━━━━━━┳━━━━━━━┳━━━━━━┓",
		"┃ ROWNUM ┃ COL1 ┃ COL2 ┃ COL3 ┃ COL4  ┃ COL5  ┃ COL6 ┃",
		"┣━━━━━━━━╋━━━━━━╋━━━━━━╋━━━━━━╋━━━━━━━╋━━━━━━━╋━━━━━━┫",
		"┃      1 ┃ 0.00 ┃ 1.23 ┃ 1.23 ┃ -1.23 ┃ -1.23 ┃ 3.14 ┃",
		"┃      2 ┃ 0.01 ┃ 1.24 ┃ 1.24 ┃ -1.24 ┃ -1.24 ┃ 3.14 ┃",
		"┗━━━━━━━━┻━━━━━━┻━━━━━━┻━━━━━━┻━━━━━━━┻━━━━━━━┻━━━━━━┛",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}

func runTimeformat(t *testing.T, format string) string {
	enc := box.NewEncoder()

	require.Equal(t, "text/plain", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTimeformat(format)
	enc.SetPrecision(0)
	enc.SetRownum(false)
	enc.SetBoxStyle("simeple")
	enc.SetBoxSeparateColumns(true)
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

func TestPrecision(t *testing.T) {
	tests := []struct {
		input  float64
		expect string
	}{
		{1.995, "1.995"},
		{1.994, "1.994"},
		{1.99, "1.99"},
	}

	ten13 := math.Pow10(13)
	for _, tt := range tests {
		floor := math.Floor(tt.input*ten13) / ten13
		result := fmt.Sprintf("%v", floor)
		fmt.Println("==>", tt.input, "=>", result)
	}
}

func TestBinaryFormat(t *testing.T) {
	tests := []struct {
		input        []byte
		binaryformat string
		expect       string
	}{
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "preview", "│ 0x0102030405.. │"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "hex", "│ 0x010203040506 │"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "bytes", "│ [1 2 3 4 5 6] │"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "base64", "│ AQIDBAUG │"},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "_unknown_", "│ 0x010203040506 │"},
	}

	for _, tt := range tests {
		enc := box.NewEncoder()

		require.Equal(t, "text/plain", enc.ContentType())

		w := &bytes.Buffer{}
		enc.SetOutputStream(w)
		enc.SetBinaryFormat(tt.binaryformat)
		enc.SetRownum(true)
		enc.SetBoxStyle("round")
		enc.SetBoxSeparateColumns(true)
		enc.SetColumns("BIN")
		enc.SetHeading(true)
		err := enc.Open()
		require.Nil(t, err)
		enc.AddRow([]any{tt.binaryformat, tt.input})
		enc.Close()

		result := w.String()
		require.Contains(t, result, tt.expect)
	}
}

func TestBoxWide(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skip on windows")
	}
	enc := box.NewEncoder()

	require.Equal(t, "text/plain", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetRownum(true)
	enc.SetBoxStyle("round")
	enc.SetBoxSeparateColumns(true)
	enc.SetColumns("col1", "col2", "col3", "col4", "col5", "col6")
	enc.SetHeading(true)
	err := enc.Open()
	require.Nil(t, err)

	// str := "🄒CD"
	// fmt.Println("=========>", str, len(str), len([]rune(str)), utf8.RuneCountInString(str))
	// str = "ABCD"
	// fmt.Println("=========>", str, len(str), len([]rune(str)), utf8.RuneCountInString(str))
	enc.AddRow([]any{
		0.0,
		1.234000,
		float32(-1.234000),
		-1.234000,
		"🄒CD",
		math.Pi,
	})
	enc.AddRow([]any{
		0.0,
		1.234000,
		float32(-1.234000),
		-1.234000,
		"ABCD",
		math.Pi,
	})
	enc.Close()

	// FIXME "| 🄒CD  |" should be "| 🄒CD |"
	expects := []string{
		"╭────────┬──────┬───────┬────────┬────────┬──────┬───────────────────╮",
		"│ ROWNUM │ COL1 │ COL2  │ COL3   │ COL4   │ COL5 │ COL6              │",
		"├────────┼──────┼───────┼────────┼────────┼──────┼───────────────────┤",
		`│      1 │ 0    │ 1.234 │ -1.234 │ -1.234 │ 🄒CD  │ 3.141592653589793 │`,
		"│      2 │ 0    │ 1.234 │ -1.234 │ -1.234 │ ABCD │ 3.141592653589793 │",
		"╰────────┴──────┴───────┴────────┴────────┴──────┴───────────────────╯",
		"",
	}
	require.Equal(t, strings.Join(expects, "\n"), w.String())
	fmt.Println()
}
