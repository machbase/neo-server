package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	mach "github.com/machbase/neo-engine"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

func TestAppendTag(t *testing.T) {
	testCount := 100

	defer func() {
		e := recover()
		if e == nil {
			return
		}
		fmt.Println(e)
	}()

	db, err := spi.New()
	require.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Connect(ctx, mach.WithTrustUser("sys"))
	if err != nil {
		t.Error(err.Error())
	}
	defer conn.Close()

	t.Log("---- append tag " + benchmarkTableName)
	appender, err := conn.Appender(ctx, benchmarkTableName)
	if err != nil {
		panic(err)
	}

	ts := time.Now()

	for i := 0; i < testCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%5),
			ts.Add(time.Duration(i)),
			1.001*float64(i+1),
			"some-id-string",
			/*nil*/ `{"name":"json"}`)
		if err != nil {
			panic(err)
		}
	}
	appender.Close()

	row := conn.QueryRow(ctx, "select count(*) from "+benchmarkTableName+" where time >= ?", ts)
	if row.Err() != nil {
		panic(row.Err())
	}
	var count int
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	t.Logf("     %s appended %d records", appender.TableName(), count)
	require.Equal(t, testCount, count)

	t.Logf("---- append tag %s done", benchmarkTableName)
}

func TestAppendTagNotExist(t *testing.T) {
	db, err := spi.New()
	require.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Connect(ctx, mach.WithTrustUser("sys"))
	if err != nil {
		t.Error(err.Error())
	}
	defer conn.Close()

	t.Log("---- append tag notexist")
	appender, err := conn.Appender(ctx, "notexist")
	require.NotNil(t, err)
	if appender != nil {
		appender.Close()
	}
	t.Logf("---- append tag notexist done")
}
