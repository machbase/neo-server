# MAPVALUE

## Kind

statement map

## Category

map monad

## Signatures

```text
MAPVALUE(index, expression, options...)
MAPVALUE(index, expression)
MAPVALUE(index, expression, name)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| index | yes | no | literal:number | 0, 1, 2 |
| expression | yes | no | expression | value, key, param, time, parseTime, tz, list, dict, in, ?? |
| options | no | yes | literal:string|helper | nullValue, lazy |

## Description

`MAPVALUE()` changes or appends a value field in the current record. The first argument selects the value index, and the second argument is evaluated as the new field value. Additional options can name the output field or adjust mapping behavior.

## Examples

### Convert a value

```js
FAKE(json({
	['temperature', 1708582790, 23.45],
	['temperature', 1708582791, 24.56]
}))
MAPVALUE(1, value(1) * 1000000000)
CSV()
```

### Append a named value

```js
FAKE(arrange(1, 3, 1))
MAPVALUE(1, value(0) * 10, 'x10')
CSV()
```

## Related

value, key, param, time, parseTime, tz, list, dict, nullValue, lazy
