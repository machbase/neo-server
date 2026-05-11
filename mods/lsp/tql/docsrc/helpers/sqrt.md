# sqrt

## Kind

helper

## Category

math

## Signatures

```text
sqrt(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning the square root of x. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, sqrt(value(0)))
CSV()
```

## Related

MAPVALUE, value
