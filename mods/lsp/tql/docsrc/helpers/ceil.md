# ceil

## Kind

helper

## Category

math

## Signatures

```text
ceil(x)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| x | yes | no | number | value, expression |

## Description

Math function returning the least integer value greater than or equal to x. The official manual notes that math functions do not guarantee bit-identical results across system architectures.

## Examples

### Basic

```js
MAPVALUE(0, ceil(value(0)))
CSV()
```

## Related

MAPVALUE, value
