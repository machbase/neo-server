# range

## Kind

helper

## Category

generator

## Signatures

```text
range(fromTime, duration, period)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`range()` specifies the time range for generated data, from `fromTime` to `fromTime + duration`, sampled by `period`.

## Examples

### Basic

```js
FAKE(oscillator(freq(3, 1.0), range('now-3s', '3s', '5ms')))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
