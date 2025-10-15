# Machbase Neo gRPC API Query

Generally an application retrieves multiple records after executing a query through for-loop.

`Query` returns result handle as result of execution, then application can fetch all records with the handle by loop.

- Request `QueryRequest`

| Field  | Type         | Desc                 |
|:-------|:-------------|:---------------------|
| sql    | string       | sql query text       |
| params | array of any | query bind variables |

- Response `QueryResponse`

| Field      | Type         | Desc                             |
|:-----------|:-------------|:---------------------------------|
| succes     | bool         | `true` success, `false` error    |
| reason     | string       | response message                 |
| elapse     | string       | string to represent elapse time  |
| rowsHandle | RowsHandle   | handle of rows                   |

## Examples

### Go

#### Select multiple records

```go
sqlText := `select name, time, value from example limit ?`
rows, _ := cli.Query(sqlText, 3)
defer rows.Close()

for rows.Next() {
    var name string
    var ts time.Time
    var value float64
    rows.Scan(&name, &ts, &value)
    fmt.Println(name, ts, value)
}
```

**Important**: Since rows handle is consuming server-side resources, it is very import to release handle resource immediately after use. We uses `defer rows.close()` to release it in this example.