package util

import (
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
		{"SHORT(-32768)", int16(-32768)},
		{"uint16(65535)", uint16(65535)},
		{"integer(2147483647)", int32(2147483647)},
		{"uinteger(4294967295)", uint32(4294967295)},
		{"long(1793577127279628041)", int64(1793577127279628041)},
		{"int64(-9223372036854775808)", int64(-9223372036854775808)},
		{"ulong(18446744073709551615)", uint64(18446744073709551615)},
		{"float(1.25)", float32(1.25)},
		{"double(-2.5e2)", float64(-250)},
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

func TestParseNamedLiteralDatetime(t *testing.T) {
	want := time.Date(2026, 11, 2, 1, 32, 7, 197719833, time.UTC)
	for _, tc := range []struct{ text, tz string }{
		{"datetime('2026-11-02 10:32:07.197719833')", "Asia/Seoul"},
		{"datetime(2026-11-02T10:32:07.197719833+09:00)", "Invalid/Zone"},
		{"datetime('2026-11-02 01:32:07.197719833')", ""},
	} {
		got, err := ParseNamedLiteral(tc.text, tc.tz)
		require.NoError(t, err)
		require.True(t, want.Equal(got.(time.Time)))
	}
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
	input := `-- env: named.date=datetime('2026-11-02 10:32:07.197719833') tz=Asia/Seoul named.epoch=long(1793577127279628041)
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
}
