# Machbase Neo gRPC API QueryRow

`QueryRow` execute a query which returns result in a row.
It is suitable to execute queries that retrieve single record, for example `select count(*) from...`, `select col1,col2 from table where id=?`.
If the query returns multiple records, only the first record will be retrieved.

- Request `QueryRowRequest`

| Field  | Type         | Desc                 |
|:-------|:-------------|:---------------------|
| sql    | string       | sql query text       |
| params | array of any | query bind variables |

- Response `QueryRowResponse`

| Field  | Type         | Desc                             |
|:-------|:-------------|:---------------------------------|
| succes | bool         | `true` success, `false` error    |
| reason | string       | response message                 |
| elapse | string       | string to represent elapse time  |
| values | array of any | column values of the first row   |

## Examples

### Go

#### Select count(*)

```go
sqlText := `select count(*) from example`
row := cli.QueryRow(sqlText)

var count int
row.Scan(&count)

fmt.Println("count=", count)
```