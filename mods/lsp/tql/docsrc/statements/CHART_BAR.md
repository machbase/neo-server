# CHART_BAR

## Kind

statement sink

## Category

deprecated chart encoder

## Signatures

```text
CHART_BAR(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | chart options |

## Description

Deprecated bar chart sink; use `CHART()` with an ECharts bar series instead.

## Examples

### Deprecated chart

```js
FAKE(oscillator(freq(1.5, 1.0), range('now', '3s', '25ms')))
CHART_BAR(size('600px', '400px'))
```

## Related

CHART, freq, range, oscillator
