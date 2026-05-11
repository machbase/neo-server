# abs

## Kind

helper

## Category

math

## Signatures

```text
abs(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning the absolute value of x. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, abs(value(0)))
CSV()
```

## Related

MAPVALUE, value
