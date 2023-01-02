
### Check table

- subscribe to `db/reply` to receive query result

```go
client.Subscribe("db/reply", 1, func(_ paho.Client, msg paho.Message) {
    buff := msg.Payload()
    t.Logf("RECV: %v", string(buff))
})
```

- publish query to `db/query`

```go
jsonStr := `{
        "q": "select count(*) from M$SYS_TABLES where name = 'SAMPLE'"
}`
client.Publish("db/query", 1, false, []byte(jsonStr))
```

- replied message

```json
{
    "success":true,
    "reason":"1 rows selected",
    "elapse":"1.4332ms",
    "data":{
        "colums":["col01"],
        "types":["int64"],
        "rows":[
            [0]
        ]
    }
}
```

### Insert

```go
jsonStr = `{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "sample.tag", 1670380342000000000, 1.0001 ],
            [ "sample.tag", 1670380343000000000, 2.0002 ]
        ]
    }
}`
client.Publish("db/write/sample", 1, false, []byte(jsonStr))
```