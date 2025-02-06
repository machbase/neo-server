# HTTP Query Stress Test Client

Linear scale multi-http-client stress tool.

## Usage

```
go run ./test/linear -scenario [scenario-name] -n [client-num] -r [run-count-per-client]
```

- Flags

```
  -n int
        Number of workers to use (default 1)
  -neo-http string
        Neo HTTP address (default "http://127.0.0.1:5654")
  -r int
        Number of runs (default 1)
  -scenario string
        Scenario to run (default "default")
```

