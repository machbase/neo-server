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
- theme (string): white, dark, essos, chalk, purple-passion, romantic, walden, westeros, wonderland, vintage, macarons, infographic, shine, roma
- theme (string): light is accepted as alias of white
- theme (string): http(s) URL is accepted for custom theme script loading
- renderer (string): canvas or svg
- loader (string): none, local, auto (default: auto)
- plugins (string or array): plugin list, for example gl,wordcloud
- echartsSrc (string): local or custom ECharts core script URL
- cdnSrc (string): fallback CDN script URL used when loader=auto

Built-in plugin aliases:

- liquidfill -> /web/echarts/echarts-liquidfill.min.js
- wordcloud -> /web/echarts/echarts-wordcloud.min.js
- gl -> /web/echarts/echarts-gl.min.js

Built-in theme script loading:

- white: no additional script
- others: /web/echarts/themes/<theme>.js

## Runtime Requirement

The rendered HTML uses local-first loading by default.

1. Reuse window.echarts when already available.
2. Load /web/echarts/echarts.min.js when not available.
3. If loader=auto and local loading fails, try cdnSrc.

If loading still fails, the chart container shows an error message instead of crashing.
