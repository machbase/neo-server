# Machbase Neo TQL Sink Functions

All TQL scripts must end with one of the sink functions.

The basic SINK function might be `INSERT()` which write the incoming records onto machbase-neo database. `CHART()` function can render various charts with incoming records. `JSON()` and `CSV()` encode incoming data into proper formats.

## INSERT()

**Syntax**: `INSERT( [bridge(),] columns..., table() [, tag()] )`

`INSERT()` stores incoming records into specified database table by an 'INSERT' statement for each record.

**Parameters:**
- `bridge()` - bridge('name'), optional
- `columns` - String, column list
- `table()` - table('name'), specify the destination table name
- `tag()` - tag('name'), optional, applicable only to tag tables

### Example: Basic INSERT

Write records to machbase that contains tag name.

```js
FAKE(json({
    ["temperature", 1708582790, 23.45],
    ["temperature", 1708582791, 24.56]
}))
MAPVALUE(1, value(1)*1000000000) // convert epoch sec to nanosec
INSERT("name", "time", "value", table("example"))
```

### Example: Using PUSHVALUE()

Write records to machbase with same tag name by adding "name" field by `PUSHVALUE()`.

```js
FAKE(json({
    [1708582792, 32.34],
    [1708582793, 33.45]
}))
PUSHVALUE(0, "temperature")
MAPVALUE(1, value(1)*1000000000) // convert epoch sec to nanosec
INSERT("name","time", "value", table("example"))
```

### Example: Using tag()

Write records to machbase with same tag name by using `tag()` option if the destination is a tag table.

```js
FAKE(json({
    [1708582792, 32.34],
    [1708582793, 33.45]
}))
MAPVALUE(0, value(0)*1000000000) // convert epoch sec to nanosec
INSERT("time", "value", table("example"), tag('temperature'))
```

### Example: Bridge Database

Insert records into bridged database.

```js
INSERT(
    bridge("sqlite"),
    "company", "employee", "created_on", table("mem_example")
)
```

## APPEND()

**Syntax**: `APPEND( table() )`

`APPEND()` stores incoming records into specified database table via the 'append' method of machbase-neo.

**Parameters:**
- `table()` - table(string), specify destination table

```js
FAKE(json({
    ["temperature", 1708582794, 12.34],
    ["temperature", 1708582795, 13.45]
}))
MAPVALUE(1, value(1)*1000000000 ) // convert epoch sec to nanosec
APPEND( table("example") )
```

## CSV()

**Syntax**: `CSV( [tz(), timeformat(), precision(), rownum(), heading(), delimiter(), nullValue() ] )`

Makes the records of the result in CSV format. The values of the records become the fields of the CSV lines. The end of the data is identified by the last two consecutive newline characters (`\n\n`).

For example, if a record was `{key: k, value:[v1,v2]}`, it generates an CSV records as `v1,v2`.

**Parameters:**
- `tz` - tz(name), time zone, default is `tz('UTC')`
- `timeformat` - timeformat(string), specify the format how represents datetime fields, default is `timeformat('ns')`
- `rownum` - rownum(boolean), adds rownum column
- `precision` - precision(int), specify precision of float fields, `precision(-1)` means no restriction, `precision(0)` converts to integer
- `heading` - heading(boolean), add fields names as the first row
- `delimiter` - delimiter(string), specify fields separator other than the default comma(`,`)
- `nullValue()` - Specify substitution string for the NULL value, default is `nullValue('NULL')` (Version 8.0.14 or later)
- `substituteNull` - substitute(string), specify substitution string for the NULL value, default is `substituteNull('NULL')` (deprecated, replaced by `nullValue()`)
- `cache()` - Cache result data. See Cache Result Data for details (Version 8.0.43 or later)

### Example: Default Output

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10)
CSV()
```

**Output:**

```csv
1,10
2,20
3,30
```

### Example: With heading()

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10, "x10")
CSV( heading(true) )
```

**Output:**

```csv
x,x10
1,10
2,20
3,30
```

### Example: With delimiter()

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10, "x10")
CSV( heading(true), delimiter("|") )
```

**Output:**

```csv
x|x10
1|10
2|20
3|30
```

### Example: With nullValue()

```js
FAKE( json({ ["A", 123], ["B", null], ["C", 234] }) )
CSV( nullValue("***") )
```

**Output:**

```csv
A|123
B|***
C|234
```

## JSON()

**Syntax**: `JSON( [transpose(), tz(), timeformat(), precision(), rownum(), rowsFlatten(), rowsArray() ] )`

Generates JSON results from the values of the records.

**Parameters:**
- `transpose` - transpose(boolean), transpose rows and columns, it is useful that specifying `transpose(true)` for the most of chart libraries
- `tz` - tz(name), time zone, default is `tz('UTC')`
- `timeformat` - timeformat(string), specify the format how represents datetime fields, default is `timeformat('ns')`
- `rownum` - rownum(boolean), adds rownum column
- `precision` - precision(int), specify precision of float fields, `precision(-1)` means no restriction, `precision(0)` converts to integer
- `rowsFlatten` - rowsFlatten(boolean), reduces the array dimension of the rows field in the JSON object. If `JSON()` has `transpose(true)` and `rowsFlatten(true)` together, it ignores `rowsFlatten(true)` and only `transpose(true)` affects on the result (Version 8.0.12 or later)
- `rowsArray` - rowsArray(boolean), produces JSON that contains only array of object for each record. The `rowsArray(true)` has higher priority than `transpose(true)` and `rowsFlatten(true)` (Version 8.0.12 or later)
- `cache()` - Cache result data. See Cache Result Data for details (Version 8.0.43 or later)

### Example: Default Output

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10)
JSON()
```

**Output:**

```json
{
    "data": {
        "columns": [ "x" ],
        "types": [ "double" ],
        "rows": [ [ 1, 10 ], [ 2, 20 ], [ 3, 30 ] ]
    },
    "success": true,
    "reason": "success",
    "elapse": "228.541µs"
}
```

### Example: With transpose()

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10, "x10")
JSON( transpose(true) )
```

**Output:**

```json
{
    "data": {
        "columns": [ "x", "x10" ],
        "types": [ "double", "double" ],
        "cols": [ [ 1, 2, 3 ], [ 20, 30, 40 ] ]
    },
    "success": true,
    "reason": "success",
    "elapse": "121.375µs"
}
```

### Example: With rowsFlatten()

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10, "x10")
JSON( rowsFlatten(true) )
```

**Output:**

```json
{
    "data": {
        "columns": [ "x", "x10" ],
        "types": [ "double", "double" ],
        "rows": [ 1, 10, 2, 20, 3, 30 ]
    },
    "success": true,
    "reason": "success",
    "elapse": "130.916µs"
}
```

### Example: With rowsArray()

```js
FAKE( arrange(1, 3, 1))
MAPVALUE(1, value(0)*10, "x10")
JSON( rowsArray(true) )
```

**Output:**

```json
{
    "data": {
        "columns": [ "x", "x10" ],
        "types": [ "double", "double" ],
        "rows": [ { "x": 1, "x10": 10 }, { "x": 2, "x10": 20 }, { "x": 3, "x10": 30 } ]
    },
    "success": true,
    "reason": "success",
    "elapse": "549.833µs"
}
```

## NDJSON()

**Syntax**: `NDJSON( [tz(), timeformat(), rownum()] )`

*Version 8.0.33 or later*

Generates NDJSON results from the values of the records.

NDJSON (Newline Delimited JSON) is a format for streaming JSON data where each line is a valid JSON object. This is useful for processing large datasets or streaming data because it allows you to handle one JSON object at a time. The end of the data is identified by the last two consecutive newline characters (`\n\n`).

**Parameters:**
- `tz` - tz(name), time zone, default is `tz('UTC')`
- `timeformat` - timeformat(string), specify the format how represents datetime fields, default is `timeformat('ns')`
- `rownum` - rownum(boolean), adds rownum column
- `cache()` - Cache result data. See Cache Result Data for details (Version 8.0.43 or later)

**Example:**

```js
SQL(`select * from example where name = 'neo_load1' limit 3`)
NDJSON(timeformat('Default'), tz('local'), rownum(true))
```

**Output:**

```json
{"NAME":"neo_load1","ROWNUM":1,"TIME":"2024-09-06 14:46:19.852","VALUE":4.58}
{"NAME":"neo_load1","ROWNUM":2,"TIME":"2024-09-06 14:46:22.853","VALUE":4.69}
{"NAME":"neo_load1","ROWNUM":3,"TIME":"2024-09-06 14:46:25.852","VALUE":4.69}

```

## MARKDOWN()

Generates a table in markdown format or HTML.

**Syntax**: `MARKDOWN( [ options... ] )`

**Parameters:**
- `tz(string)` - Time zone, default is `tz('UTC')`
- `timeformat(string)` - Specify the format how represents datetime fields, default is `timeformat('ns')`
- `html(boolean)` - Produce result by HTML renderer, default `false`
- `rownum(boolean)` - Show rownum column
- `precision` - precision(int), specify precision of float fields, `precision(-1)` means no restriction, `precision(0)` converts to integer
- `brief(boolean)` - Omit result rows, `brief(true)` is equivalent with `briefCount(5)`
- `briefCount(limit int)` - Omit result rows if the records exceeds the given limit, no omission if limit is `0`

### Example: Default Output

```js
FAKE( csv(`
10,The first line 
20,2nd line
30,Third line
40,4th line
50,The last is 5th
`))
MARKDOWN()
```

**Output:**

```
|column0 |	column1 |
|:-------|:---------|
| 10     | The first line |
| 20     | 2nd line |
| 30     | Third line |
| 40     | 4th line |
| 50     | The last is 5th |
```

### Example: With briefCount

```js
FAKE( csv(`
10,The first line 
20,2nd line
30,Third line
40,4th line
50,The last is 5th
`))
MARKDOWN( briefCount(2) )
```

**Output:**

```
|column0 |	column1 |
|:-------|:---------|
| 10     | The first line |
| 20     | 2nd line |
| ...    | ...      |

> Total 5 records
```

### Example: With html()

```js
FAKE( csv(`
10,The first line 
20,2nd line
30,Third line
40,4th line
50,The last is 5th
`))
MARKDOWN( briefCount(2), html(true) )
```

**Output:**

|column0 |	column1 |
|:-------|:---------|
| 10     | The first line |
| 20     | 2nd line |
| ...    | ...      |

> Total 5 records

## HTML()

**Syntax**: `HTML(templates...)`

*Version 8.0.52 or later*

Generates an HTML document using the provided templates.

For detailed usage and examples, refer to the HTML section.

## TEXT()

**Syntax**: `TEXT(templates...)`

*Version 8.0.52 or later*

Generates a text document using the provided templates.

It functions similarly to `HTML()`, but does not perform HTML escaping on the data.

## DISCARD()

**Syntax**: `DISCARD()`

*Version 8.0.7 or later*

`DISCARD()` silently ignore all records as its name implies, so that no output generates.

```js
FAKE( json({
    [ 1, "hello" ],
    [ 2, "world" ]
}))
WHEN( value(0) == 2, do( value(0), strToUpper(value(1)), {
    ARGS()
    WHEN( true, doLog("OUTPUT:", value(0), value(1)) )
    DISCARD()
}))
CSV()
```

## CHART()

**Syntax**: `CHART()`

*Version 8.0.8 or later*

Generates chart using Apache echarts.

Refer to CHART() examples for the various usages.

##### Using CHART()

```js
FAKE( oscillator(freq(1.5, 1.0), freq(1.0, 0.7), range('now', '3s', '25ms')))
// |    0      1
// +--> time   value
// |
CHART(
    size("600px", "400px"),
    chartOption({
        xAxis: { name: "T", type:"time" },
        yAxis: { name: "V"},
        legend: { show: true },
        tooltip: { show: true, trigger: "axis" },
        series: [{ 
            type: "line",
            name: "column[1]",
            data: column(0).map(function(t, idx){
                return [t, column(1)[idx]];
            })
        }]
    })
)
```

```js
FAKE( oscillator(freq(1.5, 1.0), freq(1.0, 0.7), range('now', '3s', '25ms')))
// |    0      1
// +--> time   value
// |
CHART(
    size("600px", "400px"),
    chartOption({
        xAxis: { name: "T", type:"time" },
        yAxis: { name: "V"},
        legend: { show: true },
        tooltip: { show: true, trigger: "axis" },
        series: [{ 
            type: "bar",
            name: "column[1]",
            data: column(0).map(function(t, idx){
                return [t, column(1)[idx]];
            })
        }]
    })
)
```

```js
FAKE( oscillator(freq(1.5, 1.0), freq(1.0, 0.7), range('now', '3s', '25ms')))
// |    0      1
// +--> time   value
// |
CHART(
    size("600px", "400px"),
    chartOption({
        xAxis: { name: "T", type:"time" },
        yAxis: { name: "V"},
        legend: { show: true },
        tooltip: { show: true, trigger: "axis" },
        series: [{ 
            type: "scatter",
            name: "column[1]",
            data: column(0).map(function(t, idx){
                return [t, column(1)[idx]];
            })
        }]
    })
)
```

```js
FAKE(meshgrid(linspace(-1.0,1.0,100), linspace(-1.0, 1.0, 100)))
// |    0   1
// +--> x   y
// |
MAPVALUE(2, sin(10*(pow(value(0), 2) + pow(value(1), 2))) / 10 )
// |    0   1   2
// +--> x   y   z
// |
CHART(
  plugins("gl"),
  size('600px', '600px'),
  chartOption({
    grid3D:{ boxWidth: 100, boxHeight: 30, boxDepth: 100},
    xAxis3D:{name:"x"},
    yAxis3D:{name:"y"},
    zAxis3D:{name:"z"},
    series:[{
        type: "line3D",
        lineStyle: { "width": 2 },
        data: column(0).map(function(x, idx){
            return [x, column(1)[idx], column(2)[idx]]
        })
    }],
    visualMap: {
        min: -0.12, max:0.12,
        inRange: {
            color:["#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"]
        }
    }
  })
)
```

```js
FAKE(meshgrid(linspace(-1.0,1.0,100), linspace(-1.0, 1.0, 100)))
// |    0   1
// +--> x   y
// |
MAPVALUE(2, sin(10*(pow(value(0), 2) + pow(value(1), 2))) / 10 )
// |    0   1   2
// +--> x   y   z
// |
CHART(
  plugins("gl"),
  size('600px', '600px'),
  chartOption({
    grid3D:{ boxWidth: 100, boxHeight: 30, boxDepth: 100},
    xAxis3D:{name:"x"},
    yAxis3D:{name:"y"},
    zAxis3D:{name:"z"},
    series:[{
        type: "bar3D",
        data: column(0).map(function(x, idx){
            return [x, column(1)[idx], column(2)[idx]]
        })
    }],
    visualMap: {
        min: -0.12, max:0.12,
        inRange: {
            color:["#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"]
        }
    }
  })
)
```

```js
FAKE(meshgrid(linspace(-1.0,1.0,100), linspace(-1.0, 1.0, 100)))
// |    0   1
// +--> x   y
// |
MAPVALUE(2, sin(10*(pow(value(0), 2) + pow(value(1), 2))) / 10 )
// |    0   1   2
// +--> x   y   z
// |
CHART(
  plugins("gl"),
  size('600px', '600px'),
  chartOption({
    grid3D:{ boxWidth: 100, boxHeight: 30, boxDepth: 100},
    xAxis3D:{name:"x"},
    yAxis3D:{name:"y"},
    zAxis3D:{name:"z"},
    series:[{
        type: "scatter3D",
        data: column(0).map(function(x, idx){
            return [x, column(1)[idx], column(2)[idx]]
        })
    }],
    visualMap: {
        min: -0.12, max:0.12,
        inRange: {
            color:["#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
		    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"]
        }
    }
  })
)
```