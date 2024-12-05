package testsuite

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"math/rand"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func LogTableExec(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	var one int = 1
	var two int = 2
	var three int16 = 3
	var four int16 = 4
	var five int32 = 5
	var f32 float32 = 6.6
	var f64 float64 = 7.77
	var tick time.Time = time.Now()

	result := conn.Exec(ctx, "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick,   // time
		0, one, // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":1}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_1_%s.", randomVarchar()), // text_value
		[]byte("binary_00"),                           // bin_value
	)
	if err := result.Err(); err != nil {
		t.Fatal(err)
	}
	result = conn.Exec(ctx, "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick.Add(1), // time
		0, one,      // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":2}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_2_%s.", randomVarchar()), // text_value
		[]byte("binary_01"),                           // bin_value
	)
	if err := result.Err(); err != nil {
		t.Fatal(err)
	}
}

func LogTableAppend(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "log_data")
	require.NoError(t, err)

	expectCols := []*api.Column{
		{Name: "_ARRIVAL_TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "DOUBLE_VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "FLOAT_VALUE", Type: api.ColumnTypeFloat, Length: 4, DataType: api.DataTypeFloat32},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeString},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
		{Name: "TEXT_VALUE", Type: api.ColumnTypeText, Length: 67108864, DataType: api.DataTypeString},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, Length: 67108864, DataType: api.DataTypeBinary},
	}
	cols, _ := appender.Columns()
	require.Equal(t, len(expectCols), len(cols), strings.Join(cols.Names(), ", "))
	for i, col := range cols {
		require.Equal(t, expectCols[i].Name, col.Name)
		require.Equal(t, expectCols[i].Type, col.Type, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].DataType, col.DataType, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].Length, col.Length, "diff column: "+col.Name)
	}

	expectCount := 10000
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.AppendLogTime(
			time.Now(),                      // _arrival_time
			time.Now(),                      // time
			int16(i),                        // short
			uint16(i*10),                    // ushort
			int(i*100),                      // int
			uint(i*1000),                    // uint
			int64(i*10000),                  // long
			uint64(i*100000),                // ulong
			float64(i),                      // double
			float32(i),                      // float
			varchar,                         // varchar
			fmt.Sprintf("{\"json\":%d}", i), // json
			ip4,                             // IPv4
			ip6,                             // IPv6
			fmt.Sprintf("text_append-%d-%s.", i, randomVarchar()),
			[]byte(fmt.Sprintf("binary_append_%02d", i)),
		)
		require.NoError(t, err)
	}
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset)-1)]
	}
	return string(b)
}

func randomVarchar() string {
	rangeStart := 0
	rangeEnd := 10
	offset := rangeEnd - rangeStart
	randLength := seededRand.Intn(offset) + rangeStart

	charSet := "aAbBcCdDeEfFgGhHiIjJkKlLmMnNoOpPqQrRsStTuUvVwWxXyYzZ"
	randString := StringWithCharset(randLength, charSet)
	return randString
}
