package machsvr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createSimpleTagTable() {
	ctx := context.TODO()
	conn, err := database.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	result := conn.Exec(ctx, SqlTidy(
		`create tag table simple_tag(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		)`))
	if result.Err() != nil {
		panic(result.Err())
	}
}

func TestAppendTagSimple(t *testing.T) {
	t.Logf("---- append simple_tag [%d]", goid())
	ctx := context.TODO()
	conn, err := database.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	pr := conn.QueryRow(ctx, "select count(*) from simple_tag")
	if pr.Err() != nil {
		panic(pr.Err())
	}
	var existingCount int
	err = pr.Scan(&existingCount)
	if err != nil {
		panic(err)
	}

	appender, err := conn.Appender(ctx, "simple_tag")
	if err != nil {
		panic(err)
	}

	t.Logf("     %s", appender.TableName())
	expectCount := 10000
	ts := time.Now()
	for i := 0; i < expectCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%10),
			ts.Add(time.Duration(i)),
			1.001*float64(i+1))
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

	rows, err := conn.Query(ctx, "select name, time, value from simple_tag where time >= ? order by time", ts)
	if err != nil {
		panic(err)
	}

	for i := 0; rows.Next(); i++ {
		var name string
		var ts time.Time
		var val float64

		err := rows.Scan(&name, &ts, &val)
		if err != nil {
			panic(err)
		}
		require.Equal(t, fmt.Sprintf("name-%d", i%10), name)
	}
	rows.Close()

	r := conn.QueryRow(ctx, "select count(*) from simple_tag where time >= ?", ts)
	if r.Err() != nil {
		panic(r.Err())
	}
	var count int
	err = r.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, expectCount, count)
	t.Log("---- append simple_tag done")

	// wait flush data of append
	time.Sleep(1 * time.Second)
	rows, err = conn.Query(ctx, "select name, time, value from simple_tag where name = 'name-0' limit 5")
	if err != nil {
		panic(err)
	}
	countName := 0
	for i := 0; rows.Next(); i++ {
		var name string
		var ts time.Time
		var val float64

		err := rows.Scan(&name, &ts, &val)
		if err != nil {
			panic(err)
		}
		require.Equal(t, "name-0", name)
		countName++
		t.Log(name, ts, val)
	}
	rows.Close()
	require.Equal(t, 5, countName)
}
