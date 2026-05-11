# CHART

## Kind

statement sink

## Category

chart encoder

## Signatures

```text
CHART(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper|expression | chart options |

## Description

`CHART()` generates an Apache ECharts based chart from incoming records. The official manual recommends `CHART()` for chart rendering and points to chart examples for detailed usage.

## Examples

### Line chart

```js
FAKE(oscillator(freq(1.5, 1.0), range('now', '3s', '25ms')))
CHART(
    size('600px', '400px'),
    chartOption({
        xAxis: { type: 'time' },
        yAxis: {},
        series: [{ type: 'line', data: column(0).map(function(t, idx){ return [t, column(1)[idx]]; }) }]
    })
)
```

## Related

CHART_LINE, CHART_BAR, CHART_SCATTER, freq, range, oscillator
