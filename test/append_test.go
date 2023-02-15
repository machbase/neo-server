package test

import (
	"fmt"
	"testing"
	"time"

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

	t.Log("---- append tag " + benchmarkTableName)
	appender, err := db.Appender(benchmarkTableName)
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

	row := db.QueryRow("select count(*) from "+benchmarkTableName+" where time >= ?", ts)
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
