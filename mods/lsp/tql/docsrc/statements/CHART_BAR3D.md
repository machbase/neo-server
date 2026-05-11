# CHART_BAR3D

## Kind

statement sink

## Category

deprecated chart encoder

## Signatures

```text
CHART_BAR3D(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | chart options |

## Description

Deprecated 3D bar chart sink; use `CHART()` with the GL plugin and an ECharts bar3D series instead.

## Examples

### Deprecated chart

```js
FAKE(oscillator(freq(1.5, 1.0), range('now', '3s', '25ms')))
CHART_BAR3D(size('600px', '400px'))
```

## Related

CHART, freq, range, oscillator
