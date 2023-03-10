package test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/machbase/neo-grpc/driver"
	"github.com/stretchr/testify/require"
)

func TestDriver(t *testing.T) {
	t.Logf("Drivers=%#v", sql.Drivers())

	db, err := sql.Open("machbase", "unix://../tmp/mach.sock")
	if err != nil {
		panic(err)
	}
	require.NotNil(t, db)

	var tableName = strings.ToUpper("tagdata")
	var count int

	row := db.QueryRow("select count(*) from M$SYS_TABLES where name = ?", tableName)
	if row.Err() != nil {
		panic(row.Err())
	}
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}

	if count == 0 {
		sqlText := fmt.Sprintf(`
			create tag table %s ( 
				name            varchar(200) primary key, 
				time            datetime basetime, 
				value           double summarized, 
				type            varchar(40),
				ivalue          long,
				svalue          varchar(400),
				id              varchar(80), 
				pname           varchar(80),
				sampling_period long,
				payload         json
			)`, tableName)
		_, err := db.Exec(sqlText)
		if err != nil {
			panic(err)
		}

		row := db.QueryRow("select count(*) from M$SYS_TABLES where name = ?", tableName)
		if row.Err() != nil {
			panic(row.Err())
		}
		err = row.Scan(&count)
		if err != nil {
			panic(err)
		}
	}
	require.Equal(t, 1, count)

	expectCount := 10000
	ts := time.Now()
	for i := 0; i < expectCount; i++ {
		result, err := db.Exec("insert into "+tableName+" (name, time, value, id) values(?, ?, ?, ?)",
			fmt.Sprintf("name-%d", count%5),
			ts.Add(time.Duration(i)),
			0.1001+0.1001*float32(count),
			fmt.Sprintf("id-%08d", i))
		if err != nil {
			panic(err)
		}
		require.Nil(t, err)
		nrows, _ := result.RowsAffected()
		require.Equal(t, int64(1), nrows)
	}

	rows, err := db.Query("select name, time, value, id from "+tableName+" where time >= ? order by time", ts)
	if err != nil {
		panic(err)
	}
	pass := 0
	for rows.Next() {
		var name string
		var ts time.Time
		var value float64
		var id string
		err := rows.Scan(&name, &ts, &value, &id)
		if err != nil {
			t.Logf("ERR> %v", err.Error())
			break
		}
		require.Equal(t, fmt.Sprintf("name-%d", count%5), name)
		pass++
		//t.Logf("==> %v %v %v %v", name, ts, value, id)
	}
	rows.Close()
	require.Equal(t, expectCount, pass)

	r := db.QueryRow("select count(*) from "+tableName+" where time >= ?", ts)
	r.Scan(&count)
	require.Equal(t, expectCount, count)
	t.Logf("DB=%#v", db.Stats())
}
