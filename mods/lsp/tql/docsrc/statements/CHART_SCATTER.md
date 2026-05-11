# CHART_SCATTER

## Kind

statement sink

## Category

deprecated chart encoder

## Signatures

```text
CHART_SCATTER(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | chart options |

## Description

Deprecated scatter chart sink; use `CHART()` with an ECharts scatter series instead.

## Examples

### Deprecated chart

```js
FAKE(oscillator(freq(1.5, 1.0), range('now', '3s', '25ms')))
CHART_SCATTER(size('600px', '400px'))
```

## Related

CHART, freq, range, oscillator
