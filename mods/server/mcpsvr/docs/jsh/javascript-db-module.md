# Machbase Neo JavaScript DB Module

## Client

The database client.

**Usage example**

```js
const db = require("@jsh/db");
const client = new db.Client();
try {    
    conn = client.connect();
    rows = conn.query("select * from example limit 10")
    cols = rows.columns()
    console.log("cols.names:", JSON.stringify(cols.columns));
    console.log("cols.types:", JSON.stringify(cols.types));
    
    count = 0;
    for (const rec of rows) {
        console.log(...rec);
        count++;
    }
    console.log("rows:", count, "selected" );
} catch(e) {
    console.log("Error:", e);
} finally {
    if (rows) rows.close();
    if (conn) conn.close();
}
```

**Creation**

| Constructor             | Description                          |
|:------------------------|:----------------------------------------------|
| new Client(*options*)     | Instantiates a database client object with an options |

If neither `bridge` nor `driver` is specified, the client defaults to connecting to the internal Machbase DBMS.

**Options**

| Option              | Type         | Default        | Description         |
|:--------------------|:-------------|:---------------|:--------------------|
| lowerCaseColumns    | Boolean      | `false`        | map the lower-cased column names to the result object |

- Options for Drivers

`driver` and `dataSource` options support `sqlite`, `mysql`, `mssql`, `postgresql` and `machbase` without pre-defined bridge.

| Option              | Type         | Default        | Description         |
|:--------------------|:-------------|:---------------|:--------------------|
| driver              | String       |                | driver name         |
| dataSource          | String       |                | database connection string |

- Options for Bridge

It is also possible to create Client with predefined bridge.

| Option              | Type         | Default        | Description         |
|:--------------------|:-------------|:---------------|:--------------------|
| bridge              | String       |                | bridge name         |

**Properties**

| Property           | Type       | Description           |
|:-------------------|:-----------|:----------------------|
| supportAppend      | Boolean    | `true` if the client supports "Append" mode. |

### connect()

connect to the database.

**Return value**

- `Object` [Conn](#Conn)

## Conn

### close()

disconnect to the database and release

**Syntax**

```js
close()
```

### query()

**Syntax**

```js
query(String *sqlText*, any ...*args*)
```

**Return value**

- `Object` [Rows](#rows)

### queryRow()

**Syntax**

```js
queryRow(String *sqlText*, any ...*args*)
```

**Return value**

- `Object` [Row](#Row)

### exec()

**Syntax**

```js
exec(sqlText, ...args)
```

**Parameters**

- `sqlText` `String` Sql text string
- `args` `any` A variable-length list of arguments.

**Return value**

- `Object` [Result](#result)

### appender()

Create new "appender".

**Syntax**

```js
appender(table_name, ...columns)
```

**Parameters**

- `table_name` `String` The table name of to append.
- `columns` `String` A variable-length list of column names. If `columns` is omitted, all columns of the table will be appended in order.

**Return value**

- `Object` [Appender](#appender)

## Rows

Rows encapsulates the result set obtained from executing a query.

It implements `Symbol.iterable`, enabling support for both patterns:

```js
for(rec := rows.next(); rec != null; rec = rows.next()) {
    console.log(...rec);
}

for (rec of rows) {
    console.log(...rec);
}
```

### close()

Release database statement

**Syntax**

```js
close()
```

**Parameters**

None.

**Return value**

None.

### next()

fetch a record, returns null if no more records

**Syntax**

```js
next()
```

**Parameters**

None.

**Return value**

- `any[]`

### columns()

**Syntax**

```js
columns()
```

**Parameters**

None.

**Return value**

- `Object` [Columns](#columns)

### columnNames()

**Syntax**

```js
columnNames()
```

**Parameters**

None.

**Return value**

- `String[]`

### columnTypes()

**Syntax**

```js
columnTypes()
```

**Parameters**

None.

**Return value**

- `String[]`

## Row
Row encapsulates the result of queryRow which retrieve a single record.

### columns()

**Syntax**

```js
columns()
```

**Parameters**

None.

**Return value**

- `Object` [Columns](#columns)

### columnNames()

names of the result

**Syntax**

```js
columnNames()
```

**Parameters**

None.

**Return value**

- `String[]`

### columnTypes()

types of the result

**Syntax**

```js
columnTypes()
```

**Parameters**

None.

**Return value**

- `String[]`

### values()

result columns

**Syntax**

```js
values()
```

**Parameters**

None.

**Return value**

- `any[]`

## Result

Result represents the outcome of the `exec()` method, providing details about the execution.

**Properties**

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| message            | String     | result message     |
| rowsAffected       | Number     |                    |

## Columns

**Properties**

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| columns            | String[]   | names of the result |
| types              | String[]   | types of the result |

## Appender

### append()

Invoke the `append` method with the specified values in the order of the columns.

**Syntax**

```js
append(...values)
```

**Parameters**

- `values` `any` The values to be appended to the table, provided in the order of the specified columns.

**Return value**

None.

### close()

Close the appender.

**Syntax**

```js
close()
```

**Parameters**

None.

**Return value**

None.

### result()

Returns the result of the append operation after the appender is closed.

**Syntax**

```js
result()
```

**Parameters**

None.

**Return value**

- `Object`

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| success            | Number     |                    |
| failed             | Number     |                    |