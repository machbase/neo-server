# random

## Kind

helper

## Category

math

## Signatures

```text
random()
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| none | no | no | none | none |

## Description

`random()` returns a pseudo-random float in the half-open interval [0.0, 1.0).

## Examples

### Basic

```js
MAPVALUE(1, value(1) + (random() - 0.5) * 0.2)
CSV()
```

## Related

FAKE, oscillator
