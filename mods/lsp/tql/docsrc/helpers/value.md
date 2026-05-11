# value

## Kind

helper

## Category

context

## Signatures

```text
value()
value(index)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| index | no | no | literal:number | 0, 1, 2 |

## Description

`value()` returns the current record value array. With an index, it returns one element from that array.

## Examples

### Basic

```js
MAPVALUE(1, value(0) * 10)
CSV()
```

## Related

key, param, context
