# simplex

## Kind

helper

## Category

math

## Signatures

```text
simplex(seed, dim1)
simplex(seed, dim1, dim2)
simplex(seed, dim1, dim2, dim3, dim4)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| seed | yes | no | number | 123 |
| dimensions | yes | yes | number | value |

## Description

`simplex()` returns Simplex noise for the given seed and dimension values.

## Examples

### Basic

```js
MAPVALUE(2, abs(simplex(123, value(0), value(1))) * 10)
CSV()
```

## Related

abs, meshgrid, linspace
