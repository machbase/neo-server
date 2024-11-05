package testsuite

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/stretchr/testify/require"
)

func Columns(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.Query(ctx, "select * from log_data limit 10")
	if err != nil {
		t.Fatal(err)
	}
	require.NotNil(t, rows, "no rows selected")
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	data := []struct {
		name   string
		typ    string
		size   int
		length int
	}{
		{"TIME", "datetime", 8, 0},
		{"SHORT_VALUE", "int16", 2, 0},
		{"USHORT_VALUE", "int16", 2, 0},
		{"INT_VALUE", "int32", 4, 0},
		{"UINT_VALUE", "int32", 4, 0},
		{"LONG_VALUE", "int64", 8, 0},
		{"ULONG_VALUE", "int64", 8, 0},
		{"DOUBLE_VALUE", "double", 8, 0},
		{"FLOAT_VALUE", "float", 4, 0},
		{"STR_VALUE", "string", 20, 0},
		{"JSON_VALUE", "string", 32767, 0},
		{"IPV4_VALUE", "ipv4", 5, 0},
		{"IPV6_VALUE", "ipv6", 17, 0},
		{"TEXT_VALUE", "string", 67108864, 0},
		{"BIN_VALUE", "binary", 67108864, 0},
	}
	require.Equal(t, len(data), len(cols), "column count was %d, want %d", len(cols), len(data))
	for i, cd := range data {
		require.Equal(t, cd.name, cols[i].Name, "column[%d] name was %q, want %q", i, cols[i].Name, cd.name)
		require.Equal(t, cd.typ, string(cols[i].DataType), "column[%d] %q's type was %q, want %q", i, cols[i].Name, cols[i].DataType, cd.typ)
	}
}
