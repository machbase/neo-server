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