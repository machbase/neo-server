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
