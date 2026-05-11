# parseFloat

## Kind

helper

## Category

string conversion

## Signatures

```text
parseFloat(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| str | yes | no | literal:string|expression | value |

## Description

`parseFloat()` parses a string into a floating point number.

## Examples

### Basic

```js
FAKE(csv(`world,3.141792`))
MAPVALUE(1, parseFloat(value(1)))
JSON()
```

## Related

parseBool, strSprintf, value
