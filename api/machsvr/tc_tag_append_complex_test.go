package machsvr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createTagTable() {
	ctx := context.TODO()
	conn, err := db.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	result := conn.Exec(ctx, SqlTidy(
		`create tag table complex_tag(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double,
			type            varchar(40),
			ivalue          long,
			svalue          varchar(400),
			id              varchar(80), 
			pname           varchar(80),
			sampling_period long,
			payload         json
		)`))
	if result.Err() != nil {
		panic(result.Err())
	}
}

func TestAppendTagComplex(t *testing.T) {
	t.Logf("---- append complex_tag [%d]", goid())

	ctx := context.TODO()
	conn, err := db.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	pr := conn.QueryRow(ctx, "select count(*) from complex_tag")
	if pr.Err() != nil {
		panic(pr.Err())
	}
	var existingCount int
	err = pr.Scan(&existingCount)
	if err != nil {
		panic(err)
	}

	appender, err := conn.Appender(ctx, "complex_tag")
	if err != nil {
		panic(err)
	}
	t.Logf("---- %v (%v)", appender.TableName(), appender.TableType())

	expectCount := 100000
	ts := time.Now()
	for i := 0; i < expectCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%10),
			ts.Add(time.Duration(i)),
			1.001*float64(i+1),
			"float64",
			int64(i),
			fmt.Sprintf("svalue-%d", i),
			"some-id-string",
			"pname",
			int64(0),
			`{"t":"json"}`)
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

	rows, err := conn.Query(ctx, `
		select 
			name, time, value, type, ivalue, pname, payload 
		from
			complex_tag 
		where
			time >= ? 
		order by time`, ts)

	if err != nil {
		panic(err)
	}

	for i := 0; rows.Next(); i++ {
		var name string
		var ts time.Time
		var val float64
		var typ string
		var ival int64
		// var sval string
		// var id string
		var pname string
		// var period int64
		var payload string

		err := rows.Scan(&name, &ts, &val, &typ, &ival, &pname, &payload)
		if err != nil {
			panic(err)
		}
		require.Equal(t, fmt.Sprintf("name-%d", i%10), name)
		require.Equal(t, int64(i), ival)
		require.Equal(t, "pname", pname)
		require.Equal(t, `{"t":"json"}`, payload)
		// fmt.Println(name, ts, val, typ, pname, payload)
	}
	rows.Close()

	r := conn.QueryRow(ctx, "select count(*) from complex_tag where time >= ?", ts)
	if r.Err() != nil {
		panic(r.Err())
	}
	var count int
	err = r.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, expectCount, count)
	t.Log("---- append complex_tag done")
}
