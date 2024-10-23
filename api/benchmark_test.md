# Benchmark

## Schema

```sql
create tag table tag_data(
    name            varchar(100) primary key, 
    time            datetime basetime, 
    value           double,
    short_value     short,
    int_value       integer,
    long_value      long,
    str_value       varchar(400),
    json_value      json
)

create tag table tag_simple(
    name            varchar(100) primary key, 
    time            datetime basetime, 
    value           double
)
```

## Benchmark

```sh
go test -benchmem -run=^$ -bench . github.com/machbase/neo-server/api

goos: darwin
goarch: arm64
pkg: github.com/machbase/neo-server/api
cpu: Apple M1
BenchmarkTagDataAppend-8          737270              1677 ns/op             280 B/op         13 allocs/op
BenchmarkTagSimpleAppend-8       1807166               664.9 ns/op            80 B/op          4 allocs/op
```