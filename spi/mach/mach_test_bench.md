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

### 2026-08-24 (with engine v8.7.0-alpha)

```sh
go run mage.go bench
SIGINT accepted. (-t will block this).
goos: darwin
goarch: arm64
pkg: github.com/machbase/neo-engine/v8
cpu: Apple M5
BenchmarkAll
BenchmarkAll/benchSimpleTagInsertDirectExecute
BenchmarkAll/benchSimpleTagInsertDirectExecute-10         	  283183	      3937 ns/op	      90 B/op	       3 allocs/op
BenchmarkAll/benchSimpleTagInsertExecute
BenchmarkAll/benchSimpleTagInsertExecute-10               	  247402	      4576 ns/op	       8 B/op	       1 allocs/op
BenchmarkAll/benchSimpleTagInsertExecute#01
BenchmarkAll/benchSimpleTagInsertExecute#01-10            	  259257	      4762 ns/op	       8 B/op	       1 allocs/op
BenchmarkAll/benchSimpleTagAppend
BenchmarkAll/benchSimpleTagAppend-10                      	 3766383	       320.1 ns/op	      32 B/op	       3 allocs/op
PASS
ok  	github.com/machbase/neo-engine/v8	17.371s
Benchmark done.
```