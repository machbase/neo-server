# Machbase Neo Import & Export Guide

## Import CSV

```sh
curl -o - https://docs.machbase.com/assets/example/example.csv.gz | \
machbase-neo shell import   \
    --input -               \
    --compress gzip         \
    --timeformat s          \
    EXAMPLE
```
The command above is downloading a compressed csv file from the remote web server by `curl`.
It writes out data (compressed, binary) into its stdout stream because we have set `-o -` option, then the output stream is passed to `machbase-neo shell import`, it reads data from stdout by flag `--input -`.

Combining two commands with pipe `|`, so that we don't need to store the data in a temporary file consuming the local storage.

The result output shows that 1,000 records are imported.

```
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  5352  100  5352    0     0   547k      0 --:--:-- --:--:-- --:--:-- 5226k
import total 1000 record(s) inserted
```

Or, we can download data file in the local storage then import from it.

```sh
curl -o data.csv.gz https://docs.machbase.com/assets/example/example.csv.gz
```

It is possible to import compressed or uncompressed csv file.

Then import csv file from local storage with `--input <file>` flag. And use `--compress gzip` option if the file is gzip'd form.

```sh
machbase-neo shell import \
    --input ./data.csv    \
    --timeformat s        \
    EXAMPLE
```

Query the table to check.

```sh
machbase-neo shell sql "select * from example order by time desc limit 5"
```
```
 ROWNUM  NAME      TIME(UTC)            VALUE     
──────────────────────────────────────────────────
 1       wave.sin  2023-02-15 03:47:50  0.994540  
 2       wave.cos  2023-02-15 03:47:50  -0.104353 
 3       wave.sin  2023-02-15 03:47:49  0.951002  
 4       wave.cos  2023-02-15 03:47:49  0.309185  
 5       wave.cos  2023-02-15 03:47:48  0.669261  
```

The sample file contains total 1,000 records and the table contains all of them after importing.

```sh
machbase-neo shell sql "select count(*) from example"
```
```
 ROWNUM  COUNT(*) 
──────────────────
 1       1000     
```

## Export CSV

Exporting table is straightforward. Set `--output` flag for a file path where to save the data.
`--format csv` makes machbase-neo to export data in csv format.
`--timeformat ns` makes any datetime fields in output will be expressed in Unix epoch nanoseconds.

```sh
machbase-neo shell export --output ./example_out.csv --format csv --timeformat ns EXAMPLE
```

## Copy a Table by Combining Export & Import

We can "copy" a table by combining export and import without a temporary file in local storage.

Make a new table where to copy data into.

```sh
machbase-neo shell sql "create tag table EXAMPLE_COPY (name varchar(100) primary key, time datetime basetime, value double)"
```

Then execute import and export command together.

```sh
machbase-neo shell export       \
    --output -                  \
    --no-heading --no-footer    \
    --format csv                \
    --timeformat ns             \
    EXAMPLE  |  \
machbase-neo shell import       \
    --input -                   \
    --format csv                \
    --timeformat ns             \
    EXAMPLE_COPY
```

Query the records count of newly create table.

```sh
machbase-neo shell sql "select count(*) from EXAMPLE_COPY"
```
```
 ROWNUM  COUNT(*) 
──────────────────
 1       1000     
```

This example is applicable in a situation that we want to "copy" a table from *A* database to *B* database.
We could set `--server <address>` flag specifies remote machbase-neo server process one of "import" and "export" commands,
And it is also possible set both of commands runs for two different remote servers.

## Import from Query Result

Let's combine "select" query and import command.

```sh
machbase-neo shell sql \
    --output -         \
    --format csv       \
    --no-rownum        \
    --no-heading       \
    --no-footer        \
    --timeformat ns    \
    "select * from example where name = 'wave.sin' order by time" | \
machbase-neo shell import \
    --input -             \
    --format csv          \
    EXAMPLE_COPY
```

We selected data that tag name is `wave.sin`, then import it into the `EXAMPLE_COPY` table.
It is required `--no-rownum` and `--no-heading` options in `sql` command because `import` command need to verify the number of fields and data type of the incoming csv data.

## Import from Query Result with HTTP API

The scenario importing from query results can be done with machbase-neo's HTTP API.

```sh
curl -o - http://127.0.0.1:5654/db/query        \
    --data-urlencode "q=select * from EXAMPLE order by time desc limit 100" \
    --data-urlencode "format=csv"                \
    --data-urlencode "heading=false" |           \
curl http://127.0.0.1:5654/db/write/EXAMPLE_COPY \
    -H "Content-Type: text/csv"                  \
    -X POST --data-binary @- 
```

## Import Methods: Insert vs Append

The import command writes the incoming data with "INSERT INTO..." statement by default.
As long as the total number of records to write is small, there is not a big difference from "append" method.

When you are expecting a large amount of data (e.g. more than several hundreds thousands records),
Use `--method append` flag that specify machbase-neo to use "append" method 
instead of "INSERT INTO..." statement which is implicitly specified as `--method insert`. 

## Detailed Examples

Data files can be written into the table using the import function.

> For smooth practice, the following query should be run to prepare tables and data.

```sql
CREATE TAG TABLE IF NOT EXISTS EXAMPLE (
    NAME VARCHAR(20) PRIMARY KEY,
    TIME DATETIME BASETIME,
    VALUE DOUBLE SUMMARIZED
);
```

### Import CSV Examples

Make test data in `data.csv`.

```
name-0,1687405320000000000,123.456
name-1,1687405320000000000,234.567000
name-2,1687405320000000000,345.678000
```

Import data

```sh
machbase-neo shell import \
    --input ./data.csv    \
    --timeformat ns        \
    EXAMPLE
```

Select data

```sh
machbase-neo shell sql "SELECT * FROM EXAMPLE"

 ROWNUM  NAME    TIME(LOCAL)          VALUE   
──────────────────────────────────────────────
      1  name-0  2023-06-22 12:42:00  123.456 
      2  name-1  2023-06-22 12:42:00  234.567 
      3  name-2  2023-06-22 12:42:00  345.678 
3 rows fetched.
```

### Import via TQL

**Import Text**

Make test data in `import-data.csv`.

```
1,100,value,10
2,200,value,11
3,140,value,12
```

Copy the code below into TQL editor and save `import-tql-csv.tql`.

```js
STRING(payload() ?? `1,100,value,10
2,200,value,11
3,140,value,12`, separator('\n'))

SCRIPT({
    str =  $.values[0].trim().split(',');
    $.yield(
        "tag-" + str[0],
        (new Date().getTime()*1000000),
        parseInt(str[1])+parseInt(str[3])
    )
})
APPEND(table("example"))
```

Post the test data CSV to the tql.

```sh
curl -o - --data-binary @import-data.csv http://127.0.0.1:5654/db/tql/import-tql-csv.tql

append 3 rows (success 3, fail 0).
```

Select data

```sh
machbase-neo shell sql "select * from example"

 ROWNUM  NAME   TIME(LOCAL)          VALUE 
───────────────────────────────────────────
      1  tag-1  1970-01-01 09:00:00  10    
      2  tag-2  1970-01-01 09:00:00  11    
      3  tag-3  1970-01-01 09:00:00  12    
3 rows fetched.
```

**Import JSON**

Prepare test data saved in `import-data.json`.

```json
{
  "tag": "pump",
  "data": {
    "string": "Hello TQL?",
    "number": "123.456",
    "time": 1687405320,
    "boolean": true
  },
  "array": ["elements", 234.567, 345.678, false]
}
```

Copy the code below into TQL editor and save `import-tql-json.tql`.

```js
BYTES( payload() ?? {
    {
        "tag": "pump",
        "data": {
            "string": "Hello TQL?",
            "number": "123.456",
            "time": 1687405320,
            "boolean": true
        },
        "array": ["elements", 234.567, 345.678, false]
    }
})
SCRIPT({
    obj = JSON.parse($.values[0]);
    $.yield(obj.tag+"_0", obj.data.time*1000000000, obj.data.number)
    $.yield(obj.tag+"_1", obj.data.time*1000000000, obj.data.array[1])
    $.yield(obj.tag+"_2", obj.data.time*1000000000, obj.data.array[2])
    for (i = 0; i < obj.array.length; i++) {
    }
})
APPEND(table("example"))
```

Post the test data JSON to the tql.

```sh
curl -o - --data-binary @import-data.json http://127.0.0.1:5654/db/tql/import-tql-json.tql

append 2 rows (success 2, fail 0).
```

Select data

```sh
machbase-neo shell sql "select * from example"

 ROWNUM  NAME    TIME(LOCAL)          VALUE   
──────────────────────────────────────────────
      1  tag-1   1970-01-01 09:00:00  10      
      2  pump_2  2023-06-22 12:42:00  345.678 
      3  tag-2   1970-01-01 09:00:00  11      
      4  tag-3   1970-01-01 09:00:00  12      
      5  pump_1  2023-06-22 12:42:00  234.567 
5 rows fetched.
```

### Import from Bridge

**Prepare**

```sh
bridge add -t sqlite mem file::memory:?cache=shared;

bridge exec mem create table if not exists mem_example(name varchar(20), time datetime, value double);

bridge exec mem insert into mem_example values('tag0', '2021-08-12', 10);
bridge exec mem insert into mem_example values('tag0', '2021-08-13', 11);
```

**Import data from Bridge**

Copy the code below into TQL editor and run

```js
SQL(bridge('mem'), "select * from mem_example")
APPEND(table('example'))
```

Select data

```sh
machbase-neo shell sql "select * from example"

 ROWNUM  NAME  TIME(LOCAL)          VALUE 
──────────────────────────────────────────
      1  tag0  2021-08-12 09:00:00  10    
      2  tag0  2021-08-13 09:00:00  11    
2 rows fetched.
```

### Export CSV Examples

Export data

```sh
machbase-neo shell export      \
    --output ./data_out.csv    \
    --format csv               \
    --timeformat ns            \
    EXAMPLE
```

Select data

```sh
cat data_out.csv 

TAG0,1628694000000000000,100
TAG0,1628780400000000000,110
```

### Export JSON Examples

Export data

```sh
machbase-neo shell export      \
    --output ./data_out.json   \
    --format json              \
    --timeformat ns            \
    EXAMPLE
```

Select data

```sh
cat data_out.json

{
  "data": {
    "columns": [
      "NAME",
      "TIME",
      "VALUE"
    ],
    "types": [
      "string",
      "datetime",
      "double"
    ],
    "rows": [
      [
        "TAG0",
        1628694000000000000,
        100
      ],
      [
        "TAG0",
        1628780400000000000,
        110
      ]
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "1.847207ms"
}
```

### Export via TQL

**Export CSV**

```js
SQL(`select * from example`)
CSV()
```

**Export JSON**

```js
SQL(`select * from example`)
JSON()
```

**Export CSV with TQL script**

Copy the code below into TQL editor and save `export-tql-csv.tql`.

```js
SQL( 'select * from example limit 30' )
SCRIPT({
    if  ($.values[2] % 2 == 0) {
        r_value = "even"
    } else {
        r_value = "odd"
    }

    $.yield($.key + "-tql", $.values[2],  r_value)
})
CSV()
```

Open it with web browser at [http://127.0.0.1:5654/db/tql/export-tql-csv.tql](http://127.0.0.1:5654/db/tql/export-tql-csv.tql), or use *curl* command on the terminal.

```sh
TAG1-tql,11,odd
TAG0-tql,10,even
```

### Export into Bridge

**Prepare**

```sh
bridge add -t sqlite mem file::memory:?cache=shared;

bridge exec mem create table if not exists mem_example(name varchar(20), time datetime, value double);
```

**Export data from Bridge**

Copy the code below into TQL editor and run

```js
SQL("select * from example")
INSERT(bridge('mem'), table('mem_example'), 'name', 'time', 'value')
```

Select bridge table data

```sh
machbase-neo shell bridge query mem "select * from mem_example";

┌──────┬───────────────────────────────┬───────┐
│ NAME │ TIME                          │ VALUE │
├──────┼───────────────────────────────┼───────┤
│ TAG0 │ 2021-08-12 00:00:00 +0900 KST │    10 │
│ TAG1 │ 2021-08-13 00:00:00 +0900 KST │    11 │
└──────┴───────────────────────────────┴───────┘
```

---

## Quick Reference

| Operation | Command | Key Options |
|-----------|---------|-------------|
| Import CSV | `machbase-neo shell import` | `--input`, `--timeformat`, `--compress` |
| Export CSV | `machbase-neo shell export` | `--output`, `--format csv`, `--timeformat` |
| Import via HTTP | `curl` to `/db/write/` | `Content-Type: text/csv` |
| Export via HTTP | `curl` to `/db/query` | `format=csv`, `heading=false` |
| Copy Table | `export \| import` | `--output -`, `--input -` |
| Large Data Import | `--method append` | For hundreds of thousands of records |
| TQL Import | `APPEND(table())` | Custom data processing |
| Bridge Integration | `SQL(bridge())` | External database connection |

### Common Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--input` | Input file path or `-` for stdin | `--input data.csv` |
| `--output` | Output file path or `-` for stdout | `--output result.csv` |
| `--format` | Data format (csv, json) | `--format csv` |
| `--timeformat` | Time format (s, ms, us, ns) | `--timeformat ns` |
| `--compress` | Compression method | `--compress gzip` |
| `--method` | Import method (insert, append) | `--method append` |