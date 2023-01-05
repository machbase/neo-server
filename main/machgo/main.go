package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods"
)

func main() {
	fmt.Println("-------------------------------")
	fmt.Println(mach.LinkInfo(), mods.VersionString())

	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	homePath := filepath.Join(filepath.Dir(exePath), "machbase")
	mach.Initialize(homePath)

	if mach.ExistsDatabase() {
		mach.DestroyDatabase()
	}
	mach.CreateDatabase()

	db := mach.New()
	if db == nil {
		panic(err)
	}
	err = db.Startup()
	if err != nil {
		panic(err)
	}
	defer db.Shutdown()

	_, err = db.Exec("alter system set trace_log_level=1023")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(db.SqlTidy(
		`create log table log(
			short short, ushort ushort, integer integer, uinteger uinteger, long long, ulong ulong, float float, double double, 
			ipv4 ipv4, ipv6 ipv6, varchar varchar(20), text text, json json, binary binary, blob blob, clob clob, 
			datetime datetime, datetime_now datetime
		)`))
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		0, 1, 2, 3, 4, 5, 6.6, 7.77,
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"),
		fmt.Sprintf("varchar_1_%s.", randomVarchar()),
		"text_1", "{\"json\":1}", []byte("binary_00"), "blob_01", "clob_01", 1, time.Now())
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		1, 1, 2, 3, 4, 5, 6.6, 7.77,
		net.ParseIP("127.0.0.2"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:DD"),
		fmt.Sprintf("varchar_2_%s.", randomVarchar()),
		"text_2", "{\"json\":1}", []byte("binary_01"), "blob_01", "clob_01", 1, time.Now())
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("insert into log values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		2, 1, 2, 3, 4, 5, 6.6, 7.77,
		net.ParseIP("127.0.0.3"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:AA"),
		fmt.Sprintf("varchar_3_%s.", randomVarchar()),
		"text_3", "{\"json\":2}", []byte("binary_02"), "blob_01", "clob_01", 1, time.Now())
	if err != nil {
		panic(err)
	}

	fmt.Println("---- insert done")
	appender, err := db.Appender("log")
	if err != nil {
		panic(err)
	}
	defer appender.Close()

	for i := 3; i < 10; i++ {
		err = appender.Append(
			int16(i),         // short
			uint16(i*10),     // ushort
			int(i*100),       // int
			uint(i*1000),     // uint
			int64(i*10000),   // long
			uint64(i*100000), // ulong
			float32(i),       // float
			float64(i),       // double
			net.ParseIP(fmt.Sprintf("192.168.0.%d", i)),              // IPv4
			net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i)), // IPv6
			fmt.Sprintf("varchar_append-%d", i),
			fmt.Sprintf("text_append-%d-%s.", i, randomVarchar()),
			fmt.Sprintf("{\"json\":%d}", i),
			[]byte(fmt.Sprintf("binary_append_%02d", i)),
			"blob_append",
			"clob_append",
			i*10000000000,
			time.Now())
		if err != nil {
			panic(err)
		}
	}
	err = appender.Close()
	if err != nil {
		panic(err)
	}
	fmt.Println("---- append done")

	row := db.QueryRow("select count(*) from m$sys_tables  where name = ?", "LOG")
	if row.Err() != nil {
		fmt.Printf("ERR-query: %s\n", row.Err().Error())
	} else {
		var count int
		err = row.Scan(&count)
		if err != nil {
			fmt.Printf("ERR-scan: %s\n", err.Error())
		} else {
			fmt.Printf("============> table 'log' exists=%v\n", count)
		}
	}

	rows, err := db.Query(db.SqlTidy(`
		select
			short, ushort, integer, uinteger, long, ulong, float, double, 
			ipv4, ipv6,
			varchar, text, json, binary, datetime, datetime_now
		from
			log`))
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
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
		var _datetime int64
		var _datetime_now time.Time

		err := rows.Scan(
			&_int16, &_uint16, &_int32, &_uint32, &_int64, &_uint64, &_float, &_double,
			&_ipv4, &_ipv6,
			&_varchar, &_text, &_json, &_bin, &_datetime, &_datetime_now)
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			panic(err)
		}
		fmt.Printf("1st ----> %d %d %d %d %d %d %f %f %v %v %s %s %s %v %d %v\n",
			_int16, _uint16, _int32, _uint32, _int64, _uint64, _float, _double,
			_ipv4, _ipv6,
			_varchar, _text, _json, string(_bin),
			_datetime, _datetime_now)
	}
	rows.Close()

	rows, err = db.Query(db.SqlTidy(`
		select 
			short, ushort, integer, uinteger, long, ulong, float, double, varchar, text, json, 
			datetime, datetime_now 
		from 
			log where short = ? and varchar = ?`), 0, "varchar_1")
	if err != nil {
		fmt.Printf("error:%s\n", err.Error())
	}
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
		fmt.Printf("2nd ----> %d %d %d %d %d %d %f %f %s %s %s %d %d\n",
			_int16, _uint16, _int32, _uint32, _int64, _uint64, _float, _double,
			_varchar, _text, _json,
			_datetime, _datetime_now)
	}
	rows.Close()

	// // signal handler
	// fmt.Printf("\npress ^C to quit.\n")
	// quitChan := make(chan os.Signal)
	// signal.Notify(quitChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// // wait signal
	// <-quitChan

	fmt.Println("-------------------------------")
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
