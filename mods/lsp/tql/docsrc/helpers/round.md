# round

## Kind

helper

## Category

math

## Signatures

```text
round(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning the nearest integer, rounding half away from zero. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, round(value(0)))
CSV()
```

## Related

MAPVALUE, value
