package mach

import (
	"errors"
	"fmt"
	"net"
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

func TestAppendBufferAcceptedValueTypes(t *testing.T) {
	now := time.Unix(1_700_000_000, 123)
	intValue := int(1)
	uintValue := uint(2)
	int16Value := int16(3)
	uint16Value := uint16(4)
	int32Value := int32(5)
	uint32Value := uint32(6)
	int64Value := int64(7)
	uint64Value := uint64(8)
	float32Value := float32(9.5)
	float64Value := float64(10.5)
	text := "neo"
	emptyText := ""

	tests := []struct {
		name       string
		columnType string
		value      any
	}{
		{name: "null", columnType: "short", value: nil},
		{name: "short_uint16", columnType: "short", value: uint16Value},
		{name: "short_uint16_pointer", columnType: "short", value: &uint16Value},
		{name: "short_int16", columnType: "short", value: int16Value},
		{name: "short_int16_pointer", columnType: "short", value: &int16Value},
		{name: "short_uint32", columnType: "short", value: uint32Value},
		{name: "short_uint32_pointer", columnType: "short", value: &uint32Value},
		{name: "short_int32", columnType: "short", value: int32Value},
		{name: "short_int32_pointer", columnType: "short", value: &int32Value},
		{name: "short_float64", columnType: "short", value: float64Value},
		{name: "short_float64_pointer", columnType: "short", value: &float64Value},
		{name: "short_float32", columnType: "short", value: float32Value},
		{name: "short_float32_pointer", columnType: "short", value: &float32Value},
		{name: "integer_int16", columnType: "integer", value: int16Value},
		{name: "integer_int16_pointer", columnType: "integer", value: &int16Value},
		{name: "integer_uint16", columnType: "integer", value: uint16Value},
		{name: "integer_uint16_pointer", columnType: "integer", value: &uint16Value},
		{name: "integer_int32", columnType: "integer", value: int32Value},
		{name: "integer_int32_pointer", columnType: "integer", value: &int32Value},
		{name: "integer_uint32", columnType: "integer", value: uint32Value},
		{name: "integer_uint32_pointer", columnType: "integer", value: &uint32Value},
		{name: "integer_int", columnType: "integer", value: intValue},
		{name: "integer_int_pointer", columnType: "integer", value: &intValue},
		{name: "integer_uint", columnType: "integer", value: uintValue},
		{name: "integer_uint_pointer", columnType: "integer", value: &uintValue},
		{name: "integer_float64", columnType: "integer", value: float64Value},
		{name: "integer_float64_pointer", columnType: "integer", value: &float64Value},
		{name: "integer_float32", columnType: "integer", value: float32Value},
		{name: "integer_float32_pointer", columnType: "integer", value: &float32Value},
		{name: "long_int16", columnType: "long", value: int16Value},
		{name: "long_int16_pointer", columnType: "long", value: &int16Value},
		{name: "long_uint16", columnType: "long", value: uint16Value},
		{name: "long_uint16_pointer", columnType: "long", value: &uint16Value},
		{name: "long_int32", columnType: "long", value: int32Value},
		{name: "long_int32_pointer", columnType: "long", value: &int32Value},
		{name: "long_uint32", columnType: "long", value: uint32Value},
		{name: "long_uint32_pointer", columnType: "long", value: &uint32Value},
		{name: "long_int", columnType: "long", value: intValue},
		{name: "long_int_pointer", columnType: "long", value: &intValue},
		{name: "long_uint", columnType: "long", value: uintValue},
		{name: "long_uint_pointer", columnType: "long", value: &uintValue},
		{name: "long_int64", columnType: "long", value: int64Value},
		{name: "long_int64_pointer", columnType: "long", value: &int64Value},
		{name: "long_uint64", columnType: "long", value: uint64Value},
		{name: "long_uint64_pointer", columnType: "long", value: &uint64Value},
		{name: "long_float64", columnType: "long", value: float64Value},
		{name: "long_float64_pointer", columnType: "long", value: &float64Value},
		{name: "long_float32", columnType: "long", value: float32Value},
		{name: "long_float32_pointer", columnType: "long", value: &float32Value},
		{name: "float_int", columnType: "float", value: intValue},
		{name: "float_int_pointer", columnType: "float", value: &intValue},
		{name: "float_int16", columnType: "float", value: int16Value},
		{name: "float_int16_pointer", columnType: "float", value: &int16Value},
		{name: "float_int32", columnType: "float", value: int32Value},
		{name: "float_int32_pointer", columnType: "float", value: &int32Value},
		{name: "float_int64", columnType: "float", value: int64Value},
		{name: "float_int64_pointer", columnType: "float", value: &int64Value},
		{name: "float_float32", columnType: "float", value: float32Value},
		{name: "float_float32_pointer", columnType: "float", value: &float32Value},
		{name: "double_int", columnType: "double", value: intValue},
		{name: "double_int_pointer", columnType: "double", value: &intValue},
		{name: "double_int16", columnType: "double", value: int16Value},
		{name: "double_int16_pointer", columnType: "double", value: &int16Value},
		{name: "double_int32", columnType: "double", value: int32Value},
		{name: "double_int32_pointer", columnType: "double", value: &int32Value},
		{name: "double_int64", columnType: "double", value: int64Value},
		{name: "double_int64_pointer", columnType: "double", value: &int64Value},
		{name: "double_float32", columnType: "double", value: float32Value},
		{name: "double_float32_pointer", columnType: "double", value: &float32Value},
		{name: "double_float64", columnType: "double", value: float64Value},
		{name: "double_float64_pointer", columnType: "double", value: &float64Value},
		{name: "datetime_time", columnType: "datetime", value: now},
		{name: "datetime_time_pointer", columnType: "datetime", value: &now},
		{name: "datetime_int", columnType: "datetime", value: intValue},
		{name: "datetime_int16", columnType: "datetime", value: int16Value},
		{name: "datetime_int32", columnType: "datetime", value: int32Value},
		{name: "datetime_int64", columnType: "datetime", value: int64Value},
		{name: "datetime_float64", columnType: "datetime", value: float64Value},
		{name: "ipv4_ip", columnType: "ipv4", value: net.IPv4(127, 0, 0, 1)},
		{name: "ipv4_string", columnType: "ipv4", value: "127.0.0.1"},
		{name: "ipv6_ip", columnType: "ipv6", value: net.IPv6loopback},
		{name: "ipv6_string", columnType: "ipv6", value: "::1"},
		{name: "varchar_string", columnType: "varchar", value: text},
		{name: "varchar_empty_string", columnType: "varchar", value: ""},
		{name: "varchar_string_pointer", columnType: "varchar", value: &text},
		{name: "varchar_empty_string_pointer", columnType: "varchar", value: &emptyText},
		{name: "binary_string", columnType: "binary", value: text},
		{name: "binary_empty_string", columnType: "binary", value: ""},
		{name: "binary_string_pointer", columnType: "binary", value: &text},
		{name: "binary_empty_string_pointer", columnType: "binary", value: &emptyText},
		{name: "binary_bytes", columnType: "binary", value: []byte("neo")},
		{name: "binary_empty_bytes", columnType: "binary", value: []byte{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buffer := EngMakeAppendBuffer(nil, []string{"VALUE", "STOP"}, []string{tc.columnType, "unknown"})
			err := buffer.Append(tc.value, true)
			if err == nil {
				t.Fatal("Append() returned nil error")
			}
			if got, want := err.Error(), "MachAppendData unknown column type 'unknown'"; got != want {
				t.Fatalf("Append() error was %q, want %q", got, want)
			}
		})
	}
}
