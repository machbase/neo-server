# oscillator

## Kind

helper

## Category

generator

## Signatures

```text
oscillator(freqs..., range)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`oscillator()` generates wave data from one or more `freq()` components and a `range()` definition.

## Examples

### Basic

```js
FAKE(oscillator(freq(3, 1.0), range('now-3s', '3s', '5ms')))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
