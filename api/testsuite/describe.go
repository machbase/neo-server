package testsuite

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func DescribeTable(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	expect := api.Columns{
		{Name: "NAME", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, DataType: api.DataTypeInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, DataType: api.DataTypeInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, DataType: api.DataTypeInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, DataType: api.DataTypeString},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, DataType: api.DataTypeIPv6},
		{Name: "_RID", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
	}

	expectColumns := []map[string]interface{}{
		{"name": "NAME", "type": "varchar", "data_type": "string", "length": 100, "flag": api.ColumnFlagTagName},
		{"name": "TIME", "type": "datetime", "data_type": "datetime", "length": 8, "flag": api.ColumnFlagBasetime},
		{"name": "VALUE", "type": "double", "data_type": "double", "length": 8, "flag": api.ColumnFlagSummarized},
		{"name": "SHORT_VALUE", "type": "short", "data_type": "int16", "length": 2},
		{"name": "USHORT_VALUE", "type": "ushort", "data_type": "int16", "length": 2},
		{"name": "INT_VALUE", "type": "integer", "data_type": "int32", "length": 4},
		{"name": "UINT_VALUE", "type": "uinteger", "data_type": "int32", "length": 4},
		{"name": "LONG_VALUE", "type": "long", "data_type": "int64", "length": 8},
		{"name": "ULONG_VALUE", "type": "ulong", "data_type": "int64", "length": 8},
		{"name": "STR_VALUE", "type": "varchar", "data_type": "string", "length": 400},
		{"name": "JSON_VALUE", "type": "json", "data_type": "string", "length": 32767},
		{"name": "IPV4_VALUE", "type": "ipv4", "data_type": "ipv4", "length": 5},
		{"name": "IPV6_VALUE", "type": "ipv6", "data_type": "ipv6", "length": 17},
		{"name": "_RID", "type": "long", "data_type": "int64", "length": 8},
	}
	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// describe table
		desc, err := api.DescribeTable(ctx, conn, table_name, true)
		require.NoError(t, err, "describe table %q fail", table_name)
		require.Equal(t, "TAG_DATA", desc.Name)
		require.Equal(t, "SYS", desc.User)
		require.Equal(t, "MACHBASEDB", desc.Database)
		require.Equal(t, "Tag Table", desc.String())
		require.Equal(t, api.TableTypeTag, desc.Type)

		require.Equal(t, len(expect), len(desc.Columns))

		for i, e := range expect {
			require.Equal(t, e.Name, desc.Columns[i].Name)
			require.Equal(t, e.Type, desc.Columns[i].Type)
			require.Equal(t, e.DataType, desc.Columns[i].DataType)
		}

		if table_name != "tag_data" {
			continue
		}

		buf := &bytes.Buffer{}
		json.NewEncoder(buf).Encode(desc)

		m := make(map[string]interface{})
		json.Unmarshal(buf.Bytes(), &m)

		require.Equal(t, "TAG_DATA", m["name"])
		require.Equal(t, "SYS", m["user"])
		require.Equal(t, "MACHBASEDB", m["database"])
		require.Equal(t, "TagTable", m["type"])
		require.Equal(t, 14, len(m["columns"].([]interface{})))

		columns := m["columns"].([]interface{})

		for i, e := range expectColumns {
			col := columns[i].(map[string]interface{})
			col["length"] = int(col["length"].(float64))
			if flag, ok := col["flag"]; ok {
				col["flag"] = int(flag.(float64))
			}
			// copy actual id to expected id, just for comparison
			if floatId, ok := col["id"]; ok {
				e["id"] = int(floatId.(float64))
				col["id"] = int(floatId.(float64))
			}
			require.Equal(t, e, col)
		}
	}

	desc, err := api.DescribeTable(ctx, conn, "m$sys_tables", false)
	require.NoError(t, err, "describe m$sys_tables fail")
	require.Equal(t, "M$SYS_TABLES", desc.Name)
}
