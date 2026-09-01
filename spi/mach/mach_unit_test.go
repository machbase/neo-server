package mach

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestErrorConstructors(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "mach", err: ErrDatabaseMach(3001, "failed"), want: "MACH-ERR 3001 failed"},
		{name: "returns", err: ErrDatabaseReturns("Fn", -1), want: "Fn returns -1"},
		{name: "returns_at_idx", err: ErrDatabaseReturnsAtIdx("Fn", 2, -1), want: "Fn idx 2 returns -1"},
		{name: "wrap", err: ErrDatabaseWrap("Fn", cause), want: "Fn cause"},
		{name: "append_unknown_type", err: ErrDatabaseAppendUnknownType("unknown"), want: "MachAppendData unknown column type 'unknown'"},
		{name: "append_wrong_type", err: ErrDatabaseAppendWrongType("value", "COL", "integer"), want: "MachAppendData cannot apply string to COL (integer)"},
		{name: "append_wrong_time_value_type", err: ErrDatabaseAppendWrongTimeValueType("string", "TIME", "datetime"), want: "MachAppendData cannot apply string to TIME (datetime)"},
		{name: "append_wrong_value_count", err: ErrDatabaseAppendWrongValueCount(2, 1), want: "MachAppendData required 2, but got 1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("error was %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStmtTypePredicates(t *testing.T) {
	tests := []struct {
		typ          StmtType
		selectStmt   bool
		ddl          bool
		alterSystem  bool
		insert       bool
		deleteStmt   bool
		insertSelect bool
		update       bool
		execRollup   bool
	}{
		{typ: 0},
		{typ: 1, ddl: true},
		{typ: 255, ddl: true},
		{typ: 256, alterSystem: true},
		{typ: 511, alterSystem: true},
		{typ: 512, selectStmt: true},
		{typ: 513, insert: true},
		{typ: 514, deleteStmt: true},
		{typ: 518, deleteStmt: true},
		{typ: 519, insertSelect: true},
		{typ: 520, update: true},
		{typ: 521},
		{typ: 522, execRollup: true},
		{typ: 524, execRollup: true},
		{typ: 525},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("stmt_%d", tc.typ), func(t *testing.T) {
			if got := tc.typ.IsSelect(); got != tc.selectStmt {
				t.Fatalf("IsSelect() was %v, want %v", got, tc.selectStmt)
			}
			if got := tc.typ.IsDDL(); got != tc.ddl {
				t.Fatalf("IsDDL() was %v, want %v", got, tc.ddl)
			}
			if got := tc.typ.IsAlterSystem(); got != tc.alterSystem {
				t.Fatalf("IsAlterSystem() was %v, want %v", got, tc.alterSystem)
			}
			if got := tc.typ.IsInsert(); got != tc.insert {
				t.Fatalf("IsInsert() was %v, want %v", got, tc.insert)
			}
			if got := tc.typ.IsDelete(); got != tc.deleteStmt {
				t.Fatalf("IsDelete() was %v, want %v", got, tc.deleteStmt)
			}
			if got := tc.typ.IsInsertSelect(); got != tc.insertSelect {
				t.Fatalf("IsInsertSelect() was %v, want %v", got, tc.insertSelect)
			}
			if got := tc.typ.IsUpdate(); got != tc.update {
				t.Fatalf("IsUpdate() was %v, want %v", got, tc.update)
			}
			if got := tc.typ.IsExecRollup(); got != tc.execRollup {
				t.Fatalf("IsExecRollup() was %v, want %v", got, tc.execRollup)
			}
		})
	}
}

func TestLinkAndCodePageHelpers(t *testing.T) {
	if got := LinkInfo(); got != LibMachLinkInfo {
		t.Fatalf("LinkInfo() was %q, want %q", got, LibMachLinkInfo)
	}
	if got := translateCodePage("abc-123"); got != "abc-123" {
		t.Fatalf("translateCodePage() changed ASCII text to %q", got)
	}
}

func TestEngMakeAppendBuffer(t *testing.T) {
	buffer := EngMakeAppendBuffer(nil, []string{"A", "B"}, []string{"integer", "varchar"})
	if buffer.stmt != nil {
		t.Fatalf("stmt was %v, want nil", buffer.stmt)
	}
	if len(buffer.columnNames) != 2 || len(buffer.columnTypes) != 2 || len(buffer.buffer) != 2 {
		t.Fatalf("unexpected buffer sizes: names=%d types=%d buffer=%d", len(buffer.columnNames), len(buffer.columnTypes), len(buffer.buffer))
	}
}

func TestAppendBufferValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		columnNames []string
		columnTypes []string
		values      []any
		want        string
	}{
		{
			name:        "wrong_value_count",
			columnNames: []string{"A", "B"},
			columnTypes: []string{"integer", "integer"},
			values:      []any{int32(1)},
			want:        "MachAppendData required 2, but got 1",
		},
		{
			name:        "unknown_type",
			columnNames: []string{"A"},
			columnTypes: []string{"unknown"},
			values:      []any{int32(1)},
			want:        "MachAppendData unknown column type 'unknown'",
		},
		{
			name:        "wrong_numeric_type",
			columnNames: []string{"A"},
			columnTypes: []string{"integer"},
			values:      []any{"bad"},
			want:        "MachAppendData cannot apply string to A (integer)",
		},
		{
			name:        "wrong_datetime_type",
			columnNames: []string{"TIME"},
			columnTypes: []string{"datetime"},
			values:      []any{"bad"},
			want:        "MachAppendData cannot apply string to TIME (datetime)",
		},
		{
			name:        "invalid_ipv4",
			columnNames: []string{"ADDR"},
			columnTypes: []string{"ipv4"},
			values:      []any{"not-an-ip"},
			want:        "MachAppendData cannot apply string to ADDR (ipv4)",
		},
		{
			name:        "invalid_ipv6",
			columnNames: []string{"ADDR"},
			columnTypes: []string{"ipv6"},
			values:      []any{"not-an-ip"},
			want:        "MachAppendData cannot apply string to ADDR (ipv6)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buffer := EngMakeAppendBuffer(nil, tc.columnNames, tc.columnTypes)
			err := buffer.Append(tc.values...)
			if err == nil {
				t.Fatal("Append() returned nil error")
			}
			if got := err.Error(); got != tc.want {
				t.Fatalf("Append() error was %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAppendBufferAcceptedConversionsBeforeValidationError(t *testing.T) {
	now := time.Now()
	text := "neo"
	buffer := EngMakeAppendBuffer(nil,
		[]string{
			"SHORT_VALUE",
			"INT_VALUE",
			"LONG_VALUE",
			"FLOAT_VALUE",
			"DOUBLE_VALUE",
			"TIME",
			"IPV4_VALUE",
			"IPV6_VALUE",
			"STR_VALUE",
			"BIN_VALUE",
			"UNKNOWN_VALUE",
		},
		[]string{
			"short",
			"integer",
			"long",
			"float",
			"double",
			"datetime",
			"ipv4",
			"ipv6",
			"varchar",
			"binary",
			"unknown",
		},
	)

	err := buffer.Append(
		int16(1),
		int(2),
		int64(3),
		float32(4.5),
		float64(5.5),
		&now,
		"127.0.0.1",
		"::1",
		&text,
		[]byte("binary"),
		"stop-before-c-call",
	)
	if err == nil {
		t.Fatal("Append() returned nil error")
	}
	if got, want := err.Error(), "MachAppendData unknown column type 'unknown'"; got != want {
		t.Fatalf("Append() error was %q, want %q", got, want)
	}
}
