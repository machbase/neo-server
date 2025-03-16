package machsvr_test

import (
	"context"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/api/testsuite"
)

//
// workerPoolSize := -1
//
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkCGoPool-32                 4593            228202 ns/op            3580 B/op         57 allocs/op
// BenchmarkCGoPool-64                 4292            262336 ns/op            3455 B/op         55 allocs/op
// BenchmarkCGoPool-128                2710            377501 ns/op            3487 B/op         55 allocs/op
// BenchmarkCGoPool-256                4089            278229 ns/op            3509 B/op         55 allocs/op
// BenchmarkCGoPool-512                3421            328371 ns/op            3567 B/op         55 allocs/op
// BenchmarkCGoPool-1024               3025            340315 ns/op            3768 B/op         56 allocs/op

//
// workerPoolSize := 12
//
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkCGoPool-32                 5461            198907 ns/op            3468 B/op         55 allocs/op
// BenchmarkCGoPool-64                 4230            253389 ns/op            3482 B/op         55 allocs/op
// BenchmarkCGoPool-128                3148            330091 ns/op            3503 B/op         55 allocs/op
// BenchmarkCGoPool-256                3250            311427 ns/op            3520 B/op         55 allocs/op
// BenchmarkCGoPool-512                3410            332480 ns/op            3576 B/op         55 allocs/op
// BenchmarkCGoPool-1024               3105            361616 ns/op            3775 B/op         56 allocs/op

func BenchmarkCGoPool(b *testing.B) {
	workerPoolSize := 12

	if workerPoolSize > 0 {
		machsvr.SetWorkerPoolSize(workerPoolSize)
		machsvr.StartWorkerPool(machsvrDB.(*machsvr.Database))
		defer machsvr.StopWorkerPool()
	}
	testsuite.CreateTestTables(machsvrDB)
	defer testsuite.DropTestTables(machsvrDB)

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.TODO()
		for pb.Next() {
			cgoPoolTestCase(ctx, b)
		}
	})
}

func cgoPoolTestCase(ctx context.Context, b *testing.B) {
	conn, err := machsvrDB.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		b.Fatal(err)
	}
	rows, err := conn.Query(ctx, "SELECT * from tag_data limit 10")
	if err != nil {
		b.Fatal(err)
	}
	for rows.Next() {
		var name string
		var ts time.Time
		var value float64
		err = rows.Scan(&name, &ts, &value)
		if err != nil {
			b.Fatal(err)
		}
	}
	rows.Close()
	conn.Close()
}
