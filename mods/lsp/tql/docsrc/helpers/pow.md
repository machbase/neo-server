# pow

## Kind

helper

## Category

math

## Signatures

```text
pow(x, y)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |
| y | yes | no | number | value, expression |

## Description

Math function returning x**y, the base-x exponential of y. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, pow(value(0)))
CSV()
```

## Related

MAPVALUE, value
