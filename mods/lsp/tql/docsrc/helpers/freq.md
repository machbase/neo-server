# freq

## Kind

helper

## Category

generator

## Signatures

```text
freq(frequency, amplitude)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`freq()` defines a sine wave component for `oscillator()`. It produces `amplitude * SIN(2*PI*frequency*time + phase) + bias`, with optional bias and phase.

## Examples

### Basic

```js
FAKE(oscillator(freq(3, 1.0), range('now-3s', '3s', '5ms')))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
