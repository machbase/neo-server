package machnet

import "testing"

func TestInferStmtTypeWithLeadingComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want StmtType
	}{
		{
			name: "line comment before select",
			sql:  "-- explain\nselect * from t",
			want: 512,
		},
		{
			name: "block comment before select",
			sql:  "/* explain */\n\tselect * from t",
			want: 512,
		},
		{
			name: "multiple comments before insert select",
			sql:  "/* a */\n-- b\ninsert into t select * from src",
			want: 519,
		},
		{
			name: "comment before alter system",
			sql:  "/* maintenance */\nALTER SYSTEM set trace_log=1",
			want: 256,
		},
		{
			name: "only line comment",
			sql:  "-- only comment",
			want: 0,
		},
		{
			name: "unterminated block comment",
			sql:  "/* only comment",
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inferStmtType(tc.sql)
			if got != tc.want {
				t.Fatalf("inferStmtType() = %d, want %d", got, tc.want)
			}
		})
	}
}
