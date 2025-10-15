# Machbase Neo HTTP Query

Query API endpoint is `/db/query`.

The query api does not support only "SELECT" but also "CREATE TABLE", "ALTER TABLE", "INSERT"... all other SQL statements.

The `/db/query` API supports *GET*, *POST JSON* and *POST form-data*. The all methods supports the same parameters.

For example the parameter `format` can be specified in query parameter in *GET* method like `GET /db/query?format=csv`,
or be a JSON field in *POST-JSON* method as `{ "format": "csv" }`.

**Query Example**

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
```
~~~

**cURL:**
```sh 
curl -o - http://127.0.0.1:5654/db/query \
     --data-urlencode "q=select * from EXAMPLE limit 2"
```

## Parameters

**Query Parameters**

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| **q**       | _required_ | SQL query string              |
| format      | `json`    | Result data format: json, csv, box, ndjson |
| timeformat  | `ns`      | Time format: s, ms, us, ns    |
| tz          | `UTC`     | Time Zone: UTC, Local and location spec |
| compress    | _no compression_   | compression method: gzip      |
| rownum      | `false`   | including rownum: true, false |

**Available parameters with `format=json`**

* The options are only available when `format=json`. Those options are exclusive each other, applicable only one of them per a request.

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| transpose   | false   | produce cols array instead of rows.|
| rowsFlatten | false   | reduce the array dimension of the *rows* field in the JSON object.|
| rowsArray   | false   | produce JSON that contains only array of object for each record.|

**Available parameters with `format=csv`**

| param       | default | description                    |
|:----------- |---------|:------------------------------ |
| header      |         | `skip` do not include header line, equivalent to `heading=false` |
| heading     | `true`  | show header line: true, false, Deprecated use `header` instead  |
| precision   | `-1`    | precision of float value, -1 for no round, 0 for integers |

**Available timeformat**
 
* Please refer to the [API Options/timeformat](../../options/timeformat/) section for the available time formats.

## Outputs

If the response content is too large to determine the total length, The header `Transfer-Encoding: chunked` is set, and the `Content-Length` header is omitted. The end of the response is identified by the last two consecutive newline characters (`\n\n`).

- `Transfer-Encoding: chunked`: Means the data is sent in a series of chunks, useful for streaming.
- Absence of `Content-Length`: Indicates that the total length of the response body is not known in advance.

### JSON

The `/db/query` api's default output format is json.
Set query param `format=json` or omit it for the default value.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2"
```

The server responses in `Content-Type: application/json`.

| name         | type       |  description                        |
|:------------ |:-----------|:------------------------------------|
| **success**  | bool       | `true` if query execution succeed |
| **reason**   | string     | execution result message, this will contains error message if `success` is `false`  |
| **elapse**   | string     | elapse time of the query execution    |
| data         |            | exists only when execution succeed  |
| data.columns | array of strings | represents columns of result    |
| data.types   | array of strings | represents data types of result |
| data.rows    | array of records | array represents the result set records.<br/>This field will be replaced with `cols` if `transpose` is `true` |
| data.cols    | array of series  | array represents the result set column-series.<br/> This element exists when `transpose` is `true` |

**default:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 3
```
~~~

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      [ "wave.sin", 1705381958775759000, 0.8563571936170834 ],
      [ "wave.sin", 1705381958785759000, 0.9011510331449053 ],
      [ "wave.sin", 1705381958795759000, 0.9379488170706388 ]
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "1.887042ms"
}
```

**transpose:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 3
    &transpose=true
```
~~~

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "cols": [
      [ "wave.sin", "wave.sin", "wave.sin" ],
      [ 1705381958775759000, 1705381958785759000, 1705381958795759000 ],
      [ 0.8563571936170834, 0.9011510331449053, 0.9379488170706388 ]
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "4.090667ms"
}
```

**rowsFlatten:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 3
    &rowsFlatten=true
```
~~~

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      "wave.sin", 1705381958775759000, 0.8563571936170834,
      "wave.sin", 1705381958785759000, 0.9011510331449053,
      "wave.sin", 1705381958795759000, 0.9379488170706388
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "2.255625ms"
}
```

**rowsArray:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 3
    &rowsArray=true
```
~~~

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      { "NAME": "wave.sin", "TIME": 1705381958775759000, "VALUE": 0.8563571936170834 },
      { "NAME": "wave.sin", "TIME": 1705381958785759000, "VALUE": 0.9011510331449053 },
      { "NAME": "wave.sin", "TIME": 1705381958795759000, "VALUE": 0.9379488170706388 }
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "3.178458ms"
}
```

### NDJSON

Set query param `format=ndjson` in the request.

NDJSON (Newline Delimited JSON) is a format for streaming JSON data where each line is a valid JSON object. This is useful for processing large datasets or streaming data because it allows you to handle one JSON object at a time.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
    &format=ndjson
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    --data-urlencode "format=ndjson"
```

The response comes with `Content-Type: application/x-ndjson`.

```json
{"NAME":"wave.sin","TIME":1705381958775759000,"VALUE":0.8563571936170834}
{"NAME":"wave.sin","TIME":1705381958785759000,"VALUE":0.9011510331449053}

```

### CSV

Set query param `format=csv` in the request.

CSV format is also useful for processing large datasets or streaming data because it allows you to handle one line at a time.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
    &format=csv
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    --data-urlencode "format=csv"
```

The response comes with `Content-Type: text/csv; utf-8`

```csv
NAME,TIME,VALUE
wave.sin,1705381958775759000,0.8563571936170834
wave.sin,1705381958785759000,0.9011510331449053
```

### BOX

Set query param `format=box` in the request.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
    &format=box
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    --data-urlencode "format=box"
```

The result data in plain text with ascii box. The Content-Type of the response is `plain/text` 

```
+----------+---------------------+--------------------+
| NAME     | TIME(UTC)           | VALUE              |
+----------+---------------------+--------------------+
| wave.sin | 1705381958775759000 | 0.8563571936170834 |
| wave.sin | 1705381958785759000 | 0.9011510331449053 |
+----------+---------------------+--------------------+
```

**Response in CSV format**

Set query param `format=csv` in the request.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 2
    &format=csv
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    --data-urlencode "format=csv"
```

The response comes with `Content-Type: text/csv`

```csv
NAME,TIME,VALUE
wave.sin,1705381958775759000,0.8563571936170834
wave.sin,1705381958785759000,0.9011510331449053
```

## POST JSON

It is also possible to request query in JSON form as below example.

**Request JSON message**

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/query
Content-Type: application/json

{
  "q": "select * from EXAMPLE limit 2"
}
```
~~~

**cURL:**
```sh
curl -o - -X POST http://127.0.0.1:5654/db/query \
    -H 'Content-Type: application/json' \
    -d '{ "q":"select * from EXAMPLE limit 2" }'
```

## POST Form

HTML Form data format is available too. HTTP header `Content-type` should be `application/x-www-form-urlencoded` in this case.

**HTTP:**
~~~
```http
POST http://127.0.0.1:5654/db/query
Content-Type: application/x-www-form-urlencoded

q=select * from EXAMPLE limit 2
```
~~~

**cURL:**
```sh
curl -o - -X POST http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2"
```

## Examples

Please refer to the detail of the API 
- [Request endpoint and params](/neo/api-http/query)
- [List of Time Zones from wikipedia.org](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)

For this tutorials, pre-write data below.

### Step 1: Create Table

~~~
```http
POST http://127.0.0.1:5654/db/query
Content-Type: application/json

{
  "q":"create tag table if not exists EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)"
}
```
~~~

### Step 2: Insert Table

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE?timeformat=ns
Content-Type: application/json

{
    "data":{
      "columns":["NAME","TIME","VALUE"],
      "rows":[
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

### Select in CSV

**Request**

Set the `format=csv` query param for CSV format.

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=csv
```
~~~

**Response**
```
NAME,TIME,VALUE
wave.sin,1676432361000000000,0.111111
wave.sin,1676432362111111111,0.222222
wave.sin,1676432363222222222,0.333333
wave.sin,1676432364333333333,0.444444
wave.sin,1676432365444444444,0.555555
```

### Select in BOX

**Request**

Set the `format=box` query param for BOX format.

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=box
```
~~~

**Response**

```
+----------+---------------------+----------+
| NAME     | TIME                | VALUE    |
+----------+---------------------+----------+
| wave.sin | 1676432361000000000 | 0        |
| wave.sin | 1676432362111111111 | 0.406736 |
| wave.sin | 1676432363222222222 | 0.743144 |
| wave.sin | 1676432364333333333 | 0.951056 |
| wave.sin | 1676432365444444444 | 0.994522 |
+----------+---------------------+----------+
```

### Select in BOX with rownum

**Request**

Set the `format=box` query param for BOX format.

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=box
    &rownum=true
```
~~~

**Response**

```
+--------+----------+---------------------+----------+
| ROWNUM | NAME     | TIME                | VALUE    |
+--------+----------+---------------------+----------+
|      1 | wave.sin | 1676432361000000000 | 0.111111 |
|      2 | wave.sin | 1676432362111111111 | 0.222222 |
|      3 | wave.sin | 1676432363222222222 | 0.333333 |
|      4 | wave.sin | 1676432364333333333 | 0.444444 |
|      5 | wave.sin | 1676432365444444444 | 0.555555 |
+--------+----------+---------------------+----------+
```


### Select in BOX without heading

**Request**

Set the `format=box` and `header=skip` query param for BOX format without header.

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=box
    &header=skip
```
~~~

**Response**

```
+----------+---------------------+----------+
| wave.sin | 1676432361000000000 | 0        |
| wave.sin | 1676432362111111111 | 0.406736 |
| wave.sin | 1676432363222222222 | 0.743144 |
| wave.sin | 1676432364333333333 | 0.951056 |
| wave.sin | 1676432365444444444 | 0.994522 |
+----------+---------------------+----------+
```

### Select in BOX value in INTEGER

**Request**

Set the `format=box` and `precision=0` query param for BOX format with integer precision.

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from EXAMPLE limit 5
    &format=box
    &precision=0
```
~~~

**Response**

```
+----------+---------------------+-------+
| NAME     | TIME                | VALUE |
+----------+---------------------+-------+
| wave.sin | 1676432361000000000 | 0     |
| wave.sin | 1676432362111111111 | 0     |
| wave.sin | 1676432363222222322 | 0     |
| wave.sin | 1676432364333333233 | 0     |
| wave.sin | 1676432365444444444 | 1     |
+----------+---------------------+-------+
```