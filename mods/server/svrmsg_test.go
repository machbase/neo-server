package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeQueryRequestJSONNormalizesParams(t *testing.T) {
	req := &QueryRequest{}
	err := decodeQueryRequestJSON(strings.NewReader(`{"q":"select * from t where a = ?","p":[1,1.5,true,"neo"]}`), req)
	require.NoError(t, err)
	require.Equal(t, "select * from t where a = ?", req.SqlText)
	require.Equal(t, []any{int64(1), 1.5, true, "neo"}, req.Params)
}

func TestDecodeQueryRequestJSONRejectsCompositeParam(t *testing.T) {
	req := &QueryRequest{}
	err := decodeQueryRequestJSON(strings.NewReader(`{"q":"select * from t","p":[{"nested":1}]}`), req)
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
