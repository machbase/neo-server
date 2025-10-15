# Machbase Neo HTTP Write

The Write API endpoint is `/db/write/{TABLE}`, where `{TABLE}` is the name of the table you want to write to.

Even `query` api can execute 'INSERT' statement, it is not an efficient way to write data,
since clients should build a static sql text in `q` parameter for the every request.
The proper way writing data is the `write` api which is the `INSERT` statement equivalent. 
And another benefit of `write` is that a client application can insert multiple records in a single `write` request.

## Parameters

**Write Parameters**

| param       | default | description                     |
|:----------- |---------|:------------------------------- |
| timeformat  | `ns`     | Time format: `s`, `ms`, `us`, `ns` |
| tz          | `UTC`    | Time Zone: `UTC`, `Local` and location spec |
| method      | `insert` | Writing methods: `insert`, `append`  |

**INSERT vs. APPEND**

By default, the `/db/write` API uses the `INSERT INTO...` statement to write data. For a small number of records, this method performs similarly to the `append` method.

When writing a large amount of data (e.g., more than several hundred thousand records), use the `method=append` parameter. This specifies that Machbase Neo should use the "append" method instead of the default "INSERT INTO..." statement, which is implicitly specified as `method=insert`.

**Content-Type Header**

The machbase-neo server recognizes the format of incoming data stream by `Content-Type` header,
for example, `Content-Type: application/json` for JSON data, `Content-Type: text/csv` for csv data, and `Content-type: application/x-ndjson` for newline delimiter json data.

**Content-Encoding Header**

If client sends gzip'd compress stream, it should set the header `Content-Encoding: gzip` 
that tells the machbase-neo the incoming data stream is encoded in gzip.

## Inputs

### JSON

This request message is equivalent that consists INSERT SQL statement as `INSERT into {table} (columns...) values (values...)`

| name         | type       |  description                        |
|:------------ |:-----------|:------------------------------------|
| data         | object           |                               |
| data.columns | array of strings | represents columns            |
| data.rows    | array of tuples  | values of records             |

**JSON**

```json
{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "json-data", 1670380342000000000, 1.0001 ],
            [ "json-data", 1670380343000000000, 2.0002 ]
        ]
    }
}
```

Set `Content-Type` header as `application/json`.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
Content-Type: application/json

{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "json-data", 1670380342000000000, 1.0001 ],
            [ "json-data", 1670380343000000000, 2.0002 ]
        ]
    }
}
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE \
    -H "Content-Type: application/json" \
    --data-binary "@post-data.json"
```

**Compressed JSON**

Set the header `Content-Encoding: gzip` tells machbase-neo that the incoming stream is gzip-compressed.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
Content-Type: application/json
Content-Encoding: gzip

< /csv/post-data.json.gz
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE \
    -H "Content-Type: application/json" \
    -H "Content-Encoding: gzip" \
    --data-binary "@post-data.json.gz"
```

**JSON with timeformat**

When time fields are string format instead of UNIX epoch.

Add `timeformat` and `tz` parameters.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=DEFAULT
    &tz=Asia/Seoul
Content-Type: application/json

{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "json-data", "2022-12-07 02:32:22", 1.0001 ],
            [ "json-data", "2022-12-07 02:32:23", 2.0002 ]
        ]
    }
}
```
~~~

**cURL:**
```sh
curl -X POST 'http://127.0.0.1:5654/db/write/EXAMPLE?timeformat=DEFAULT&tz=Asia/Seoul' \
    -H "Content-Type: application/json" \
    --data-binary "@post-data.json"
```

- `post-data.json`

```json
{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "json-data", "2022-12-07 02:32:22", 1.0001 ],
            [ "json-data", "2022-12-07 02:32:23", 2.0002 ]
        ]
    }
}
```

### NDJSON

NDJSON (Newline Delimited JSON) is a format for streaming JSON data where each line is a valid JSON object. This is useful for processing large datasets or streaming data.

This request message is equivalent that consists INSERT SQL statement as `INSERT into {table} (columns...) values (values...)`

**NDJSON**

```json
{"NAME":"ndjson-data", "TIME":1670380342000000000, "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":1670380343000000000, "VALUE":2.002}
```

Set `Content-Type` header as `application/x-ndjson`.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
Content-Type: application/x-ndjson

{"NAME":"ndjson-data", "TIME":1670380342000000000, "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":1670380343000000000, "VALUE":2.002}
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE \
    -H "Content-Type: application/x-ndjson" \
    --data-binary "@post-data.json"
```

**NDJSON with timeformat**

When time fields are string format instead of UNIX epoch.

```json
{"NAME":"ndjson-data", "TIME":"2022-12-07 02:33:22", "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":"2022-12-07 02:33:23", "VALUE":2.002}
```

Add `timeformat` and `tz` parameters.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=DEFAULT
    &tz=Local
Content-Type: application/x-ndjson

{"NAME":"ndjson-data", "TIME":"2022-12-07 02:33:22", "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":"2022-12-07 02:33:23", "VALUE":2.002}
```
~~~

**cURL:**
```sh
curl -X POST 'http://127.0.0.1:5654/db/write/EXAMPLE?timeformat=Default&tz=Local' \
    -H "Content-Type: application/x-ndjson" \
    --data-binary "@post-data.json"
```

### CSV

These options are only applicable when the content body is in CSV format.

| param         | default | description                     |
|:------------- |---------|:------------------------------- |
| header        |         | `skip`: simply skip the first line<br/> `columns`: the CSV has a header line where fields match the column names. |
| heading       | false   | Deprecated, `heading=true` is equivalent with `header=skip`. |
| delimiter     | ,       | field delimiter |

If the CSV data includes a header line, set the `header=skip` query parameter to make machbase-neo to ignore the first line.

If the CSV header line specifies columns to write, use `header=columns`. This option ensures that the header matches the column names of the table. The header line will be used as the *columns* part of the SQL statement `INSERT INTO TABLE(columns...) VALUES(...)`.

If the header line is not included and omit `header` option (or equivalent with `heading=false`) which is default, each line's fields must match all the columns of the table in order to match the SQL statement `INSERT INTO TABLE VALUES(...)`.

> According to the semantics of append method, `header=columns` does not work with `method=append`.

**header=skip**

If you set `header=skip`, the server will ignore the first line, and the data should be in the same order as the columns of the table.

```csv
NAME,TIME,VALUE
csv-data,1670380342000000000,1.0001
csv-data,1670380343000000000,2.0002
```

The `Content-Type` header should be `text/csv`.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?header=skip
Content-Type: text/csv

NAME,TIME,VALUE
csv-data,1670380342000000000,1.0001
csv-data,1670380343000000000,2.0002
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE?header=skip \
    -H "Content-Type: text/csv" \
    --data-binary "@post-data.csv"
```

**header=columns**

If the CSV fields are in a different order or are a subset of the actual table columns, set `header=columns`. The server will then treat the first line as the column names. The example below generates an internal SQL statement similar to `INSERT INTO EXAMPLE (TIME, NAME, VALUE) VALUES(?, ?, ?)`

```csv
TIME,NAME,VALUE
1670380342000000000,csv-data,1.0001
1670380343000000000,csv-data,2.0002
```

The `Content-Type` header should be `text/csv`.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?header=columns
Content-Type: text/csv

TIME,NAME,VALUE
1670380342000000000,csv-data,1.0001
1670380343000000000,csv-data,2.0002
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE?header=columns \
    -H "Content-Type: text/csv" \
    --data-binary "@post-data.csv"
```

**Compressed CSV**

Set the header `Content-Encoding: gzip` to inform machbase neo that the incoming stream is gzip-compressed.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?header=skip
Content-Type: text/csv
Content-Encoding: gzip

< /csv/post-data.json.gz
```
~~~

**cURL:**
```sh
curl -X POST http://127.0.0.1:5654/db/write/EXAMPLE?header=skip \
    -H "Content-Type: text/csv" \
    -H "Content-Encoding: gzip" \
    --data-binary "@post-data.csv.gz"
```

**CSV with timeformat**

Add `timeformat` and `tz` query parameters.

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?header=skip
    &timeformat=Default
    &tz=Asia/Seoul
Content-Type: text/csv

NAME,TIME,VALUE
csv-data,2022-12-07 11:39:32,1.0001
csv-data,2022-12-07 11:39:33,2.0002
```
~~~

## Examples

Please refer to the detail of the API [Request endpoint and params](/neo/api-http/write#request-endpoint-and-params)

**Test Table**

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=create tag table if not exists EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode \
    "q=create tag table EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)"
```

**Time**

The time stored in the sample files saved in these examples is represented in Unix epoch, measured in seconds. Therefore, when loading the data, it should be performed with the `timeformat=s` option specified. If the data has been stored in a different resolution, this option needs to be modified to ensure proper input. Note that in Machbase Neo, the default time resolution is assumed to be in `nanoseconds (ns)` and is executed accordingly.

### JSON with epoch

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=s
Content-Type: application/json

{
  "data":  {
    "columns":["NAME","TIME","VALUE"],
    "rows": [
        ["wave.sin",1676432361,0],
        ["wave.sin",1676432362,0.406736],
        ["wave.sin",1676432363,0.743144],
        ["wave.sin",1676432364,0.951056],
        ["wave.sin",1676432365,0.994522]
    ]
  }
}
```
~~~

**Select rows**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
```
~~~

### CSV with epoch

If csv data has header line like below, set the `header=skip` query param.

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=s
    &header=skip
Content-Type: text/csv

NAME,TIME,VALUE
wave.sin,1676432361,0.000000
wave.cos,1676432361,1.000000
wave.sin,1676432362,0.406736
wave.cos,1676432362,0.913546
wave.sin,1676432363,0.743144

```
~~~

### CSV without header

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?timeformat=s
Content-Type: text/csv

wave.sin,1676432361,0.000000
wave.cos,1676432361,1.000000
wave.sin,1676432362,0.406736
wave.cos,1676432362,0.913546
wave.sin,1676432363,0.743144
```
~~~

### CSV

**Insert**

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=Default
Content-Type: text/csv

wave.sin,2023-02-15 03:39:21,0.111111
wave.sin,2023-02-15 03:39:22.111,0.222222
wave.sin,2023-02-15 03:39:23.222,0.333333
wave.sin,2023-02-15 03:39:24.333,0.444444
wave.sin,2023-02-15 03:39:25.444,0.555555
```
~~~

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
    &timeformat=Default
    &format=csv
```
~~~

**Append**

When loading a large CSV file, using the "append" method can allow data to be input several times faster compared to the "insert" method.

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?timeformat=s&method=append
Content-Type: text/csv

wave.sin,1676432361,0.000000
wave.cos,1676432361,1.000000
wave.sin,1676432362,0.406736
wave.cos,1676432362,0.913546
wave.sin,1676432363,0.743144
```
~~~

### CSV with time zone

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=Default
    &tz=Asia/Seoul
Content-Type: text/csv

wave.sin,2023-02-15 12:39:21,0.111111
wave.sin,2023-02-15 12:39:22.111,0.222222
wave.sin,2023-02-15 12:39:23.222,0.333333
wave.sin,2023-02-15 12:39:24.333,0.444444
wave.sin,2023-02-15 12:39:25.444,0.555555
```
~~~

**Select rows in UTC**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
    &timeformat=Default
    &format=csv
```
~~~

### `RFC3339`

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=RFC3339
Content-Type: text/csv

wave.sin,2023-02-15T03:39:21Z,0.111111
wave.sin,2023-02-15T03:39:22Z,0.222222
wave.sin,2023-02-15T03:39:23Z,0.333333
wave.sin,2023-02-15T03:39:24Z,0.444444
wave.sin,2023-02-15T03:39:25Z,0.555555
```
~~~

**Select rows in UTC**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
    &format=csv
    &timeformat=RFC3339
```
~~~

### `RFC3339Nano` in time zone

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=RFC3339Nano
    &tz=America/New_York
Content-Type: text/csv

wave.sin,2023-02-14T22:39:21.000000000-05:00,0.111111
wave.sin,2023-02-14T22:39:22.111111111-05:00,0.222222
wave.sin,2023-02-14T22:39:23.222222222-05:00,0.333333
wave.sin,2023-02-14T22:39:24.333333333-05:00,0.444444
wave.sin,2023-02-14T22:39:25.444444444-05:00,0.555555
```
~~~


**Select rows in America/New_York**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
    &format=box
    &timeformat=RFC3339Nano
    &tz=America/New_York
```
~~~

### Timeformat

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=Default
Content-Type: text/csv

wave.sin,2023-02-15 03:39:21,0.111111
wave.sin,2023-02-15 03:39:22.111111111,0.222222
wave.sin,2023-02-15 03:39:23.222222222,0.333333
wave.sin,2023-02-15 03:39:24.333333333,0.444444
wave.sin,2023-02-15 03:39:25.444444444,0.555555
```
~~~

**Select rows in UTC**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 10
    &format=csv
    &timeformat=Default

```
~~~

### Custom Timeformat

- `hour:min:sec-SPLIT-year-month-day` format in New York timezone

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
    ?timeformat=03:04:05.999999999-SPLIT-2006-01-02
    &tz=America/New_York
Content-Type: text/csv

wave.sin,10:39:21-SPLIT-2023-02-14 ,0.111111
wave.sin,10:39:22.111111111-SPLIT-2023-02-14 ,0.222222
wave.sin,10:39:23.222222222-SPLIT-2023-02-14 ,0.333333
wave.sin,10:39:24.333333333-SPLIT-2023-02-14 ,0.444444
wave.sin,10:39:25.444444444-SPLIT-2023-02-14 ,0.555555
```
~~~

**Select rows**

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=csv
    &timeformat=2006-01-02 03:04:05.999999999
    &tz=America/New_York
```
~~~