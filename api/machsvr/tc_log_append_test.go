package machsvr_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createLogTable() {
	ctx := context.TODO()
	conn, err := database.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	result := conn.Exec(ctx, SqlTidy(
		`create log table log(
			short short, ushort ushort, 
			integer integer, uinteger uinteger, 
			long long, ulong ulong, 
			float float, double double, 
			ipv4 ipv4, ipv6 ipv6, 
			varchar varchar(20), text text, json json, 
			binary binary, blob blob, clob clob, 
			datetime datetime, 
			datetime_now datetime
		)`))
	if result.Err() != nil {
		panic(result.Err())
	}
}

func TestAppendLog(t *testing.T) {
	ctx := context.TODO()
	conn, err := database.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	pr := conn.QueryRow(ctx, "select count(*) from log")
	if pr.Err() != nil {
		panic(pr.Err())
	}
	var existingCount int
	err = pr.Scan(&existingCount)
	if err != nil {
		panic(err)
	}

	t.Log("---- append log")
	appender, err := conn.Appender(ctx, "log")
	if err != nil {
		panic(err)
	}

	expectCount := 10000

	epochTime := int(time.Now().UnixNano()) - expectCount
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)

		err = appender.AppendWithTimestamp(
			time.Now(),
			int16(i),         // short
			uint16(i*10),     // ushort
			int(i*100),       // int
			uint(i*1000),     // uint
			int64(i*10000),   // long
			uint64(i*100000), // ulong
			float32(i),       // float
			float64(i),       // double
			ip4,              // IPv4
			ip6,              // IPv6
			varchar,
			fmt.Sprintf("text_append-%d-%s.", i, randomVarchar()),
			fmt.Sprintf("{\"json\":%d}", i),
			[]byte(fmt.Sprintf("binary_append_%02d", i)),
			"blob_append",
			"clob_append",
			epochTime+i,
			time.Now())
		if err != nil {
			panic(err)
		}
	}
	sc, fc, err := appender.Close()
	if err != nil {
		panic(err)
	}
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)

	r := conn.QueryRow(ctx, "select count(*) from log")
	if r.Err() != nil {
		panic(r.Err())
	}
	var count int
	err = r.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, expectCount+existingCount, count)

	t.Log("---- append log done")

	row := conn.QueryRow(ctx, "select count(*) from m$sys_tables  where name = ?", "LOG")
	if row.Err() != nil {
		panic(row.Err())
	} else {
		var count int
		err = row.Scan(&count)
		if err != nil {
			t.Logf("ERR-scan: %s\n", err.Error())
		}
		require.Equal(t, 1, count)
	}

	rows, err := conn.Query(ctx, SqlTidy(`
		select
			short, ushort, integer, uinteger, long, ulong, float, double, 
			ipv4, ipv6,
			varchar, text, json, binary, blob, clob, datetime, datetime_now
		from
			log`))
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var _int16 int16
		var _uint16 int16
		var _int32 int32
		var _uint32 int32
		var _int64 int64
		var _uint64 int64
		var _float float32
		var _double float64
		var _ipv4 net.IP
		var _ipv6 net.IP
		var _varchar string
		var _text string
		var _json string
		var _bin []byte
		var _blob []byte
		var _clob []byte
		var _datetime int64
		var _datetime_now time.Time

		err := rows.Scan(
			&_int16, &_uint16, &_int32, &_uint32, &_int64, &_uint64, &_float, &_double,
			&_ipv4, &_ipv6,
			&_varchar, &_text, &_json, &_bin, &_blob, &_clob, &_datetime, &_datetime_now)
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			panic(err)
		}
		// t.Logf("----> %d %d %d %d %d %d %f %f %v %v %s %s %s %v %d %v\n",
		// 	_int16, _uint16, _int32, _uint32, _int64, _uint64, _float, _double,
		// 	_ipv4, _ipv6,
		// 	_varchar, _text, _json, string(_bin),
		// 	_datetime, _datetime_now)
	}
	rows.Close()

	rows, err = conn.Query(ctx, SqlTidy(`
		select 
			short, ushort, integer, uinteger, long, ulong, float, double, varchar, text, json, 
			datetime, datetime_now 
		from 
			log where short = ? and varchar = ?`), 3, "varchar_append-3")
	if err != nil {
		t.Logf("error:%s\n", err.Error())
	}
	passCount := 0
	for rows.Next() {
		var _int16 int16
		var _uint16 int16
		var _int32 int32
		var _uint32 int32
		var _int64 int64
		var _uint64 int64
		var _float float32
		var _double float64
		var _varchar string
		var _text string
		var _json string
		var _datetime int64
		var _datetime_now int64

		err := rows.Scan(
			&_int16, &_uint16, &_int32, &_uint32, &_int64, &_uint64, &_float, &_double,
			&_varchar, &_text, &_json,
			&_datetime, &_datetime_now)
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			break
		}
		// t.Logf("2nd ----> %d %d %d %d %d %d %f %f %s %s %s %d %d\n",
		// 	_int16, _uint16, _int32, _uint32, _int64, _uint64, _float, _double,
		// 	_varchar, _text, _json,
		// 	_datetime, _datetime_now)

		passCount++
	}
	rows.Close()

	require.GreaterOrEqual(t, passCount, 1)
}
