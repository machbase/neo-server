# limit

## Kind

helper

## Category

database source

## Signatures

```text
limit(count)
limit(offset, count)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| offset | no | no | literal:number | 0 |
| count | yes | no | literal:number | 1000 |

## Description

`limit()` supplies the LIMIT clause to `SQL_SELECT()`. With one argument it is treated as count; with two arguments it is offset and count.

## Examples

### Basic

```js
SQL_SELECT('time', 'value', from('example', 'temperature'), between('last-10s', 'last'), limit(1000))
CSV()
```

## Related

SQL_SELECT, from, between
