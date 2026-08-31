package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
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

func TestDecodeQueryRequestJSONNormalizesNamedParams(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t where a = :name","p":{"name":"neo"}}`))
	require.NoError(t, err)
	require.Equal(t, "select * from t where a = :name", req.SqlText)
	require.Equal(t, []any{sql.Named("name", "neo")}, req.Params)
}

func TestDecodeQueryRequestJSONRejectsCompositeNamedParam(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t","p":{"name":["neo"]}}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
	require.Contains(t, err.Error(), "scalar")
}

func TestDecodeQueryRequestJSONDB(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select 1","db":"testdb"}`))
	require.NoError(t, err)
	require.Equal(t, "testdb", req.DB)
}

func TestDecodeQueryRequestQueryDB(t *testing.T) {
	req := &QueryRequest{}
	ctx, _ := newTestHTTPContext(http.MethodGet, "/db/query?q=select+1&db=testdb", nil)
	err := req.DecodeQuery(ctx)
	require.NoError(t, err)
	require.Equal(t, "testdb", req.DB)
}

func TestDecodeQueryRequestPostFormDB(t *testing.T) {
	req := &QueryRequest{}
	ctx, _ := newTestHTTPContext(http.MethodPost, "/db/query", []byte("q=select+1&db=testdb"))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	err := req.DecodePostForm(ctx)
	require.NoError(t, err)
	require.Equal(t, "testdb", req.DB)
}

func TestValidateDatabaseName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple", input: "testdb"},
		{name: "with underscore and digits", input: "_test_db2"},
		{name: "empty", input: "", wantErr: true},
		{name: "starts with digit", input: "2db", wantErr: true},
		{name: "contains space", input: "test db", wantErr: true},
		{name: "contains quote", input: `test"db`, wantErr: true},
		{name: "contains semicolon", input: "testdb;drop table x", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 41), wantErr: true},
		{name: "max length", input: strings.Repeat("a", 40)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDatabaseName(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseQueryParams(t *testing.T) {
	params, err := parseQueryParams("   ")
	require.NoError(t, err)
	require.Nil(t, params)

	params, err = parseQueryParams(`[1,2.5,false,"x"]`)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), 2.5, false, "x"}, params)

	params, err = parseQueryParams(`{"name":"neo","from":1,"to":2.5}`)
	require.NoError(t, err)
	require.Equal(t, []any{sql.Named("from", int64(1)), sql.Named("name", "neo"), sql.Named("to", 2.5)}, params)

	_, err = parseQueryParams(`{"nested":["x"]}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")

	_, err = parseQueryParams(`123`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
}

func TestNormalizeQueryParams(t *testing.T) {
	params, err := normalizeQueryParams(nil)
	require.NoError(t, err)
	require.Nil(t, params)

	params, err = normalizeQueryParams([]any{})
	require.NoError(t, err)
	require.Nil(t, params)

	params, err = normalizeQueryParams(map[string]any{})
	require.NoError(t, err)
	require.Nil(t, params)

	_, err = normalizeQueryParams("not an array or object")
	require.Error(t, err)
	require.Contains(t, err.Error(), "p must be an array or object")
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
		name               string
		databaseType       string
		scanType           string
		nullable           bool
		length             int64
		supportDecimalSize bool
		decimalPrecision   int64
		decimalScale       int64
	}{
		{name: "NAME", databaseType: "VARCHAR", scanType: "string", nullable: false, length: 100},
		{name: "TIME", databaseType: "DATETIME", scanType: "time.Time", nullable: false, length: 8},
		{name: "VALUE", databaseType: "DOUBLE", scanType: "float64", nullable: false, length: 8,
			supportDecimalSize: true, decimalPrecision: 8, decimalScale: 0},
		{name: "SHORT_VALUE", databaseType: "SHORT", scanType: "int16", nullable: true, length: 2},
		{name: "USHORT_VALUE", databaseType: "USHORT", scanType: "uint16", nullable: true, length: 2},
		{name: "INT_VALUE", databaseType: "INTEGER", scanType: "int32", nullable: true, length: 4},
		{name: "UINT_VALUE", databaseType: "UINTEGER", scanType: "uint32", nullable: true, length: 4},
		{name: "LONG_VALUE", databaseType: "LONG", scanType: "int64", nullable: true, length: 8},
		{name: "ULONG_VALUE", databaseType: "ULONG", scanType: "uint64", nullable: true, length: 8},
		{name: "STR_VALUE", databaseType: "VARCHAR", scanType: "string", nullable: true, length: 400},
		{name: "JSON_VALUE", databaseType: "JSON", scanType: "api.JSONString", nullable: true, length: 32767},
		{name: "IPV4_VALUE", databaseType: "IPV4", scanType: "net.IP", nullable: true, length: 5},
		{name: "IPV6_VALUE", databaseType: "IPV6", scanType: "net.IP", nullable: true, length: 17},
		{name: "BIN_VALUE", databaseType: "BINARY", scanType: "[]uint8", nullable: true, length: 32767},
	}
	require.Equal(t, len(expects), len(columnTypes))
	for i, ct := range columnTypes {
		nullable, supportNullable := ct.Nullable()
		length, supportLength := ct.Length()
		decimalPrecision, decimalScale, supportDecimalSize := ct.DecimalSize()
		require.True(t, supportNullable, "column %s does not support Nullable()", ct.Name())
		require.True(t, supportLength, "column %s does not support Length()", ct.Name())
		require.Equal(t, expects[i].supportDecimalSize, supportDecimalSize, "column %s does not support DecimalSize()", ct.Name())
		require.Equal(t, expects[i].name, ct.Name(), "column %s", ct.Name())
		require.Equal(t, expects[i].databaseType, ct.DatabaseTypeName(), "column %s", ct.Name())
		require.Equal(t, expects[i].scanType, ct.ScanType().String(), "column %s", ct.Name())
		require.Equal(t, expects[i].nullable, nullable, "column %s", ct.Name())
		require.Equal(t, expects[i].length, length, "column %s", ct.Name())
		require.Equal(t, expects[i].decimalPrecision, decimalPrecision, "column %s", ct.Name())
		require.Equal(t, expects[i].decimalScale, decimalScale, "column %s", ct.Name())
	}
}
