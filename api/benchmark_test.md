# Benchmark

## Schema

```sql
create tag table tag_data(
    name            varchar(100) primary key, 
    time            datetime basetime, 
    value           double,
    short_value     short,
    ushort_value    ushort,
    int_value       integer,
    uint_value 	    uinteger,
    long_value      long,
    ulong_value 	ulong,
    str_value       varchar(400),
    json_value      json,
    ipv4_value      ipv4,
    ipv6_value      ipv6
)

create tag table tag_simple(
    name            varchar(100) primary key, 
    time            datetime basetime, 
    value           double
)
```

## Benchmark

- 2024.10.31

```sh
go test -benchmem -run=^$ -bench . github.com/machbase/neo-server/api

goos: darwin
goarch: arm64
pkg: github.com/machbase/neo-server/api
cpu: Apple M1
BenchmarkTagDataAppend-8           59035             17203 ns/op             432 B/op         20 allocs/op
BenchmarkTagSimpleAppend-8       1715670               669.2 ns/op            80 B/op          4 allocs/op
```