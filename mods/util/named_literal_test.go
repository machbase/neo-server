package util

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseNamedLiteral(t *testing.T) {
	tests := []struct {
		text string
		want any
	}{
		{"SHORT(-32767)", int16(-32767)},
		{"uint16(65534)", uint16(65534)},
		{"int(-2147483647)", int32(-2147483647)},
		{"integer(2147483647)", int32(2147483647)},
		{"uinteger(4294967294)", uint32(4294967294)},
		{"long(1793577127279628041)", int64(1793577127279628041)},
		{"int64(-9223372036854775807)", int64(-9223372036854775807)},
		{"uint64(9223372036854775808)", uint64(9223372036854775808)},
		{"ulong(18446744073709551614)", uint64(18446744073709551614)},
		{"float(1.25)", float32(1.25)},
		{"double(-2.5e2)", float64(-250)},
		{"float(3.4028233e38)", math.Nextafter32(math.MaxFloat32, 0)},
		{"double(1.7976931348623155e308)", math.Nextafter(math.MaxFloat64, 0)},
		{"string('')", ""},
		{`string('long(123)')`, "long(123)"},
		{`string('string(\'a\\b\')')`, `string('a\b')`},
		{"007", "007"},
		{" custom(42) ", " custom(42) "},
		{"[1=>1.0, 2=>2.1]", "[1=>1.0, 2=>2.1]"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, err := ParseNamedLiteral(tt.text, "")
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
	for _, text := range []string{
		"short(32768)", "ushort(65536)", "int(-2147483649)", "uint32(-1)",
		"long(9223372036854775808)", "ulong(18446744073709551616)", "long(1.25)",
		"long(abc)", "long(1)extra", "float(1e40)", "double(NaN)", "double(Inf)",
		"string('a', 'b')", "string(long(1))", "string(abc)", "long(",
	} {
		t.Run(text, func(t *testing.T) {
			_, err := ParseNamedLiteral(text, "")
			require.Error(t, err)
		})
	}
}

func TestParseNamedLiteralReservedNull(t *testing.T) {
	for _, text := range []string{
		"short(-32768)", "uint16(65535)", "integer(-2147483648)", "uinteger(4294967295)",
		"long(-9223372036854775808)", "ulong(18446744073709551615)",
		"float(3.402823466e38)", "float32(3.4028235e38)",
		"double(1.7976931348623158e308)", "float64(1.7976931348623157e308)",
		"datetime(1969-12-31T23:59:59.999999999Z)",
		"datetime(1970-01-01T08:59:59.999999999+09:00)",
	} {
		t.Run(text, func(t *testing.T) {
			_, err := ParseNamedLiteral(text, "")
			require.ErrorContains(t, err, "reserved for Machbase NULL")
		})
	}
	for _, tc := range []struct {
		text string
		ns   int64
	}{
		{"datetime(1969-12-31T23:59:59.999999998Z)", -2},
		{"datetime(1970-01-01T00:00:00Z)", 0},
	} {
		got, err := ParseNamedLiteral(tc.text, "")
		require.NoError(t, err)
		require.Equal(t, tc.ns, got.(time.Time).UnixNano())
	}
}

func TestParseNamedLiteralDatetime(t *testing.T) {
	want := time.Date(2026, 11, 2, 1, 32, 7, 197719833, time.UTC)
	for _, tc := range []struct{ text, tz string }{
		{"datetime('2026-11-02 10:32:07.197719833')", "Asia/Seoul"},
		{"datetime('2026-11-02 10:32:07.197719833')", "KST"},
		{"datetime(2026-11-02T10:32:07.197719833+09:00)", "Invalid/Zone"},
		{"datetime('2026-11-02 01:32:07.197719833')", ""},
		{"datetime('2026-11-02 01:32:07.197719833')", "utc"},
	} {
		got, err := ParseNamedLiteral(tc.text, tc.tz)
		require.NoError(t, err)
		require.True(t, want.Equal(got.(time.Time)))
	}
	value, err := ParseNamedLiteral("datetime('2026-11-02 01:32:07.197719833')", "local")
	require.NoError(t, err)
	require.True(t, time.Date(2026, 11, 2, 1, 32, 7, 197719833, time.Local).Equal(value.(time.Time)))
	_, err = ParseNamedLiteral("datetime('2026-11-02 01:32:07')", "Invalid/Zone")
	require.Error(t, err)
	for _, text := range []string{
		"datetime('2026-02-30 10:32:07')",
		"datetime(2026-11-02T10:32:07.1234567890Z)",
		"datetime(2026-11-02T10:32:07+09:60)",
		"datetime(2262-04-11T23:47:16.854775808Z)",
	} {
		_, err := ParseNamedLiteral(text, "")
		require.Error(t, err)
	}
}

func TestSplitSqlStatementsNamedLiterals(t *testing.T) {
	input := `-- env: named.date=datetime('2026-11-02 10:32:07.197719833') tz=KST named.epoch=long(1793577127279628041)
-- env: named.x="string('a\b)')" named.arr=[1=>1.0, 2=>2.1]
select :epoch, :date, :x, :arr;
-- env: tz=UTC
select :date;
-- env: reset tz=UTC named.date=datetime('2026-11-02 10:32:07.197719833') reset named.x="long(abc)"
select :x;`
	stmts, err := SplitSqlStatements(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, stmts, 7)
	first, second, third := stmts[2].Env.Named, stmts[4].Env.Named, stmts[6].Env.Named
	require.Equal(t, "long(1793577127279628041)", first["epoch"])
	require.Equal(t, "datetime(2026-11-02T01:32:07.197719833Z)", first["date"])
	require.Equal(t, "[1=>1.0, 2=>2.1]", first["arr"])
	require.Equal(t, map[string]string{"date": first["date"]}, second)
	require.Equal(t, map[string]string{"x": "string('long(abc)')"}, third)
	value, err := ParseNamedLiteral(first["x"], "")
	require.NoError(t, err)
	require.Equal(t, `string('a\b)')`, value)

	_, err = SplitSqlStatements(strings.NewReader("-- env: named.date=datetime('2026-11-02 10:32:07'\nselect :date;"))
	require.ErrorContains(t, err, "line 1: named.date: datetime literal")
	_, err = SplitSqlStatements(strings.NewReader("-- env: tz=KST named.date=datetime('1970-01-01 08:59:59.999999999')\nselect :date;"))
	require.ErrorContains(t, err, `line 1: named parameter "date": datetime literal: value 1970-01-01 08:59:59.999999999 is reserved for Machbase NULL`)
}
