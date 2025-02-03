# Stress-Test Client

## Usage

```sh
go run ./test/stress/stress.go [-scenario <scenario_name>] [-clean-start] [-clean-stop]
```

**help**

```sh
go run test/stress/stress.go -h
  -clean-start
        drop table before start
  -clean-stop
        drop table after stop
  -neo-http string
        machbase-neo http address (default "http://127.0.0.1:5654")
  -scenario string
        scenario name (default "default")
  -timeout duration
        override timeout of the scenario
```

**scenario names**

- default
- rollup

## Example

```sh
go run test/stress/stress.go \
    -neo-http http://127.0.0.1:5654 \
    -scenario rollup \
    -timeout 30m
```

