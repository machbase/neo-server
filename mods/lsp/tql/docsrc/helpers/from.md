# from

## Kind

helper

## Category

database source

## Signatures

```text
from(table, tag)
from(table, tag, timeColumn, nameColumn)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| table | yes | no | literal:string | table name |
| tag | yes | no | literal:string | tag name |
| timeColumn | no | no | literal:string | 'time' |
| nameColumn | no | no | literal:string | 'name' |

## Description

`from()` supplies table and tag information to `SQL_SELECT()`. It represents the FROM table and tag-name condition used when generating SQL internally.

## Examples

### Basic

```js
SQL_SELECT('time', 'value', from('example', 'temperature'), between('last-10s', 'last'))
CSV()
```

## Related

SQL_SELECT, between, limit
