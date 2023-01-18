package test

import (
	"fmt"
	"testing"
	"time"

	mach "github.com/machbase/neo-engine"
	"github.com/stretchr/testify/require"
)

func TestAppendTag(t *testing.T) {
	testCount := 10

	db := mach.New()

	t.Log("---- append tag " + benchmarkTableName)
	appender, err := db.Appender(benchmarkTableName)
	if err != nil {
		panic(err)
	}
	//defer appender.Close()
	for i := 0; i < testCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%02d", i),
			time.Now(),
			1.001*float64(i+1),
			"some-id-string",
			nil)
		if err != nil {
			panic(err)
		}
	}
	appender.Close()

	r := db.QueryRow("select count(*) from " + benchmarkTableName)
	if r.Err() != nil {
		panic(r.Err())
	}
	var count int
	err = r.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, testCount, count)
	t.Logf("---- append tag %s done", benchmarkTableName)
}
