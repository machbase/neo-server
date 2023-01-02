
### Check table

```sh
curl -o - -X POST http://127.0.0.1:4088/db/query \
    --data-urlencode "q=select count(*) from M\$SYS_TABLES where name = 'SAMPLE'"
```

```json
{
  "success": true,
  "reason": "1 rows selected",
  "elapse": "502.233µs",
  "data": {
    "colums": [
      "col01"
    ],
    "types": [
      "int64"
    ],
    "rows": [
      [
        0
      ]
    ]
  }
}
```

> The result is `0`

### Create table

```sh
curl -o - -X POST http://127.0.0.1:4088/db/query \
    --data-urlencode "q=create tag table sample (name varchar(200) primary key, time datetime basetime, value double summarized, jsondata json)"
```

```json
{
  "success":true,
  "reason":"success",
  "elapse":"305.626978ms"
}
```

- if the table already exists

```json
{
  "success":false,
  "reason":"MachError 2024 Table SAMPLE already exists.",
  "elapse":"313.574µs"
}
```

### Drop table

```sh
curl -o - -X POST http://127.0.0.1:4088/db/query \
    --data-urlencode "q=drop table sample"
```

```json
{
  "success": true,
  "reason": "success",
  "elapse": "800.64471ms"
}
```

- if the table does not exist

```json
{
  "success":false,
  "reason":"MachError 2025 Table SAMPLE does not exist.",
  "elapse":"359.153µs"
}
```

### Write

```sh
curl -o - -X POST http://127.0.0.1:4088/db/write/sample \
  -H "Content-Type: application/json" \
  -d "@sample.json"
```

- `sample.json`

```json
{
  "data": {
    "columns":["name", "time", "value"],
    "rows": [
      [ "sample.tag", 1670380342000000000, 1.0001 ],
      [ "sample.tag", 1670380343000000000, 2.0002 ]
    ]
  }
}
```

```json
{
  "success":true,
  "reason":"2 rows inserted",
  "elapse":"509.136µs",
  "data":{
    "affectedRows":2
  }
}
```

- if there is an error

```json
{
  "success":false,
  "reason":"record[0] bind unsupported idx 0 type []interface {}",
  "elapse":"420.814µs",
  "data":{
    "affectedRows":0
  }
}
```

### Query

```sh
curl -o - -X POST http://127.0.0.1:4088/db/query \
    --data-urlencode "q=select * from sample"
```

or use GET method

```sh
curl -o - http://127.0.0.1:4088/db/query?q=select%20*%20from%20sample
```

| param      | default  | desc                                |
| ---------- | -------- | ----------------------------------- |
| q          |          | sql text                            |
| timeformat | epoch    | format of datetime column           |
|            |          | `epoch`: unix epoch in nano seconds |

```json
{
  "success": true,
  "reason": "2 rows selected",
  "elapse": "387.663µs",
  "data": {
    "colums": [
      "col01",
      "col02",
      "col03",
      "col04"
    ],
    "types": [
      "string",
      "time",
      "float64",
      "string"
    ],
    "rows": [
      [
        "sample.tag",
        1670380342000000000,
        1.0001,
        null
      ],
      [
        "sample.tag",
        1670380343000000000,
        2.0002,
        null
      ]
    ]
  }
}
```