package tql

import (
	"testing"

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
