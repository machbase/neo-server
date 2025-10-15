# Machbase Neo TQL SCRIPT Function

TQL supports the `SCRIPT()` function, which utilizes JavaScript (ECMA5) in both **SRC** and **MAP** contexts (Version 8.0.36 or later). This feature offers developers the flexibility to use a familiar programming language, enhancing their ability to create more dynamic and powerful scripts within TQL.

**Syntax**: `SCRIPT({main_code})`

**Syntax**: `SCRIPT({init_code}, {main_code})`

**Syntax**: `SCRIPT({init_code}, {main_code}, {deinit_code})`

**Parameters:**
- `init_code` - Initialize code (optional but mandatory if deinit_code exists)
- `main_code` - Script code (mandatory)
- `deinit_code` - Destruct code (optional)

The `init_code` is optional and runs only once at the beginning. The `main_code` is mandatory and cannot be omitted.

## Caveat Emptor

- `use strict` does nothing.
- ECMA5 only. Some ES6 features e.g. Typed Arrays and back-tick string interpolation are not supported.
- Regular expression is not fully compatible with the ECMA5 specification. The following regular expression syntax is incompatible:
    - `(?=)` Lookahead (positive), it produces a parsing error
    - `(?!)` Lookahead (backhead), it produces a parsing error
    - `\1`, `\2`, `\3`, ... Backreference, it produces a parsing error

## JSH Modules

*Version 8.0.52 or later*

You can import JSH modules into the SCRIPT() using `require()`. All "@jsh" modules are available except `@jsh/process` which is only accessible from inside JSH application.

```js
SCRIPT({
    const { arrange } = require("@jsh/generator")
    arrange(0, 6, 3).forEach((i) =>$.yield(i))
})
CSV()
```

## Context Object

Machbase-neo exposes the `$` variable as the context object. JavaScript can access and yield records and database through this context.

**Properties and Methods:**
- `$.payload` - Input data of the request. Available only when `SCRIPT()` is used as an SRC node; otherwise, it is `undefined`.
- `$.params` - Input query parameters of the request.
- `$.result` - Specifies the names and types of the result columns yielded by the `SCRIPT()` function.
- `$.key`, `$.values` - Javascript access point to the key and values of the current record. It is only available if the `SCRIPT()` is a MAP function.
- `$.yield()` - Yield a new record with values
- `$.yieldKey()` - Yield a new record with key and values
- `$.yieldArray()` - Same as `$.yield()`, but it take only one argument of array type instead of multiple arguments.
- `$.db()` - Returns a new database connection.
- `$.db().query()` - Execute SQL query.
- `$.db().exec()` - Execute non-SELECT SQL.
- `$.request().do()` - Request HTTP to the remote server.

### $.payload

JavaScript can access the input data of the request using `$.payload`. If there is no input data, `$.payload` will be `undefined`. `$.payload` is available only when `SCRIPT()` is used as an SRC node; otherwise, it is `undefined`.

```js
SCRIPT({
    var data = $.payload;
    if (data === undefined) {
        data = '{ "prefix": "name", "offset": 0, "limit": 10}';
    }
    var obj = JSON.parse(data);
    $.yield(obj.prefix, obj.offset, obj.limit);
})
CSV()
```

Call the tql file without any request body which makes the `$.payload` is `undefined`.

```sh
curl -o - -X POST http://127.0.0.1:5654/db/tql/test.tql
```

Then the result is the default values: `name,0,10`.

Call the tql file with a custom data.

```sh
curl -o - -X POST http://127.0.0.1:5654/db/tql/test.tql \
-d '{"prefix":"testing", "offset":10, "limit":10}'
```

Then the result is: `testing,10,10`

### $.params

JavaScript can access the request's query parameters using `$.params`. The value of a query parameter can be accessed in two ways: using dot notation (`$.params.name`) or bracket notation (`$.params["name"]`). Both forms are interchangeable and can be used based on the context or preference.

```js
SCRIPT({
    var prefix = $.params.prefix ? $.params.prefix : "name";
    var offset = $.params.offset ? $.params.offset : 0;
    var limit = $.params.limit ? $.params.limit: 10;
    $.yield(prefix, offset, limit);
})
CSV()
```

Call the tql file without parameters.

```sh
curl -o - -X POST http://127.0.0.1:5654/db/tql/test.tql
```

The result will be the default values: `name,0,10`.

Call the tql file with parameters.

```sh
curl -o - -X POST "http://127.0.0.1:5654/db/tql/test.tql?"\
"prefix=testing&offset=12&limit=20"
```

The result is: `testing,12,20`.

### $.result

Specifies the type of result data that the `SCRIPT` function yields. It works within the init code section, as shown in the example below.

```js
SCRIPT({
    $.result = {
        columns: ["val", "sig"],
        types: ["double", "double"] 
    }
},{
    for (i = 1.0; i <= 5.0; i+=0.03) {
        val = Math.round(i*100)/100;
        sig = Math.sin( 1.2*2*Math.PI*val );
        $.yield( val, sig );
    }
})
JSON()
```

### $.key

Access the key of the current record. This is defined only when `SCRIPT` is used as a MAP function. If `SCRIPT` is used as an SRC function, it will be `undefined`.

```js
SCRIPT({
    for( i = 0; i < 3; i++) {
        $.yieldKey(i, "hello-"+(i+1));
    }
})
SCRIPT({
    $.yieldKey($.key, $.values[0], 'key is '+$.key);
})
CSV()
```

**Output:**

```csv
hello-1,key is 0
hello-2,key is 1
hello-3,key is 2
```

### $.values

Access the values of the current record. This is defined only when `SCRIPT` is used as a MAP function. If `SCRIPT` is used as an SRC function, it will be `undefined`.

```js
SCRIPT({
        $.yield("string", 10, 3.14);
})
SCRIPT({
    $.yield(
        "the first value is "+$.values[0],
        "2nd value is "+$.values[1],
        "3rd is "+$.values[2]
    );
})
CSV()
```

**Output:**

`the first value is string,2nd value is 10,3rd is 3.14`

### $.yield()

Yield the new record to the next step, with the key automatically assigned as a sequentially increasing number.

```js
$.yield(field1, field2, field3);
```

### $.yieldKey()

`yieldKey()` functions similarly to `$.yield()`, with the exception that the first argument specifies the key of the record.

```js
$.yieldKey(key, field1, field2, field3);
```

### $.yieldArray()

*Version 8.0.39 or later*

Yield a record contained in an array. `$.yieldArray()` takes a single array argument representing a record, in contrast to `$.yield()`, which takes variable-length arguments. This is useful when working with arrays.

```js
var arr = [];
for( i = 0; i < unknown; i++) {
    arr.push(field_values[i]);
}
$.yieldArray(arr);
```

### $.db()

Returns a new database connection. The connection provides `query()`, `exec()` functions.

If the option object is specified as a parameter, for example, `$.db({bridge: "sqlite"})`, it returns a new connection to the bridged database instead of the machbase database.

**Option:**

The option parameter is supported (Version 8.0.37 or later)

```js
{
    bridge: "name", // bridge name
}
```

### $.db().query()

JavaScript can query the database using `$.db().query()`. Apply a callback function with `forEach()` to the return value of `query()` to iterate over the query results.

If the callback function of `.forEach()` explicitly returns `false`, the iteration stops immediately. If the callback function returns `true` or does not return anything (which means it returns `undefined`), the iteration continues until the end of the query result.

#### Query MACHBASE

```js
SCRIPT({
  var data = $.payload;
  if (data === undefined) {
    data = '{ "tag": "cpu.percent", "offset": 0, "limit": 3 }';
  }
  var obj = JSON.parse(data);
  $.db()
   .query("SELECT name, time, value FROM example WHERE name = ? LIMIT ?, ?",
    obj.tag, obj.offset, obj.limit
  ).forEach( function(row){
    name = row[0]
    time = row[1]
    value = row[2]
    $.yield(name, time, value);
  })
})
CSV()
```

**Output:**

```csv
cpu.percent,1725330085908925000,73.9
cpu.percent,1725343895315420000,73.6
cpu.percent,1725343898315887000,6.1
```

#### Query BRIDGE-SQLITE

```js
SCRIPT({
  var data = $.payload;
  if (data === undefined) {
    data = '{ "tag": "testing", "offset": 0, "limit": 3 }';
  }
  var obj = JSON.parse(data);
  $.db({bridge:"mem"})
   .query("SELECT name, time, value FROM example WHERE name = ? LIMIT ?, ?",
    obj.tag, obj.offset, obj.limit
  ).forEach( function(row){
    name = row[0]
    time = row[1]
    value = row[2]
    $.yield(name, time, value);
  })
})
CSV()
```

**Output:**

```csv
testing,1732589744886,16.70559756851126
testing,1732589744886,49.93214293713331
testing,1732589744886,54.485508690434905
```

#### Using $.yieldArray()

Choose specific columns from the result of `$.db().query()` to yield using `$.yield()`. Use `$.yieldArray()` (Version 8.0.39 or later) to yield all columns in a time.

```js
SCRIPT({
    var sql = "SELECT name, time, value FROM example WHERE name = 'cpu.percent' LIMIT 3";
    $.db().query(sql).forEach( function(row){
        $.yieldArray(row);
    });
})
CSV()
```

#### Using $.db().query().yield()

Or use `$.db().query().yield()` (Version 8.0.39 or later) to yield automatically.

```js
SCRIPT({
    var tags = ["mem.total", "mem.used", "mem.free"];
    for( i = 0; i < tags.length; i++) {
        $.yield(tags[i]);
    }
})
SCRIPT({
    var sql = "SELECT * FROM example WHERE name = ? LIMIT 1";
    $.db().query(sql, $.values[0]).yield();
})
CSV( header(true) )
```

### $.db().exec()

If the SQL is not a SELECT statement, use `$.db().exec()` to execute INSERT, DELETE, CREATE TABLE statements.

#### Execute on MACHBASE

```js
SCRIPT({
    for( i = 0; i < 3; i++) {
        ts = Date.now()*1000000; // ms to ns
        $.yield("testing", ts, Math.random()*100);
    }
})
SCRIPT({
    // This section contains initialization code
    // that runs once before processing the first record.
    err = $.db().exec("CREATE TAG TABLE IF NOT EXISTS example ("+
        "NAME varchar(80) primary key,"+
        "TIME datetime basetime,"+
        "VALUE double"+
    ")");
    if (err instanceof Error) {
        console.error("Fail to create table", err.message);
    }
}, {
    // This section contains the main code
    // that runs over every record.
    err = $.db().exec("INSERT INTO example values(?, ?, ?)", 
        $.values[0], $.values[1], $.values[2]);
    if (err instanceof Error) {
        console.error("Fail to insert", err.message);
    } else {
        $.yield($.values[0], $.values[1], $.values[2]);
    }
})
CSV()
```

#### Execute on BRIDGE-SQLITE

```js
SCRIPT({
    for( i = 0; i < 3; i++) {
        ts = Date.now(); // ms
        $.yield("testing", ts, Math.random()*100);
    }
})
SCRIPT({
    // This section contains initialization code
    // that runs once before processing the first record.
    err = $.db({bridge:"mem"}).exec("CREATE TABLE IF NOT EXISTS example ("+
        "NAME TEXT,"+
        "TIME INTEGER,"+
        "VALUE REAL"+
    ")");
    if (err instanceof Error) {
        console.error("Fail to create table", err.message);
    }
}, {
    // This section contains the main code
    // that runs over every record.
    err = $.db({bridge:"mem"}).exec("INSERT INTO example values(?, ?, ?)", 
        $.values[0], $.values[1], $.values[2]);
    if (err instanceof Error) {
        console.error("Fail to insert", err.message);
    } else {
        $.yield($.values[0], $.values[1], $.values[2]);
    }
})
CSV()
```

To query a bridged database in the SQL editor, use the `-- env: bridge=name` notation for the query.

```sql
-- env: bridge=mem
SELECT
    name,
    datetime(time/1000, 'unixepoch', 'localtime') as time,
    value
FROM
    example;
-- env: reset
```

### $.request().do()

**Syntax**: `$.request(url [, option]).do(callback)`

**Request Option:**

```js
{
    method: "GET|POST|PUT|DELETE", // default is "GET"
    headers: { "Authorization": "Bearer auth-token"}, // key value map
    body: "body content if the method is POST or PUT"
}
```

The actual request is made when `.then()` is called with a callback function to handle the response. The callback function receives a Response object as an argument, which provides several properties and methods.

**Response Properties:**

| Property | Type | Description |
|:---------|:----:|:------------|
| `.ok` | Boolean | `true` if the status code of the response is success. (`200<= status < 300`) |
| `.status` | Number | http response code |
| `.statusText` | String | status code and message. e.g. `200 OK` |
| `.url` | String | request url |
| `.headers` | Map | response headers |

**Response Methods:**

The Response object provides useful methods that serves the body content of the response.

| Method | Description |
|:-------|:------------|
| `.text(callback(txt))` | Call the callback with content in a string |
| `.blob(callback(bin))` | Call the callback with content in a binary array |
| `.csv(callback(row))` | Parse the content into CSV format and call `callback()` for each row (record). |

**Usage:**

```js
$.request("https://server/path", {
    method: "GET",
    headers: { "Authorization": "Bearer auth-token" }
  }).do( function(rsp){
    console.log("ok:", rsp.ok);
    console.log("status:", rsp.status);
    console.log("statusText:", rsp.statusText);
    console.log("url:", rsp.url);
    console.log("Content-Type:", rsp.headers["Content-Type"]);
});
```

### finalize()

If the JavaScript code in `SCRIPT()` defines a `function finalize() {}`, the system will automatically call this function after all records have been processed.

The following two code examples are equivalent; both yield `999` as the final record.

**Using finalize() function:**

```js
FAKE( arrange(1, 3, 1) )
SCRIPT({
    function finalize() {
        $.yield(999);
    }
    $.yield($.values[0]);
})
CSV()
```

**Using deinit code:**

```js
FAKE( arrange(1, 3, 1) )
SCRIPT({
    // init; do nothing
},{
    // main
    $.yield($.values[0]);
}, {
    // deinit;
    $.yield(999);
})
CSV()
```

This example yields 4 records: `1`,`2`,`3`,`999`.

## Examples

### Hello World

```js
SCRIPT({
    console.log("Hello World?");
})
DISCARD()
```

### Builtin Math Object

#### Using JavaScript

Javascript builtin functions are available:

```js
FAKE(meshgrid(linspace(0,2*3.1415,30), linspace(0, 3.1415, 20)))

SCRIPT({
  x = Math.cos($.values[0]) * Math.sin($.values[1]);
  y = Math.sin($.values[0]) * Math.sin($.values[1]);
  z = Math.cos($.values[1]);
  $.yield([x,y,z]);
})

CHART(
  plugins("gl"),
  size("600px", "600px"),
  chartOption({
    grid3D:{}, xAxis3D:{}, yAxis3D:{}, zAxis3D:{},
    visualMap:[{  min:-1, max:1, 
      inRange:{color:["#313695",  "#74add1", "#ffffbf","#f46d43", "#a50026"]
    }}],
    series:[ { type:"scatter3D", data: column(0)} ]
  })
)
```

#### Using SET-MAP Functions

The equivalent result using SET-MAP functions instead of Javascript is:

```js
FAKE(meshgrid(linspace(0,2*3.1415,30), linspace(0, 3.1415, 20)))

SET(x, cos(value(0))*sin(value(1)))
SET(y, sin(value(0))*sin(value(1)))
SET(z, cos(value(1)))

MAPVALUE(0, list($x, $y, $z))
POPVALUE(1)

CHART(
  plugins("gl"),
  size("600px", "600px"),
  chartOption({
    grid3D:{}, xAxis3D:{}, yAxis3D:{}, zAxis3D:{},
    visualMap:[{  min:-1, max:1, 
      inRange:{color:["#313695",  "#74add1", "#ffffbf","#f46d43", "#a50026"]
    }}],
    series:[ { type:"scatter3D", data: column(0)} ]
  })
)
```

### JSON Parser

```js
SCRIPT({
    $.result = {
        columns: ["NAME", "AGE", "IS_MEMBER", "HOBBY"],
        types: ["string", "int32", "bool", "string"],
    }
},{
    content = $.payload;
    if (content === undefined) {
        content = '{"name":"James", "age": 24, "isMember": true, "hobby": ["book", "game"]}';
    }
    obj = JSON.parse(content);
    $.yield(obj.name, obj.age, obj.isMember, obj.hobby.join(","));
})
JSON()
```

**Output:**

```json
{
    "data": {
        "columns": [ "NAME", "AGE", "IS_MEMBER", "HOBBY" ],
        "types": [ "string", "int32", "bool", "string" ],
        "rows": [ [ "James", 24, true, "book,game" ] ]
    },
    "success": true,
    "reason": "success",
    "elapse": "627.958µs"
}
```

### Request CSV

```js
SCRIPT({
    $.result = {
        columns: ["SepalLen", "SepalWidth", "PetalLen", "PetalWidth", "Species"],
        types: ["double", "double", "double", "double", "string"]
    };
},{
    $.request("https://docs.machbase.com/assets/example/iris.csv")
     .do(function(rsp){
        console.log("ok:", rsp.ok);
        console.log("status:", rsp.status);
        console.log("statusText:", rsp.statusText);
        console.log("url:", rsp.url);
        console.log("Content-Type:", rsp.headers["Content-Type"]);
        if ( rsp.error() !== undefined) {
            console.error(rsp.error())
        }
        var err = rsp.csv(function(fields){
            $.yield(fields[0], fields[1], fields[2], fields[3], fields[4]);
        })
        if (err !== undefined) {
            console.warn(err);
        }
    })
})
CSV(header(true))
```

### Request JSON Text

This example demonstrates how to fetch JSON content from a remote server and parse it using Javascript.

```js
SCRIPT({
    $.result = {
        columns: ["ID", "USER_ID", "TITLE", "COMPLETED"],
        types: ["int64", "int64", "string", "boolean"]
    };
},{
    $.request("https://jsonplaceholder.typicode.com/todos")
     .do(function(rsp){
        console.log("ok:", rsp.ok);
        console.log("status:", rsp.status);
        console.log("statusText:", rsp.statusText);
        console.log("URL:", rsp.url);
        console.log("Content-Type:", rsp.headers["Content-Type"]);
        if ( rsp.error() !== undefined) {
            console.error(rsp.error())
        }
        rsp.text( function(txt){
            list = JSON.parse(txt);
            for (i = 0; i < list.length; i++) {
                obj = list[i];
                $.yield(obj.id, obj.userId, obj.title, obj.completed);
            }
        })
    })
})
CSV(header(false))
```