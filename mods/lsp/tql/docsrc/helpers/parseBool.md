# parseBool

## Kind

helper

## Category

string conversion

## Signatures

```text
parseBool(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| str | yes | no | literal:string|expression | true, false |

## Description

`parseBool()` converts accepted boolean strings such as `1`, `t`, `TRUE`, `true`, `0`, `f`, `FALSE`, and `false` to boolean values.

## Examples

### Basic

```js
FAKE(csv(`world,True`))
MAPVALUE(1, parseBool(value(1)))
JSON()
```

## Related

parseFloat, value
