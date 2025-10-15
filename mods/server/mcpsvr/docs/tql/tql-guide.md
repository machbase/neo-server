# Machbase Neo TQL Guide

## What is TQL?

Machbase Neo supports TQL (Transforming Query Language) and execution API.

As application developers, we typically follow a similar approach to build applications that utilize databases. The process usually starts with querying the database and retrieving data in a tabular form (rows and columns). This data is then converted into the desired data structure, manipulated, and displayed in the required formats such as JSON, CSV, or charts.

TQL simplifies this process with just a few lines of script. Additionally, other applications can interact with TQL via HTTP endpoints, treating it as an executable API.

## TQL Concepts

TQL (Transforming Query Language) is a domain-specific language (DSL) for data manipulation. It defines a flow of data streams, where each individual data unit is a record. A record has a key and a value. The key is generally an auto-generated sequential integer, similar to ROWNUM in query results. The value is a tuple that contains the actual data fields.

TQL scripts start with SRC (source) functions that define how to retrieve data and generate records by transforming the raw data. The script ends with SINK functions, which define how to output the records. Between SRC and SINK, you can use MAP functions to transform the data as needed.

In some cases, a TQL script needs to transform records, involving mathematical calculations, simple string concatenations, or interactions with external databases. These tasks can be defined using MAP functions.

Thus, a TQL script should start with SRC functions and end with SINK functions. It can include zero or more MAP functions in between to perform the necessary transformations.

### SRC Functions

There are several SRC functions available in TQL. For example, the `SQL()` function produces records by querying the Machbase Neo database or even external (bridged) databases using the given SQL statement. The `FAKE()` function generates artificial data for testing purposes. The `CSV()` function reads data from CSV files, while the `BYTES()` function reads arbitrary binary data from the file system, client's HTTP request, or MQTT payload.

### SINK Functions

The basic SINK functions include `INSERT()`, which writes the incoming records to the Machbase Neo database, and `CHART()`, which renders a chart with the incoming records. Additionally, `JSON()` and `CSV()` functions encode the incoming data into their respective formats, making it easier to integrate with other applications or display the data in a user-friendly manner.

### MAP Functions

MAP functions are essential for transforming data from one shape to another. They allow you to perform various operations such as mathematical calculations, string manipulations, and data format conversions. By using MAP functions, you can efficiently process and reshape your data to meet the specific requirements of your application.

## Output Format Independence

You can generate various output formats from the same data source:

**CSV Format:**
```js
SQL( `SELECT TIME, VALUE FROM EXAMPLE WHERE NAME='signal' LIMIT 100` )
CSV( timeformat("Default") )
```

**JSON Format:**
```js
SQL( `SELECT TIME, VALUE FROM EXAMPLE WHERE NAME='signal' LIMIT 100` )
JSON( timeformat("Default") )
```

**CHART Format:**
```js
SQL( `SELECT TIME, VALUE FROM EXAMPLE WHERE NAME='signal' LIMIT 100` )
CHART(
    size("600px", "340px"),
    chartOption({
        xAxis:{data:column(0)},
        yAxis:{},
        series:[ { type:"line", data:column(1)} ]
    })
)
```

**HTML Format:**
```html
SQL(`SELECT TIME, VALUE FROM EXAMPLE WHERE NAME='signal' LIMIT 100`)
HTML({
  {{if .IsFirst }}
    <table>
    <tr>
        <th>TIME</th><th>VALUE</th>
    </tr>
  {{end}}
    <tr>
        <td>{{.V.TIME}}</td><td>{{.V.VALUE}}</td>
    </tr>
  {{if .IsLast }}
    </table>
  {{end}}
})
```

## Data Source Independence

You can apply the same transformation logic to various data sources:

**JSON Data:**
```js
FAKE( json({ 
    [ "A", 1.0 ],
    [ "B", 1.5 ],
    [ "C", 2.0 ],
    [ "D", 2.5 ] }))

MAPVALUE(1, value(1) * 10 )

CSV()
```

**CSV Data:**
```js
CSV(`A,1.0
B,1.5
C,2.0
D,2.5`, field(1, floatType(), "value"))

MAPVALUE(1, value(1) * 10 )

CSV()
```

**SQL Query:**
```js
SQL(`select time, value from example where name = 'my-car' limit 4`)

MAPVALUE(1, value(1) * 10 )

CSV()
```

**Script - JSON Parsing:**
```js
SCRIPT({
    list = JSON.parse(`[["A",1.0], ["B",1.5], ["C",2.0], ["D",2.5]]`);
    for( v of list) {
        $.yield(v[0], v[1])
    }
})
MAPVALUE(1, value(1) * 10 )
CSV()
```

**Script - For Loop:**
```js
SCRIPT({
    for (i = 0; i < 10; i++) {
        $.yield("script", Math.random())
    }
})

MAPVALUE(1, value(1) * 10 )

CSV()
```

The purpose of TQL is transforming data format. This chapter shows how to do this without developing additional applications.

## N:M Transforming

(Content about N:M transforming would be here from original document)

## Iris Dataset Examples

The example TQL code below gives a brief idea of what TQL is for.

### Average Values

Calculate average values of each classes:

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
GROUP( by(value(4), "species"),
    avg(value(0), "Avg. Sepal L."),
    avg(value(1), "Avg. Sepal W."),
    avg(value(2), "Avg. Petal L."),
    avg(value(3), "Avg. Petal W.")
)
CHART(
    chartOption({
        "xAxis":{"type": "category", "data": column(0)},
        "yAxis": {},
        "legend": {"show": true},
        "series": [
            { "type": "bar", "name": "Avg. Sepal L.", "data": column(1)},
            { "type": "bar", "name": "Avg. Sepal W.", "data": column(2)},
            { "type": "bar", "name": "Avg. Petal L.", "data": column(3)},
            { "type": "bar", "name": "Avg. Petal W.", "data": column(4)}
        ]
    })
)
```

### Statistical Analysis

Calculate min, median, avg, max, stddev of sepal length of the setosa class:

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
FILTER( strToUpper(value(4)) == "IRIS-SETOSA")
GROUP( by(value(4)), 
    min(value(0), "Min"),
    median(value(0), "Median"),
    avg(value(0), "Avg"),
    max(value(0), "Max"),
    stddev(value(0), "StdDev.")
)
CHART(
    chartOption({
        "xAxis": { "type": "category", "data": ["iris-setosa"]},
        "yAxis": {},
        "legend": {"show": "true"},
        "series": [
            {"type":"bar", "name": "Min", "data": column(1)},
            {"type":"bar", "name": "Median", "data": column(2)},
            {"type":"bar", "name": "Avg", "data": column(3)},
            {"type":"bar", "name": "Max", "data": column(4)},
            {"type":"bar", "name": "StdDev.", "data": column(5)}
        ]
    })
)
```

### Bar Chart with Script

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
SCRIPT({
    var board = {};
},{
    species = $.values[4];
    o = board[species];
    if(o === undefined) {
        o = {
            sepalLength: [],
            sepalWidth: [],
            petalLength: [],
            petalWidth: [],
        };
        board[species] = o;
    }
    o.sepalLength.push(parseFloat($.values[0]));
    o.sepalWidth.push(parseFloat($.values[1]));
    o.petalLength.push(parseFloat($.values[2]));
    o.petalWidth.push(parseFloat($.values[3]))
},{
    chart = {
        xAxis: {type: "category", data:[]},
        yAxis: {},
        legend: {show:true},
        series: [
            {type: "bar", name: "min. sepal L.", data:[]},
            {type: "bar", name: "max. sepal L.", data:[]},
            {type: "bar", name: "min. sepal W.", data:[]},
            {type: "bar", name: "max. sepal W.", data:[]},
            {type: "bar", name: "min. petal L.", data:[]},
            {type: "bar", name: "max. petal L.", data:[]},
            {type: "bar", name: "min. petal W.", data:[]},
            {type: "bar", name: "max. petal W.", data:[]},
        ],
    };
    for( s in board) {
        o = board[s];
        chart.xAxis.data.push(s);
        chart.series[0].data.push(Math.min(...o.sepalLength));
        chart.series[1].data.push(Math.max(...o.sepalLength));
        chart.series[2].data.push(Math.min(...o.sepalWidth));
        chart.series[3].data.push(Math.max(...o.sepalWidth));
        chart.series[4].data.push(Math.min(...o.petalLength));
        chart.series[5].data.push(Math.max(...o.petalLength));
        chart.series[6].data.push(Math.min(...o.petalWidth));
        chart.series[7].data.push(Math.max(...o.petalWidth));
    }
    $.yield(chart);
})
CHART()
```

### Boxplot with Script

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
SCRIPT({
    var board = {};
},{
    species = $.values[4];
    o = board[species];
    if(o === undefined) {
        o = {
            sepalLength: [],
            sepalWidth: [],
            petalLength: [],
            petalWidth: [],
        };
        board[species] = o;
    }
    o.sepalLength.push(parseFloat($.values[0]));
    o.sepalWidth.push(parseFloat($.values[1]));
    o.petalLength.push(parseFloat($.values[2]));
    o.petalWidth.push(parseFloat($.values[3]))
},{
    chart = {
        title: {text: "Iris Sepal/Petal Length", left: "center"},
        grid: {bottom: "10%"},
        xAxis: {type: "category", data:[], boundaryGap: true},
        yAxis: {type: "value", splitArea:{show:true}},
        legend: {show:true, bottom:"2%"},
        tooltip: {trigger: "item", axisPointer:{type:"shadow"}},
        series: [
            {type: "boxplot", name: "sepal length", data:[]},
            {type: "boxplot", name: "petal length", data:[]},
        ],
    };
    const ana = require("@jsh/analysis");
    for( s in board) {
        o = board[s];
        chart.xAxis.data.push(s);
        // sepal length
        o.sepalLength = ana.sort(o.sepalLength)
        min = Math.min(...o.sepalLength);
        max = Math.max(...o.sepalLength);
        q1 = ana.quantile(0.25, o.sepalLength);
        q2 = ana.quantile(0.5, o.sepalLength);
        q3 = ana.quantile(0.75, o.sepalLength);
        chart.series[0].data.push([min, q1, q2, q3, max]);
        // petal length
        o.petalLength = ana.sort(o.petalLength)
        min = Math.min(...o.petalLength);
        max = Math.max(...o.petalLength);
        q1 = ana.quantile(0.25, o.petalLength);
        q2 = ana.quantile(0.5, o.petalLength);
        q3 = ana.quantile(0.75, o.petalLength);
        chart.series[1].data.push([min, q1, q2, q3, max]);
    }
    $.yield(chart);
})
CHART()
```

## Running TQL

### Step 1: Open Web UI

Open the Machbase Neo web UI in your web browser. The default address is `http://127.0.0.1:5654/`. Use the username `sys` and the password `manager`.

### Step 2: New TQL Page

Select "TQL" on the 'New...' page.

### Step 3: Copy the Code and Run

Copy and paste the sample TQL code into the TQL editor.

Then click the ▶︎ icon on the top left of the editor. It will display a line chart as shown in the image below, representing a wave with a frequency of 1.5 Hz and an amplitude of 1.0.

**SCATTER Chart:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
CHART_SCATTER()
```

**LINE Chart:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
CHART_LINE()
```

**BAR Chart:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
CHART_BAR()
```

### Exploring Data Formats

Let's explore some data formats like CSV and JSON.

- **CSV Format**: This format is useful for exporting data to spreadsheets or other applications that support CSV files.
- **JSON Format**: This format is ideal for web applications and APIs, as it is easy to parse and integrate with JavaScript.

By using TQL, you can easily convert your data into these formats with just a few lines of code.

**JSON - Rows Format:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
JSON()
```

**JSON - Columns Format:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
JSON( transpose(true) )
```

**CSV Format:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
CSV()
```

**MARKDOWN Format:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
MARKDOWN()
```

**HTML Format:**
```js
FAKE( oscillator(freq(1.5, 1.0), range('now', '3s', '10ms')) )
MARKDOWN( html(true) )
```

## TQL as API

Save this code as `hello.tql` by clicking the save icon at the top right corner of the editor. You can then access it through your web browser at http://127.0.0.1:5654/db/tql/hello.tql, or use the curl command in the terminal to retrieve the data.

> When a TQL script is saved, the editor displays a link icon at the top right corner. Click this icon to copy the address of the script file.

```sh
curl -o - http://127.0.0.1:5654/db/tql/hello.tql
```

Execution result:
```sh
$ curl -o - -v http://127.0.0.1:5654/db/tql/hello.tql
...omit...
>
< HTTP/1.1 200 OK
< Content-Type: text/csv
< Transfer-Encoding: chunked
<
1686787739025518000,-0.238191
1686787739035518000,-0.328532
1686787739045518000,-0.415960
1686787739055518000,-0.499692
1686787739065518000,-0.578992
...omit...
```

### JSON Output

Let's try to change `CSV()` to `JSON()`, save and execute again.

And invoke the hello.tql with curl from terminal:
```sh
$ curl -o - -v http://127.0.0.1:5654/db/tql/hello.tql
...omit...
< HTTP/1.1 200 OK
< Content-Type: application/json
< Transfer-Encoding: chunked
<
{
"data": {
    "columns": [ "time", "value" ],
    "types": [ "datetime", "double" ],
    "rows": [
    [ 1686788907538618000, 0.9344920354538058 ],
    [ 1686788907548618000, 0.8968436523101743 ],
    ...omit...
},
"success": true,
"reason": "success",
"elapse": "956.291µs"
}
```

### JSON with transpose()

If you are developing a data visualization application, it is helpful to know that TQL's JSON output supports transposing the result into columns instead of rows. By applying `JSON( transpose(true) )` and invoking it again, the resulting JSON will contain a `cols` array.

```sh
$ curl -o - -v http://127.0.0.1:5654/db/tql/hello.tql
...omit...
< HTTP/1.1 200 OK
< Content-Type: application/json
< Transfer-Encoding: chunked
<
{
"data": {
    "columns": [ "time", "value" ],
    "types": [ "datetime", "double" ],
    "cols": [
        [ 1686789517241103000, ...omit..., 1686789520231103000],
        [ -0.7638449771082523, ...omit..., 0.8211935584502427]
    ]
},
"success": true,
"reason": "success",
"elapse": "1.208166ms"
}
```

This feature is the simplest way for developers to create RESTful APIs, allowing other applications to access data seamlessly.

### INSERT Data

Change `CSV()` to `INSERT("time", "value", table("example"), tag("temperature"))` and execute again.

### Select Table

```js
SQL('select * from tag limit 10')
CSV()
```