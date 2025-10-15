# Machbase Neo TQL Writing API

> **Note**: For the examples, create a table with the following SQL statements.

```sql
CREATE TAG TABLE IF NOT EXISTS EXAMPLE (
    NAME VARCHAR(20) PRIMARY KEY,
    TIME DATETIME BASETIME,
    VALUE DOUBLE SUMMARIZED
);
```

## INSERT CSV

### 1. Create TQL File

Save the code below as `input-csv.tql`. When you save a TQL script, the editor will display a link icon  in the top right corner. Click on it to copy the script file's address.

```js
CSV(payload(), 
    field(0, stringType(), 'name'),
    field(1, timeType('ns'), 'time'),
    field(2, floatType(), 'value'),
    header(false)
)
INSERT("name", "time", "value", table("example"))
```

### 2. HTTP POST

#### Using HTTP Request

~~~
```http
POST http://127.0.0.1:5654/db/tql/input-csv.tql
Content-Type: text/csv

TAG0,1628866800000000000,12
TAG0,1628953200000000000,13
```
~~~

#### Using cURL

Prepare data file as `input-csv.csv`

```csv
TAG0,1628866800000000000,12
TAG0,1628953200000000000,13
```

Invoke `input-csv.tql` with the data file with `curl` command

```sh
curl -X POST http://127.0.0.1:5654/db/tql/input-csv.tql \
    -H "Content-Type: text/csv" \
    --data-binary "@input-csv.csv"
```

### 3. MQTT PUBLISH

Prepare data file as `input-csv.csv`

```csv
TAG1,1628866800000000000,12
TAG1,1628953200000000000,13
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/tql/input-csv.tql \
    -f input-csv.csv
```

## APPEND CSV

### 1. Create TQL File

Save the code below as `append-csv.tql`. When you save a TQL script, the editor will display a link icon  in the top right corner. Click on it to copy the script file's address.

```js
CSV(payload(), 
    field(0, stringType(), 'name'),
    field(1, timeType('ns'), 'time'),
    field(2, floatType(), 'value'),
    header(false)
)
APPEND(table('example'))
```

### 2. HTTP POST

#### Using HTTP Request

~~~
```http
POST http://127.0.0.1:5654/db/tql/append-csv.tql
Content-Type: text/csv

TAG0,1628866800000000000,12
TAG0,1628953200000000000,13
```
~~~

#### Using cURL

Prepare data file as `append-csv.csv`

```csv
TAG2,1628866800000000000,12
TAG2,1628953200000000000,13
```

Invoke `append-csv.tql` with the data file with `curl` command

```sh
curl -X POST http://127.0.0.1:5654/db/tql/append-csv.tql \
    -H "Content-Type: text/csv" \
    --data-binary "@append-csv.csv"
```

### 3. MQTT PUBLISH

Prepare data file as `append-csv.csv`

```csv
TAG3,1628866800000000000,12
TAG3,1628953200000000000,13
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/tql/input-csv.tql \
    -f append-csv.csv
```

## Custom JSON

### 1. Create TQL File

Use SCRIPT() function to parse a custom format JSON.

Save the code below as `input-json.tql`.

```js
SCRIPT({
    obj = JSON.parse($.payload)
    obj.data.rows.forEach(r => $.yield(...r))
})
INSERT("name", "time", "value", table("example"))
```

### 2. HTTP POST

#### Using HTTP Request

~~~
```http
POST http://127.0.0.1:5654/db/tql/input-json.tql
Content-Type: application/json

{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      [ "TAG0", 1628866800000000000, 12 ],
      [ "TAG0", 1628953200000000000, 13 ]
    ]
  }
}
```
~~~

#### Using cURL

Prepare data file as `input-json.json`

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      [ "TAG0", 1628866800000000000, 12 ],
      [ "TAG0", 1628953200000000000, 13 ]
    ]
  }
}
```

Invoke `input-json.tql` with the data file with `curl` command

```sh
curl -X POST http://127.0.0.1:5654/db/tql/input-json.tql \
    -H "Content-Type: application/json" \
    --data-binary "@input-json.json"
```

### 3. MQTT PUBLISH

Prepare data file as `input-json.json`

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      [ "TAG1", 1628866800000000000, 12 ],
      [ "TAG1", 1628953200000000000, 13 ]
    ]
  }
}
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/tql/input-json.tql \
    -f input-json.json
```

## Custom Text

When the data transforming is required for writing to the database, prepare the proper TQL script and publish the data to the topic named `db/tql/` + `{tql_file.tql}`.

### 1. Create TQL File

The example code below shows how to handle multi-lines text data for writing into a table.

#### Using MAP Functions

Transforming using MAP functions.

```js
// payload() returns the payload that arrived via HTTP-POST or MQTT,
// The ?? operator means that if tql is called without content,
//        the right side value is applied
// It is a good practice while the code is being developed on the tql editor of web-ui.
STRING( payload() ?? ` 12345
                     23456
                     78901
                     89012
                     90123
                  `, separator('\n'), trimspace(true))
FILTER( len(value(0)) > 0 )   // filter empty line
// transforming data
MAPVALUE(-1, time("now"))     // equiv. PUSHVALUE(0, time("now"))
MAPVALUE(-1, "text_"+key())   // equiv. PUSHVALUE(0, "text_"+key())
MAPVALUE(2, strSub( value(2), 0, 2 ) )

// Run this code in the tql editor of web-ui for testing
CSV( timeformat("DEFAULT") )
// Use APPEND(table('example')) for the real action
// APPEND(table('example'))
```

#### Using SCRIPT Function

The alternative way using SCRIPT function.

```js
// payload() returns the payload that arrived via HTTP-POST or MQTT,
// The ?? operator means that if tql is called without content,
//        the right side value is applied
// It is a good practice while the code is being developed on the tql editor of web-ui.
STRING( payload() ?? ` 12345
                     23456
                     78901
                     89012
                     90123
                  `, separator('\n'), trimspace(true) )
FILTER( len(value(0)) > 0) // filter empty line
// transforming data
SCRIPT({
  str = $.values[0].trim() ;  // trim spaces
  str = str.substring(0, 2);  // takes the first 2 letters of the line
  ts = (new Date()).getTime() * 1000000 // ms. to ns.
  $.yieldKey("text_"+$.key, ts, parseInt(str))
})
CSV()
// APPEND(table('example'))
```

**Result:**

```csv
text_1,2023-12-02 11:03:36.054,12
text_2,2023-12-02 11:03:36.054,23
text_3,2023-12-02 11:03:36.054,78
text_4,2023-12-02 11:03:36.054,89
text_5,2023-12-02 11:03:36.054,90
```

Run the code above and if there is no error and works as expected, then replace the last line `CSV()` with `APPEND(table('example'))`.

Save the code as "script-post-lines.tql", then send some test data to the topic `db/tql/script-post-lines.tql`.

**Sample Data File** - `cat lines.txt`

```
110000
221111
332222
442222
```

### 2. HTTP POST

For the note, the same TQL file also works with HTTP POST.

```sh
curl -H "Content-Type: text/plain" \
    --data-binary @lines.txt \
    http://127.0.0.1:5654/db/tql/script-post-lines.tql
```

### 3. MQTT PUBLISH

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/tql/script-post-lines.tql \
    -f lines.txt
```

Then find if the data was successfully transformed and stored.

```sh
$ machbase-neo shell "select * from example where name like 'text_%'"
 ROWNUM  NAME    TIME(LOCAL)              VALUE     
────────────────────────────────────────────────────
      1  text_3  2023-07-14 08:51:10.926  44.000000 
      2  text_0  2023-07-14 08:51:10.925  11.000000 
      3  text_1  2023-07-14 08:51:10.926  22.000000 
      4  text_2  2023-07-14 08:51:10.926  33.000000 
4 rows fetched.
```