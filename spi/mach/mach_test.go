package mach_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/machbase/neo-server/v8/spi/mach"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
)

var testServer *machsvr.TestServer

var global = struct {
	SvrEnv unsafe.Pointer
}{}

func TestMain(m *testing.M) {
	testServer = &machsvr.TestServer{}
	testServer.StartServer("./tmp")
	global.SvrEnv = testServer.SvrEnv()

	code := m.Run()

	testServer.StopServer()
	os.Exit(code)
}

func TestAll(t *testing.T) {
	createTables()
	tests := []struct {
		name string
		tc   func(t *testing.T)
	}{
		{name: "SvrSimpleTagInsert", tc: SvrSimpleTagInsert},
		{name: "SvrTagTableInsertAndSelect", tc: SvrTagTableInsertAndSelect},
		{name: "SvrAdditionalEngineWrappers", tc: SvrAdditionalEngineWrappers},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.tc(t)
		})
	}
	dropTables()
}

func createTables() {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	// trace_log_level
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, "alter system set trace_log_level=1024")
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)

	// create tag table simple_tag
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `create tag table if not exists simple_tag (name varchar(100) primary key, time datetime basetime, value double)`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)

	// create tag table tag_data
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `
		create tag table tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6
		)
	`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)

	// create log table log_data
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `
		create table log_data(
		    time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary)
	`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)
}

func dropTables() {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	// drop table simple_tag
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `drop table simple_tag`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)

	// drop table tag_data
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `drop table tag_data`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)

	// drop table log_data
	mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	mach.EngAllocStmt(conn, &stmt)
	mach.EngDirectExecute(stmt, `drop table log_data`)
	mach.EngFreeStmt(stmt)
	mach.EngDisconnect(conn)
}

func BenchmarkAll(b *testing.B) {
	benches := []struct {
		name  string
		bench func(*testing.B)
	}{
		{name: "benchSimpleTagInsertDirectExecute", bench: benchSimpleTagInsertDirectExecute},
		{name: "benchSimpleTagInsertExecute", bench: benchSimpleTagInsertExecute},
		{name: "benchSimpleTagInsertExecute", bench: benchSimpleTagInsertExecute},
		{name: "benchSimpleTagAppend", bench: benchSimpleTagAppend},
	}

	createTables()
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			bench.bench(b)
		})
	}
	dropTables()
}

func benchSimpleTagInsertDirectExecute(b *testing.B) {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	err := mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	require.NoError(b, err)
	defer mach.EngDisconnect(conn)

	for i := 0; i < b.N; i++ {
		sqlText := fmt.Sprintf(`insert into simple_tag values('bench-insert', now, %f)`, 1.001*float64(i+1))
		err = mach.EngAllocStmt(conn, &stmt)
		require.NoError(b, err)
		err = mach.EngDirectExecute(stmt, sqlText)
		require.NoError(b, err)
		mach.EngFreeStmt(stmt)
	}
}

func benchSimpleTagInsertExecute(b *testing.B) {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	err := mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	require.NoError(b, err)
	defer mach.EngDisconnect(conn)

	sqlText := `insert into simple_tag values(?, ?, ?)`

	for i := 0; i < b.N; i++ {
		err = mach.EngAllocStmt(conn, &stmt)
		require.NoError(b, err)

		err = mach.EngPrepare(stmt, sqlText)
		require.NoError(b, err)
		err = mach.EngBindString(stmt, 0, "bench-insert")
		require.NoError(b, err)
		err = mach.EngBindInt64(stmt, 1, time.Now().UnixNano())
		require.NoError(b, err)
		err = mach.EngBindFloat64(stmt, 2, 1.001*float64(i+1))
		require.NoError(b, err)
		err = mach.EngExecute(stmt)
		require.NoError(b, err)

		mach.EngFreeStmt(stmt)
	}
}

func benchSimpleTagAppend(b *testing.B) {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	err := mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	require.NoError(b, err)
	defer mach.EngDisconnect(conn)

	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(b, err)

	err = mach.EngAppendOpen(stmt, "simple_tag")
	require.NoError(b, err)

	columnCount, err := mach.EngColumnCount(stmt)
	require.NoError(b, err)
	require.Equal(b, 3, columnCount)

	columnNames := make([]string, columnCount)
	columnTypes := make([]int, columnCount)
	for i := 0; i < columnCount; i++ {
		columnNames[i], err = mach.EngColumnName(stmt, i)
		require.NoError(b, err)
		columnTypes[i], _, err = mach.EngColumnType(stmt, i)
		require.NoError(b, err)
	}
	require.Equal(b, []string{"NAME", "TIME", "VALUE"}, columnNames)
	require.Equal(b, []int{
		int(mach.MACHCLI_SQL_TYPE_STRING),
		int(mach.MACHCLI_SQL_TYPE_DATETIME),
		int(mach.MACHCLI_SQL_TYPE_DOUBLE)}, columnTypes)

	buf := mach.EngMakeAppendBuffer(stmt, columnNames, []string{"string", "datetime", "double"})
	for i := 0; i < b.N; i++ {
		err := buf.Append("bench-append", time.Now().UnixNano(), 1.001*float64(i+1))
		require.NoError(b, err)
	}

	s, f, err := mach.EngAppendClose(stmt)
	require.NoError(b, err)
	require.Equal(b, int64(b.N), s)
	require.Equal(b, int64(0), f)
	mach.EngFreeStmt(stmt)
}

func SvrSimpleTagInsert(t *testing.T) {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	// connect
	err := mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	require.NoError(t, err)
	defer mach.EngDisconnect(conn)

	expectCount := 100_000

	// insert direct_execute
	for i := 0; i < expectCount; i++ {
		sqlText := fmt.Sprintf(`insert into simple_tag values('insert', now, %f)`, 1.001*float64(i+1))
		err = mach.EngAllocStmt(conn, &stmt)
		require.NoError(t, err)
		err = mach.EngDirectExecute(stmt, sqlText)
		require.NoError(t, err)
		mach.EngFreeStmt(stmt)
	}

	// flush
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngDirectExecute(stmt, `EXEC table_flush(simple_tag)`)
	require.NoError(t, err)
	mach.EngFreeStmt(stmt)

	// select count(*) form simple_tag
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngDirectExecute(stmt, `select count(*) from simple_tag where name = 'insert'`)
	require.NoError(t, err)

	// fetch
	next, err := mach.EngFetch(stmt)
	require.NoError(t, err)
	require.True(t, next)

	// get column
	count, valid, err := mach.EngColumnDataInt64(stmt, 0)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, int64(expectCount), count)

	mach.EngFreeStmt(stmt)

	// JOIN tag stat and meta
	//
	// Issue: https://github.com/machbase/neo/issues/889
	//
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)

	err = mach.EngPrepare(stmt, `SELECT m._ID, m.NAME, s.ROW_COUNT FROM _SIMPLE_TAG_META m, V$SIMPLE_TAG_STAT s WHERE s.NAME = m.NAME`)
	require.NoError(t, err)
	err = mach.EngExecute(stmt)
	require.NoError(t, err)

	// fetch
	next, err = mach.EngFetch(stmt)
	require.NoError(t, err)
	require.True(t, next)
	mach.EngFreeStmt(stmt)
}

func SvrTagTableInsertAndSelect(t *testing.T) {
	var conn unsafe.Pointer
	var stmt unsafe.Pointer

	// connect
	err := mach.EngConnectTrust(global.SvrEnv, "sys", &conn)
	require.NoError(t, err)
	defer mach.EngDisconnect(conn)

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)

	// Because INSERT statement uses '2021-01-01 00:00:00' as time value which was parsed in Local timezone,
	// the time value should be converted to UTC timezone to compare
	// TODO: improve this behavior
	nowStrInLocal := now.In(time.Local).Format("2006-01-02 15:04:05")

	// insert
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngPrepare(stmt,
		`insert into tag_data values('insert-once', '`+nowStrInLocal+`', 1.23, `+ // name, time, value
			`?, ?, ?, ?,`+ // short_value, ushort_value, int_value, uint_value
			`?, ?, ?, ?,`+ // long_value, ulong_value, str_value, json_value
			`?, ?)`, // ipv4_value, ipv6_value
	)
	require.NoError(t, err, "insert prepare fail")

	err = mach.EngBindInt32(stmt, 0, 1) // short_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindInt32(stmt, 1, 2) // ushort_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindInt32(stmt, 2, 3) // int_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindInt32(stmt, 3, 4) // uint_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindInt64(stmt, 4, 5) // long_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindInt64(stmt, 5, 6) // ulong_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindString(stmt, 6, "str1") // str_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindString(stmt, 7, `{"key1": "value1"}`) // json_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindString(stmt, 8, net.IPv4(192, 168, 0, 1).String()) // ipv4_value
	require.NoError(t, err, "bind fail")
	err = mach.EngBindString(stmt, 9, net.IPv6loopback.String()) // ipv6_value
	require.NoError(t, err, "bind fail")

	err = mach.EngExecute(stmt)
	require.NoError(t, err, "execute fail")
	err = mach.EngFreeStmt(stmt)
	require.NoError(t, err, "close fail")

	// flush
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngDirectExecute(stmt, `EXEC table_flush(tag_data)`)
	require.NoError(t, err, "table_flush fail")
	err = mach.EngFreeStmt(stmt)
	require.NoError(t, err, "close fail")

	// select
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngPrepare(stmt, `select * from tag_data where name = 'insert-once'`)
	require.NoError(t, err, "select fail")
	err = mach.EngExecute(stmt)
	require.NoError(t, err, "execute fail")
	stmtType, err := mach.EngStmtType(stmt)
	require.NoError(t, err, "stmt type fail")
	require.Equal(t, 512, stmtType)
	columnCount, err := mach.EngColumnCount(stmt)
	require.NoError(t, err, "column count fail")
	require.Equal(t, 13, columnCount)

	var columnName string
	var columnType int
	var columnSize int
	var columnLength int
	err = mach.EngColumnInfo(stmt, 0, &columnName, &columnType, &columnSize, &columnLength)
	require.NoError(t, err, "column info fail")
	require.Equal(t, "NAME", columnName)
	require.Equal(t, int(mach.MACHCLI_SQL_TYPE_STRING), columnType)

	columnName, err = mach.EngColumnName(stmt, 0)
	require.NoError(t, err, "column name fail")
	require.Equal(t, "NAME", columnName)
	columnType, columnSize, err = mach.EngColumnType(stmt, 0)
	require.NoError(t, err, "column type fail")
	require.Equal(t, int(mach.MACHCLI_SQL_TYPE_STRING), columnType)
	require.Greater(t, columnSize, 0)

	next, err := mach.EngFetch(stmt)
	require.NoError(t, err, "fetch fail")
	require.True(t, next, "fetch fail")
	columnLength, err = mach.EngColumnLength(stmt, 0)
	require.NoError(t, err, "column length fail")
	require.Greater(t, columnLength, 0)

	// name
	if v, isValid, err := mach.EngColumnDataString(stmt, 0); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, "insert-once", v)
	}

	// time
	if v, isValid, err := mach.EngColumnDataDateTime(stmt, 1); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, now.In(time.UTC), v.In(time.UTC))
	}

	// value
	if v, isValid, err := mach.EngColumnDataFloat64(stmt, 2); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, 1.23, v)
	}

	// short_value
	if v, isValid, err := mach.EngColumnDataInt16(stmt, 3); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, int16(1), v)
	}

	// ushort_value
	if v, isValid, err := mach.EngColumnDataUInt16(stmt, 4); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, uint16(2), v)
	}

	// int_value
	if v, isValid, err := mach.EngColumnDataInt32(stmt, 5); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, int32(3), v)
	}

	// uint_value
	if v, isValid, err := mach.EngColumnDataUInt32(stmt, 6); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, uint32(4), v)
	}

	// long_value
	if v, isValid, err := mach.EngColumnDataInt64(stmt, 7); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, int64(5), v)
	}

	// ulong_value
	if v, isValid, err := mach.EngColumnDataUInt64(stmt, 8); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, uint64(6), v)
	}

	// str_value
	if v, isValid, err := mach.EngColumnDataString(stmt, 9); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, "str1", v)
	}

	// json_value
	if v, isValid, err := mach.EngColumnDataString(stmt, 10); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, `{"key1": "value1"}`, v)
	}

	// ipv4_value
	if v, isValid, err := mach.EngColumnDataIPv4(stmt, 11); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, net.IPv4(192, 168, 0, 1).To4(), v)
	}

	// ipv6_value
	if v, isValid, err := mach.EngColumnDataIPv6(stmt, 12); err != nil || !isValid {
		require.NoError(t, err, "column data fail")
	} else {
		require.True(t, isValid, "column data fail")
		require.Equal(t, net.IPv6loopback, v)
	}
	err = mach.EngFreeStmt(stmt)
	require.NoError(t, err, "close fail")
}

func SvrAdditionalEngineWrappers(t *testing.T) {
	require.GreaterOrEqual(t, mach.EngConnectionCount(global.SvrEnv), 0)

	ok, err := mach.EngUserAuth(global.SvrEnv, "sys", "manager")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = mach.EngUserAuth(global.SvrEnv, "sys", "wrong-password")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = mach.EngUserAuth(global.SvrEnv, "missing_user", "manager")
	require.NoError(t, err)
	require.False(t, ok)

	var cancelConn unsafe.Pointer
	err = mach.EngConnect(global.SvrEnv, "sys", "manager", &cancelConn)
	require.NoError(t, err)
	require.NoError(t, mach.EngCancel(cancelConn))
	require.NoError(t, mach.EngDisconnect(cancelConn))

	var conn unsafe.Pointer
	err = mach.EngConnect(global.SvrEnv, "sys", "manager", &conn)
	require.NoError(t, err)
	defer mach.EngDisconnect(conn)

	var stmt unsafe.Pointer
	err = mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	err = mach.EngPrepare(stmt, "select * from simple_tag")
	require.NoError(t, err)
	plan, err := mach.EngExplain(stmt, false)
	require.NoError(t, err)
	require.Contains(t, plan, "PROJECT")
	require.NoError(t, mach.EngFreeStmt(stmt))

	prepareInvalidSQL(t, conn)
	executeCleanSimpleSelect(t, conn)
	insertLogWithPreparedBindings(t, conn)
	selectLogBoundValues(t, conn)
	appendSimpleTagWithAppender(t, conn)
}

func prepareInvalidSQL(t *testing.T, conn unsafe.Pointer) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)
	err = mach.EngPrepare(stmt, "select * from missing_table_for_error_coverage")
	require.Error(t, err)
	require.Contains(t, err.Error(), "MACH-ERR")
}

func executeCleanSimpleSelect(t *testing.T, conn unsafe.Pointer) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)
	require.NoError(t, mach.EngPrepare(stmt, "select 1"))
	require.NoError(t, mach.EngExecuteClean(stmt))
}

func insertLogWithPreparedBindings(t *testing.T, conn unsafe.Pointer) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)

	err = mach.EngPrepare(stmt, `insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) `)
	require.NoError(t, err)
	require.NoError(t, mach.EngBindInt64(stmt, 0, time.Now().UnixNano()))
	require.NoError(t, mach.EngBindInt32(stmt, 1, 11))
	require.NoError(t, mach.EngBindInt32(stmt, 2, 12))
	require.NoError(t, mach.EngBindInt32(stmt, 3, 13))
	require.NoError(t, mach.EngBindInt32(stmt, 4, 14))
	require.NoError(t, mach.EngBindInt64(stmt, 5, 15))
	require.NoError(t, mach.EngBindInt64(stmt, 6, 16))
	require.NoError(t, mach.EngBindFloat64(stmt, 7, 17.25))
	require.NoError(t, mach.EngBindFloat64(stmt, 8, 18.5))
	require.NoError(t, mach.EngBindString(stmt, 9, "eng-api-cover"))
	require.NoError(t, mach.EngBindString(stmt, 10, `{"ok":true}`))
	require.NoError(t, mach.EngBindString(stmt, 11, "127.0.0.2"))
	require.NoError(t, mach.EngBindString(stmt, 12, "::2"))
	require.NoError(t, mach.EngBindNull(stmt, 13))
	require.NoError(t, mach.EngBindBinary(stmt, 14, []byte("eng-binary")))
	require.NoError(t, mach.EngExecute(stmt))
}

func selectLogBoundValues(t *testing.T, conn unsafe.Pointer) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)

	err = mach.EngDirectExecute(stmt, "select int_value, float_value, bin_value, text_value from log_data where str_value = 'eng-api-cover'")
	require.NoError(t, err)
	next, err := mach.EngFetch(stmt)
	require.NoError(t, err)
	require.True(t, next)

	var intValue int32
	valid, err := mach.EngColumnData(stmt, 0, unsafe.Pointer(&intValue), 4)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, int32(13), intValue)

	floatValue, valid, err := mach.EngColumnDataFloat32(stmt, 1)
	require.NoError(t, err)
	require.True(t, valid)
	require.InDelta(t, float32(18.5), floatValue, 0.001)

	binaryValue, valid, err := mach.EngColumnDataBinary(stmt, 2)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, []byte("eng-binary"), binaryValue)

	textValue, valid, err := mach.EngColumnDataString(stmt, 3)
	require.NoError(t, err)
	require.False(t, valid)
	require.Empty(t, textValue)
}

func appendSimpleTagWithAppender(t *testing.T, conn unsafe.Pointer) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)

	require.NoError(t, mach.EngAppendOpen(stmt, "simple_tag"))
	buffer := mach.EngMakeAppendBuffer(stmt,
		[]string{"NAME", "TIME", "VALUE"},
		[]string{"string", "datetime", "double"},
	)
	require.NoError(t, buffer.Append(fmt.Sprintf("append-cover-%d", time.Now().UnixNano()), time.Now().UnixNano(), 42.5))
	successCount, failureCount, err := mach.EngAppendClose(stmt)
	require.NoError(t, err)
	require.Equal(t, int64(1), successCount)
	require.Equal(t, int64(0), failureCount)

	flushStmt(t, conn, "simple_tag")
}

func flushStmt(t *testing.T, conn unsafe.Pointer, tableName string) {
	t.Helper()

	var stmt unsafe.Pointer
	err := mach.EngAllocStmt(conn, &stmt)
	require.NoError(t, err)
	defer mach.EngFreeStmt(stmt)
	require.NoError(t, mach.EngDirectExecute(stmt, fmt.Sprintf("EXEC table_flush(%s)", tableName)))
}
