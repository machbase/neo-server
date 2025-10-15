# Machbase Neo gRPC API Exec

`Exec` execute query which doesn't require result set.
It is suitable to execute queries that doesn't return any record but success or failure,
for example `create table...`, `drop table...`, `insert into...`.

- Request `ExecRequest`

| Field  | Type         | Desc                 |
|:-------|:-------------|:---------------------|
| sql    | string       | sql query text       |
| params | array of any | query bind variables |

- Response `ExecResponse`

| Field  | Type         | Desc                             |
|:-------|:-------------|:---------------------------------|
| succes | bool         | `true` success, `false` error    |
| reason | string       | response message                 |
| elapse | string       | string to represent elapse time  |

## Examples

### Go

#### Create table

```go
sqlText := `
    create tag table example (
        name varchar(100) primary key, 
        time datetime basetime, 
        value double
    )`

cli.Exec(sqlText)
```

#### Drop table

```go
sqlText := `drop table example`
cli.Exec(sqlText)
```

#### Insert

```go
sqlText := `insert into example (name, time, value) values (?, ?, ?)`
cli.Exec(sqlText, "tag-name-1", time.Now(), 1.234)
```