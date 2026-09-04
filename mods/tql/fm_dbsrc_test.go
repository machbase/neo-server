package tql

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSplitExplainSQLText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tokens   []string
		sqlText  string
		wantFull bool
		wantErr  string
	}{
		{
			name:     "full-flag-select",
			input:    "EXPLAIN --full select * from tag_data",
			tokens:   []string{"--full"},
			sqlText:  "select * from tag_data",
			wantFull: true,
		},
		{
			name:     "bare-full-with-cte",
			input:    "explain full with cte as (select 1) select * from cte",
			tokens:   []string{"full"},
			sqlText:  "with cte as (select 1) select * from cte",
			wantFull: true,
		},
		{
			name:     "delimiter-before-select",
			input:    "explain -- select * from log_data",
			tokens:   []string{},
			sqlText:  "select * from log_data",
			wantFull: false,
		},
		{
			name:    "missing-statement",
			input:   "explain --full",
			wantErr: "f(SQL) missing statement after explain options",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, sqlText, err := splitExplainSQLText(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.tokens, tokens)
			require.Equal(t, tc.sqlText, sqlText)
			require.Equal(t, tc.wantFull, explainHasFullFlag(tokens))
		})
	}
}

func TestValidateSqlVerbForSink(t *testing.T) {
	require.NoError(t, validateSqlVerbForSink("insert into t values(1)"))
	require.NoError(t, validateSqlVerbForSink("update t set v = 1"))
	require.NoError(t, validateSqlVerbForSink("delete from t"))
	require.NoError(t, validateSqlVerbForSink("show tables"))

	err := validateSqlVerbForSink("select * from t")
	require.Error(t, err)
	require.Equal(t, `f(SQL) sink does not allow fetch verb "SELECT"`, err.Error())
}

func TestParseRowsAffectedFromMessage(t *testing.T) {
	n, ok := parseRowsAffectedFromMessage("2 rows inserted.")
	require.True(t, ok)
	require.EqualValues(t, 2, n)

	n, ok = parseRowsAffectedFromMessage("a row updated.")
	require.True(t, ok)
	require.EqualValues(t, 1, n)

	n, ok = parseRowsAffectedFromMessage("Created successfully.")
	require.False(t, ok)
	require.EqualValues(t, 0, n)
}

func TestDatabaseStatementHelpers(t *testing.T) {
	node := NewNode(NewTask())

	t.Run("source options", func(t *testing.T) {
		from := node.fmFrom("tag_data", "sensor", "event_time", "tag_name")
		require.Equal(t, "tag_data", from.Table)
		require.Equal(t, "sensor", from.Tag)
		require.Equal(t, "event_time", from.BaseTime)
		require.Equal(t, "tag_name", from.BaseName)

		limit := node.fmLimit(10, 20)
		require.Equal(t, 10, limit.Offset)
		require.Equal(t, 20, limit.Limit)

		dump := node.fmDump(true, true)
		require.True(t, dump.Flag)
		require.True(t, dump.Escape)
	})

	t.Run("between expressions", func(t *testing.T) {
		between, err := node.fmBetween("now-1s", "last+2s", "100ms")
		require.NoError(t, err)
		require.True(t, between.HasPeriod())
		require.Equal(t, 100*time.Millisecond, between.Period())
		require.Equal(t, "(now-1000000000)", between.BeginPart("tag_data", "sensor"))
		require.Equal(t, "(SELECT MAX_TIME+2000000000 FROM V$tag_data_STAT WHERE name = 'sensor')", between.EndPart("tag_data", "sensor"))

		at := time.Unix(0, 123)
		between, err = node.fmBetween(float64(456), at, float64(time.Second))
		require.NoError(t, err)
		require.Equal(t, "456", between.BeginPart("", ""))
		require.Equal(t, "123", between.EndPart("", ""))
		require.Equal(t, time.Second, between.Period())

		_, err = node.fmBetween("tomorrow", "now")
		require.EqualError(t, err, "invalid between expression")
		_, err = node.fmBetween("now", "last", "invalid")
		require.Error(t, err)
	})

	t.Run("sink configuration", func(t *testing.T) {
		tag := node.fmTag("sensor")
		require.Equal(t, "name", tag.Column)
		tag = node.fmTag("sensor", "device")
		require.Equal(t, "device", tag.Column)

		insert, err := node.fmInsert(node.fmTable("tag_data"), tag, "time", "value")
		require.NoError(t, err)
		require.Equal(t, []string{"device", "time", "value"}, insert.columns)
		require.Same(t, node, insert.node)

		_, err = node.fmInsert("value")
		require.EqualError(t, err, "f(INSERT) arg(0) table is not specified")

		appender, err := node.fmAppend(node.fmTable("tag_data"))
		require.NoError(t, err)
		require.Equal(t, "tag_data", appender.table.Name)

		_, err = node.fmAppend()
		require.EqualError(t, err, "f(APPEND) arg(0) table is not specified")
	})
}

// TestFmNamedArrayLiteralPassthrough locks in the contract that named()
// forwards an ARRAY literal string unchanged as a sql.NamedArg value. The
// actual "[1,2,3,10=>4]" text is parsed into an api.Array only later, at
// neo-client bind time (see plan-tql-array-literal.md); the tql layer itself
// performs no parsing.
func TestFmNamedArrayLiteralPassthrough(t *testing.T) {
	node := NewNode(NewTask())

	tests := []struct {
		name  string
		value any
	}{
		{"dense", "[1,2,3,4]"},
		{"sparse", "[1=>1.0, 2=>2.1, 11=>3.14]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			arg := node.fmNamed("arr", tc.value)
			require.Equal(t, sql.NamedArg{Name: "arr", Value: tc.value}, arg)
			// unquoted [1,2,3,4] and quoted "[1,2,3,4]" both arrive here as the
			// same Go string; there is no separate "array literal" tql type.
			require.IsType(t, "", arg.Value)
		})
	}
}
