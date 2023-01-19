package test

import (
	"fmt"
	"testing"
	"time"

	mach "github.com/machbase/neo-engine"
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

	db := mach.New()

	t.Log("---- append tag " + benchmarkTableName)
	appender, err := db.Appender(benchmarkTableName)
	if err != nil {
		panic(err)
	}
	defer appender.Close()

	for i := 0; i < testCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%5),
			time.Now(),
			1.001*float64(i+1),
			"some-id-string",
			/*nil*/ `{"name":"json"}`)
		if err != nil {
			panic(err)
		}
	}
	row := db.QueryRow("select count(*) from " + benchmarkTableName)
	if row.Err() != nil {
		panic(row.Err())
	}
	var count int
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	t.Logf("     %d records appended", count)
	require.Equal(t, testCount, count)

	t.Logf("---- append tag %s done", benchmarkTableName)
}
