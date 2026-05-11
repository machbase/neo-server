# exp2

## Kind

helper

## Category

math

## Signatures

```text
exp2(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning 2**x, the base-2 exponential of x. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, exp2(value(0)))
CSV()
```

## Related

MAPVALUE, value
