# tan

## Kind

helper

## Category

math

## Signatures

```text
tan(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning the tangent of the radian argument x. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, tan(value(0)))
CSV()
```

## Related

MAPVALUE, value
