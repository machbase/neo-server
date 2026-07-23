# chartext

chartext is a Goldmark extension for rendering ECharts fenced code blocks.

## Fence Language

Use chart as the primary fence language.

echarts is also accepted as an alias for compatibility.

## Fence Option Syntax

Use Hugo-style inline fence options.

~~~markdown
```echarts {width=600px,height=400px,theme=dark,renderer=canvas}
function digit_format(v) {
  return "DIGIT: " + v;
}
option = {
  xAxis: { type: "category", data: ["Mon", "Tue"] },
  yAxis: { type: "value" },
  series: [{ type: "line", data: [820, 932] }]
};
```
~~~

## Supported Options

- width (string): chart width, for example 600px, 100%, 50vw
- height (string): chart height, for example 400px
- theme (string): light or dark
- renderer (string): canvas or svg

## Runtime Requirement

The rendered HTML expects window.echarts to be available.

If ECharts is not loaded, the chart container shows an error message instead of crashing.
