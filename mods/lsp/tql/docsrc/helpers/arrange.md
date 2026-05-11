# arrange

## Kind

helper

## Category

generator

## Signatures

```text
arrange(start, stop, step)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`arrange()` generates a numeric sequence from start to stop by step.

## Examples

### Basic

```js
FAKE(arrange(1, 2, 0.5))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
