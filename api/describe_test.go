package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/stretchr/testify/require"
)

func TestDescribeTable(t *testing.T) {
	db := machsvrDatabase(t)

	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// describe table
		desc, err := api.DescribeTable(ctx, conn, table_name, true)
		require.NoError(t, err, "describe table %q fail", table_name)
		require.Equal(t, "TAG_DATA", desc.Name)
		require.Equal(t, "SYS", desc.User)
		require.Equal(t, "MACHBASEDB", desc.Database)
		require.Equal(t, "Tag Table", desc.String())
		require.Equal(t, api.TableTypeTag, desc.Type)
		require.Equal(t, 9, len(desc.Columns))
		require.Equal(t, "NAME", desc.Columns[0].Name)
		require.Equal(t, "TIME", desc.Columns[1].Name)
		require.Equal(t, "VALUE", desc.Columns[2].Name)
		require.Equal(t, "SHORT_VALUE", desc.Columns[3].Name)
		require.Equal(t, "INT_VALUE", desc.Columns[4].Name)
		require.Equal(t, "LONG_VALUE", desc.Columns[5].Name)
		require.Equal(t, "STR_VALUE", desc.Columns[6].Name)
		require.Equal(t, "JSON_VALUE", desc.Columns[7].Name)
		require.Equal(t, "_RID", desc.Columns[8].Name)
		require.Equal(t, api.ColumnTypeVarchar, desc.Columns[0].Type)    // NAME
		require.Equal(t, api.ColumnTypeDatetime, desc.Columns[1].Type)   // TIME
		require.Equal(t, api.ColumnTypeDouble, desc.Columns[2].Type)     // VALUE
		require.Equal(t, api.ColumnTypeShort, desc.Columns[3].Type)      // SHORT_VALUE
		require.Equal(t, api.ColumnTypeInteger, desc.Columns[4].Type)    // INT_VALUE
		require.Equal(t, api.ColumnTypeLong, desc.Columns[5].Type)       // LONG_VALUE
		require.Equal(t, api.ColumnTypeVarchar, desc.Columns[6].Type)    // STR_VALUE
		require.Equal(t, api.ColumnTypeJson, desc.Columns[7].Type)       // JSON_VALUE
		require.Equal(t, api.ColumnTypeLong, desc.Columns[8].Type)       // _RID
		require.Equal(t, api.DataTypeString, desc.Columns[0].DataType)   // NAME
		require.Equal(t, api.DataTypeDatetime, desc.Columns[1].DataType) // TIME
		require.Equal(t, api.DataTypeFloat64, desc.Columns[2].DataType)  // VALUE
		require.Equal(t, api.DataTypeInt16, desc.Columns[3].DataType)    // SHORT_VALUE
		require.Equal(t, api.DataTypeInt32, desc.Columns[4].DataType)    // INT_VALUE
		require.Equal(t, api.DataTypeInt64, desc.Columns[5].DataType)    // LONG_VALUE
		require.Equal(t, api.DataTypeString, desc.Columns[6].DataType)   // STR_VALUE
		require.Equal(t, api.DataTypeString, desc.Columns[7].DataType)   // JSON_VALUE
		require.Equal(t, api.DataTypeInt64, desc.Columns[8].DataType)    // _RID

		if table_name != "tag_data" {
			continue
		}
		buf := &bytes.Buffer{}
		json.NewEncoder(buf).Encode(desc)
		//t.Log(buf.String())
		m := make(map[string]interface{})
		json.Unmarshal(buf.Bytes(), &m)

		require.Equal(t, "TAG_DATA", m["name"])
		require.Equal(t, "SYS", m["user"])
		require.Equal(t, "MACHBASEDB", m["database"])
		require.Equal(t, "TagTable", m["type"])
		require.Equal(t, 9, len(m["columns"].([]interface{})))
		columns := m["columns"].([]interface{})
		col0 := columns[0].(map[string]interface{})
		col1 := columns[1].(map[string]interface{})
		col2 := columns[2].(map[string]interface{})
		col3 := columns[3].(map[string]interface{})
		col4 := columns[4].(map[string]interface{})
		col5 := columns[5].(map[string]interface{})
		col6 := columns[6].(map[string]interface{})
		col7 := columns[7].(map[string]interface{})
		col8 := columns[8].(map[string]interface{})
		require.Equal(t, "NAME", col0["name"])
		require.Equal(t, "TIME", col1["name"])
		require.Equal(t, "VALUE", col2["name"])
		require.Equal(t, "SHORT_VALUE", col3["name"])
		require.Equal(t, "INT_VALUE", col4["name"])
		require.Equal(t, "LONG_VALUE", col5["name"])
		require.Equal(t, "STR_VALUE", col6["name"])
		require.Equal(t, "JSON_VALUE", col7["name"])
		require.Equal(t, "_RID", col8["name"])
		require.Equal(t, "varchar", col0["type"])
		require.Equal(t, "datetime", col1["type"])
		require.Equal(t, "double", col2["type"])
		require.Equal(t, "short", col3["type"])
		require.Equal(t, "integer", col4["type"])
		require.Equal(t, "long", col5["type"])
		require.Equal(t, "varchar", col6["type"])
		require.Equal(t, "json", col7["type"])
		require.Equal(t, "long", col8["type"])
	}

	desc, err := api.DescribeTable(ctx, conn, "m$sys_tables", false)
	require.NoError(t, err, "describe m$sys_tables fail")
	require.Equal(t, "M$SYS_TABLES", desc.Name)
}
