# SQL

## Kind

statement source

## Category

database source

## Signatures

```text
SQL(sqlText, params...)
SQL(bridge, sqlText, params...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| bridge | no | no | helper:bridge | bridge |
| sqlText | yes | no | literal:string | backtick-sql-string |
| params | no | yes | expression | param, tz, sqlTimeformat, ansiTimeformat |

## Description

`SQL()` executes a SQL SELECT statement and produces records from the database. When `bridge()` is supplied, the query is executed through that bridge. Use backtick strings for multi-line SQL text and variadic arguments for bind parameters.

## Examples

### Query with parameters

```js
SQL(`SELECT time, value FROM example WHERE name = ? LIMIT ?`,
    param('name') ?? 'temperature',
    param('limit') ?? 10)
CSV()
```

### Query bridge database

```js
SQL(bridge('sqlite'), `SELECT * FROM EXAMPLE`)
CSV()
```

## Related

bridge, param, tz, sqlTimeformat, ansiTimeformat
