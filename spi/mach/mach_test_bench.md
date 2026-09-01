## 2024-10-21

```sh
go run mage.go bench
SIGINT accepted. (-t will block this).
goos: darwin
goarch: arm64
pkg: github.com/machbase/neo-engine
cpu: Apple M1
BenchmarkSimpleTagInsertDirectExecute
BenchmarkSimpleTagInsertDirectExecute-8            58429             19865 ns/op              80 B/op          3 allocs/op
BenchmarkSimpleTagInsertExecute
BenchmarkSimpleTagInsertExecute-8                  59935             19328 ns/op               8 B/op          1 allocs/op
BenchmarkSimpleTagAppend
BenchmarkSimpleTagAppend-8                       1909909               620.1 ns/op            32 B/op          3 allocs/op
PASS
ok      github.com/machbase/neo-engine  8.351s
Benchmark done.
```

### 2026-09-01 (with engine v8.7.0 dev-4158)

```sh
go test -benchmem -run ^$ -bench '^Benchmark.*$' github.com/machbase/neo-server/v8/spi/mach -timeout 60s -v
SIGINT accepted. (-t will block this).
goos: darwin
goarch: arm64
pkg: github.com/machbase/neo-server/v8/spi/mach
cpu: Apple M1
BenchmarkAll
BenchmarkAll/benchSimpleTagInsertDirectExecute
BenchmarkAll/benchSimpleTagInsertDirectExecute-8                  179400              6508 ns/op              87 B/op          3 allocs/op
BenchmarkAll/benchSimpleTagInsertExecute
BenchmarkAll/benchSimpleTagInsertExecute-8                        154387              7949 ns/op               8 B/op          1 allocs/op
BenchmarkAll/benchSimpleTagInsertExecute#01
BenchmarkAll/benchSimpleTagInsertExecute#01-8                     152755              7733 ns/op               8 B/op          1 allocs/op
BenchmarkAll/benchSimpleTagAppend
BenchmarkAll/benchSimpleTagAppend-8                              2067346               554.3 ns/op            32 B/op          3 allocs/op
PASS
ok      github.com/machbase/neo-server/v8/spi/mach      16.489s
```