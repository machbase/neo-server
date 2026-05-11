# meshgrid

## Kind

helper

## Category

generator

## Signatures

```text
meshgrid(xseries, yseries)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`meshgrid()` generates paired mesh values from x and y series.

## Examples

### Basic

```js
FAKE(meshgrid(linspace(1, 3, 3), linspace(10, 30, 3)))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
