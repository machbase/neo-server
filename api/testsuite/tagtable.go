package testsuite

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func TagTableAppend(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "tag_data")
	if err != nil {
		t.Fatal(err)
	}

	expectCols := []*api.Column{
		{Name: "NAME", Type: api.ColumnTypeVarchar, Length: 100, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeString},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
	}
	cols, _ := appender.Columns()
	require.Equal(t, len(expectCols), len(cols))
	for i, c := range cols {
		require.Equal(t, expectCols[i].Name, c.Name)
		require.Equal(t, expectCols[i].Type, c.Type, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].DataType, c.DataType, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].Length, c.Length, "diff column: "+c.Name)
	}

	expectCount := 10000
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.Append(
			fmt.Sprintf("name-%d", i%100),   // name
			time.Now(),                      // time
			float64(i)*1.1,                  // value
			int16(i),                        // short_value
			uint16(i*10),                    // ushort_value
			int(i*100),                      // int_value
			uint(i*1000),                    // uint_value
			int64(i*10000),                  // long_value
			uint64(i*100000),                // ulong_value
			varchar,                         // str_value
			fmt.Sprintf("{\"json\":%d}", i), // json_value
			ip4,                             // IPv4_value
			ip6,                             // IPv6_value
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}
