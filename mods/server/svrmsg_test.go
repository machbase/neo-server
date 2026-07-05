package server

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeQueryRequestJSONNormalizesParams(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t where a = ?","p":[1,1.5,true,"neo"],"binaryformat":"base64"}`))
	require.NoError(t, err)
	require.Equal(t, "select * from t where a = ?", req.SqlText)
	require.Equal(t, []any{int64(1), 1.5, true, "neo"}, req.Params)
	require.Equal(t, "base64", req.BinaryFormat)
}

func TestDecodeQueryRequestJSONRejectsCompositeParam(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t","p":[{"nested":1}]}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
	require.Contains(t, err.Error(), "scalar")
}

func TestParseQueryParams(t *testing.T) {
	params, err := parseQueryParams("   ")
	require.NoError(t, err)
	require.Nil(t, params)

	params, err = parseQueryParams(`[1,2.5,false,"x"]`)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), 2.5, false, "x"}, params)

	_, err = parseQueryParams(`{"not":"an array"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
}

func TestNormalizeQueryParamValue(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    any
		wantErr string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "string", input: "neo", want: "neo"},
		{name: "bool", input: true, want: true},
		{name: "int", input: 3, want: 3},
		{name: "float", input: 1.25, want: 1.25},
		{name: "json integer", input: json.Number("42"), want: int64(42)},
		{name: "json float", input: json.Number("3.14"), want: 3.14},
		{name: "invalid json number", input: json.Number("nope"), wantErr: "invalid syntax"},
		{name: "slice", input: []any{"x"}, wantErr: "scalar"},
		{name: "map", input: map[string]any{"x": 1}, wantErr: "scalar"},
		{name: "struct", input: struct{ Value string }{Value: "x"}, wantErr: "unsupported bind parameter type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeQueryParamValue(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSqlRowsScanTypes(t *testing.T) {
	dsn := "server=127.0.0.1:15656;user=sys;password=manager;fetch_rows=100"
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	require.NoError(t, err)
	defer conn.Close()

	rows, err := conn.QueryContext(t.Context(), "select * from TAG_DATA")
	require.NoError(t, err)
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	expects := []struct {
		name         string
		databaseType string
		scanType     string
		nullable     bool
		length       int64
		decimalSize  int64
	}{
		{name: "NAME", databaseType: "VARCHAR", scanType: "string", nullable: true, length: 100},                //
		{name: "TIME", databaseType: "DATETIME", scanType: "time.Time", nullable: true, length: 8},              // length??
		{name: "VALUE", databaseType: "DOUBLE", scanType: "float64", nullable: true, length: 8, decimalSize: 8}, //
		{name: "SHORT_VALUE", databaseType: "SHORT", scanType: "int16", nullable: true, length: 2},              //
		{name: "USHORT_VALUE", databaseType: "USHORT", scanType: "uint16", nullable: true, length: 2},           //
		{name: "INT_VALUE", databaseType: "INTEGER", scanType: "int32", nullable: true, length: 4},              //
		{name: "UINT_VALUE", databaseType: "UINTEGER", scanType: "uint32", nullable: true, length: 4},           //
		{name: "LONG_VALUE", databaseType: "LONG", scanType: "int64", nullable: true, length: 8},                //
		{name: "ULONG_VALUE", databaseType: "ULONG", scanType: "uint64", nullable: true, length: 8},             //
		{name: "STR_VALUE", databaseType: "VARCHAR", scanType: "string", nullable: true, length: 400},           //
		{name: "JSON_VALUE", databaseType: "JSON", scanType: "api.JSONString", nullable: true, length: 32767},   //
		{name: "IPV4_VALUE", databaseType: "IPV4", scanType: "net.IP", nullable: true, length: 5},               //
		{name: "IPV6_VALUE", databaseType: "IPV6", scanType: "net.IP", nullable: true, length: 17},              //
		{name: "BIN_VALUE", databaseType: "BINARY", scanType: "[]uint8", nullable: true, length: 32767},         //
	}
	require.Equal(t, len(expects), len(columnTypes))
	for i, ct := range columnTypes {
		nullable, _ := ct.Nullable()
		length, _ := ct.Length()
		decimalSize, _, _ := ct.DecimalSize()
		require.Equal(t, expects[i].name, ct.Name(), "column %s", ct.Name())
		require.Equal(t, expects[i].databaseType, ct.DatabaseTypeName(), "column %s", ct.Name())
		require.Equal(t, expects[i].scanType, ct.ScanType().String(), "column %s", ct.Name())
		require.Equal(t, expects[i].nullable, nullable, "column %s", ct.Name())
		require.Equal(t, expects[i].length, length, "column %s", ct.Name())
		require.Equal(t, expects[i].decimalSize, decimalSize, "column %s", ct.Name())
	}
}
