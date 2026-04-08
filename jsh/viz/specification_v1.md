# VIZ Specification V1

## 1. Purpose

VIZ (Abstract data visualization notation) is an intermediate representation for analysis results produced in jsh and Machbase Neo workflows.

The purpose of VIZ is to separate:

- analytical meaning
- density control for large datasets
- renderer-specific output generation

VIZ is designed primarily for large-scale time-series analysis and is intended to be adaptable to:

- ECharts JSON
- TUI block sequences
- standalone HTML
- SVG
- raster image export

## 2. Design Goals

VIZ V1 has the following goals:

1. Renderer independence
2. Large time-series suitability
3. Level-of-detail friendliness
4. Explicit provenance and data origin
5. Statistical expressiveness
6. First-class annotations
7. Stable adapter boundaries

## 3. Top-Level Structure

```json
{
  "version": 1,
  "domain": {},
  "axes": {
    "x": {},
    "y": []
  },
  "series": [],
  "annotations": [],
  "view": {},
  "meta": {}
}
```

Top-level objects:

- `domain`
- `axes`
- `series`
- `annotations`
- `view`
- `meta`

## 4. Domain

`domain` describes the logical range of the result set.

Recommended fields:

- `kind`
- `from`
- `to`
- `timeformat`
- `tz`
- `categories`

Supported domain kinds:

- `time`
- `numeric`
- `category`

### 4.1 Time Encoding

For time domains, VIZ V1 supports explicit time encoding metadata.

- `timeformat`: `rfc3339`, `s`, `ms`, `us`, `ns`

Rules:

- `timeformat=rfc3339` means time values are RFC3339 strings
- `timeformat=s|ms|us|ns` means time values are epoch values in that unit
- `from`, `to`, and time-related payload fields are interpreted according to these settings
- when omitted, adapters may use best-effort inference
- `domain.timeformat` and `domain.tz` describe how time values are encoded in the VIZ payload
- renderer output time formatting is an independent adapter concern and may use different options
- adapters should preserve the instant in time while allowing output representation to change

For Machbase Neo, the recommended default is:

```json
{
  "domain": {
    "kind": "time",
    "timeformat": "ns"
  }
}
```

## 5. Axes

Axis types:

- `time`
- `linear`
- `log`
- `category`

Axis fields:

- `id`
- `type`
- `unit`
- `label`
- `tz`
- `extent.min`
- `extent.max`

## 6. Series

Each series is a semantic data layer, not just a visual line.

Common fields:

- `id`
- `name`
- `axis`
- `representation`
- `source`
- `data`
- `quality`
- `style`
- `extra`

Each series must have `representation.kind`.

Supported `source.kind` values:

- `raw`
- `rollup`
- `sampled`
- `derived`

## 7. Style Hints

Common style keys in V1:

- `preferredRenderer`
- `color`
- `opacity`
- `lineColor`
- `lineWidth`
- `bandColor`

Style is not a renderer-specific option tree. Adapters map these hints to concrete renderer options.

## 8. Representation Kinds

### 8.1 raw-point

Purpose:

- represent raw points
- suitable for narrow time ranges and drill-down

Typical payload:

- `[time, value]`
- `[x, y]`

### 8.2 time-bucket-value

Purpose:

- represent one aggregated value per time bucket

Payload:

- `[time, value]`

Required fields:

- `time`
- `value`

Time field rule:

- `time` may be an RFC3339 string or an epoch value
- epoch values follow `domain.timeformat`

### 8.3 time-bucket-band

Purpose:

- represent range-oriented bucket summaries
- recommended default overview representation for large time-series data

Typical payload:

- `[time, min, max, avg, count]`
- `[time, p05, p50, p95]`

Required rules:

- `fields` must include `time`
- `fields` must include at least one of `min`, `max`, `avg`

Time field rule:

- `time` may be RFC3339 or epoch
- epoch values follow `domain.timeformat`

Recommended ECharts mapping:

- lower baseline + stacked range area + average line

### 8.4 distribution-histogram

Payload:

- `[binStart, binEnd, count]`

Required fields:

- `binStart`
- `binEnd`
- `count`

Recommended ECharts mapping:

- category x-axis + `bar` series

### 8.5 distribution-boxplot

Main payload:

- `[category, low, q1, median, q3, high]`

Outliers:

- `[category, value]`

Required fields:

- `category`, `low`, `q1`, `median`, `q3`, `high`

Optional outlier fields:

- `category`, `value`

Recommended ECharts mapping:

- category x-axis + `boxplot`
- optional `scatter` series for `extra.outliers`

### 8.6 event-point

Payload:

- `[time, value, label, severity]`

Required fields:

- `time`
- `value`

Recommended ECharts mapping:

- `scatter`

If time input is epoch, the adapter may normalize it to RFC3339 or another renderer-safe time value.

### 8.7 event-range

Payload:

- `[from, to, label]`

Required fields:

- `from`
- `to`

Recommended ECharts mapping:

- helper `line` series + `markArea`

If `from` and `to` are epoch values, the adapter may normalize them to RFC3339 or another renderer-safe representation.

## 9. Annotations

Annotations are top-level first-class objects.

Supported kinds:

- `point`
- `line`
- `range`

When an annotation targets the x-axis, its time values follow the time-domain rules and may use RFC3339 or epoch `s/ms/us/ns`.

## 10. Quality and Provenance

Recommended quality fields:

- `sampled`
- `coverage`
- `rowCount`
- `estimatedPoints`
- `downsamplePolicy`

Recommended `downsamplePolicy` values:

- `none`
- `rollup`
- `stride`
- `lttb`
- `quantile-band`

## 11. Level of Detail

Recommended LOD pattern:

- overview: `time-bucket-band`
- intermediate: `time-bucket-value`
- detail: `raw-point`

`meta.lodGroup` may be used to connect multiple series that belong to the same logical signal.

## 12. Renderer Boundary Principles

VIZ must not embed renderer-specific option trees directly.

Adapters are responsible for converting VIZ into:

- ECharts JSON
- TUI block sequences
- standalone HTML
- SVG
- raster images

Time normalization is also an adapter responsibility.

- the model preserves time-domain meaning and encoding metadata
- adapters may normalize epoch inputs into renderer-safe time values
- output adapters should accept independent `timeformat` and `tz` options for output-side time representation
- when omitted, output adapters default to `rfc3339` and local timezone
- the current Go ECharts adapter may emit RFC3339 or epoch-unit values depending on output options
- the current Go TUI adapter may render summary, timeline, event, annotation, and table time values using adapter options

## 13. TUI Adapter V1

The TUI adapter returns block sequences rather than terminal escape sequences.

Recommended options:

- `width`
- `rows`
- `compact`
- `timeformat`
- `tz`

Recommended block types:

- `summary`
- `series-summary`
- `sparkline`
- `bandline`
- `bars`
- `box-summary`
- `event-list`
- `timeline`
- `table`
- `annotations`

Current default TUI strategy:

- one top-level `summary` block per spec
- one `series-summary` block per series
- one visualization block per series
- one truncated `table` block per series
- one final `annotations` block when annotations exist

Time-domain behavior:

- `summary`, `event-list`, `timeline`, `annotations`, and table rows may render time values using adapter `timeformat` and `tz`
- `event-range` timeline strip calculation uses `domain.from`, `domain.to`, and `timeformat`

## 14. SVG Adapter V1

The SVG adapter produces a standalone SVG document string.

The intended role of the SVG adapter is:

- deterministic static rendering
- export-friendly output
- a low-dependency path to image generation
- a shared basis for future raster image conversion

### 14.1 Recommended API Shape

Go model API:

- `ToSVG(spec *Spec, options *SVGOptions) ([]byte, error)`

JSH-facing API:

- `toSVG(spec, options)`

The adapter should return a complete SVG document rather than a partial fragment.

### 14.2 SVGOptions Type Definition

V1 should use a fixed typed options object rather than an open-ended style map.

Recommended Go shape:

```go
type SVGOptions struct {
  Width      int    `json:"width,omitempty"`
  Height     int    `json:"height,omitempty"`
  Padding    int    `json:"padding,omitempty"`
  Background string `json:"background,omitempty"`
  FontFamily string `json:"fontFamily,omitempty"`
  FontSize   int    `json:"fontSize,omitempty"`
  ShowLegend *bool  `json:"showLegend,omitempty"`
  Title      string `json:"title,omitempty"`
  Timeformat string `json:"timeformat,omitempty"`
  TZ         string `json:"tz,omitempty"`
}
```

Field semantics:

- `width`: outer SVG width in CSS pixels
- `height`: outer SVG height in CSS pixels
- `padding`: outer chart padding applied before plot layout
- `background`: document background fill color
- `fontFamily`: default font family applied to text nodes
- `fontSize`: base font size in CSS pixels
- `showLegend`: enables or suppresses legend rendering when set explicitly
- `title`: optional document title rendered above the plot region
- `timeformat`: output-side time representation for axis labels and time text
- `tz`: output-side timezone used when `timeformat=rfc3339`

Validation and normalization rules:

- `width` must be greater than `0`
- `height` must be greater than `0`
- `padding` must be `0` or greater
- `fontSize` must be greater than `0`
- empty `background` falls back to default
- empty `fontFamily` falls back to default
- nil `showLegend` falls back to default

Recommended defaults:

- `width`: `960`
- `height`: `420`
- `padding`: `48`
- `background`: `white`
- `fontFamily`: `sans-serif`
- `fontSize`: `12`
- `showLegend`: `true`
- `title`: empty string

JSH bindings should expose the same logical shape.

### 14.3 Output Schema

The adapter output is a UTF-8 SVG document.

Required SVG characteristics:

- root `<svg>` element with explicit `width`, `height`, and `viewBox`
- a background layer
- a plot region clipped to the chart rectangle
- axis layers
- series layers
- annotation layers
- optional legend layer

Recommended layer structure:

```xml
<svg width="960" height="420" viewBox="0 0 960 420" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <clipPath id="plot-clip">...</clipPath>
  </defs>
  <g data-viz-role="background">...</g>
  <g data-viz-role="axes">...</g>
  <g data-viz-role="series">...</g>
  <g data-viz-role="annotations">...</g>
  <g data-viz-role="legend">...</g>
</svg>
```

Legacy implementations may still use the `data-advn-role` prefix for backward compatibility.

The exact element tree does not need to be fixed rigidly, but the output should remain deterministic for testing.

### 14.4 Recommended V1 Support Scope

The recommended first implementation scope is:

- `time-bucket-value`
- `time-bucket-band`
- `distribution-histogram`
- `distribution-boxplot`
- `event-point`
- `event-range`
- `point`, `line`, `range` annotations

`raw-point` may be included in V1 when point density is manageable, but it is not the primary export target for large overview datasets.

### 14.5 Representation Mapping

Recommended SVG mappings:

- `time-bucket-value` -> one polyline or path in plot coordinates
- `time-bucket-band` -> one filled band path plus one average line path
- `distribution-histogram` -> one rect per bin
- `distribution-boxplot` -> whisker lines, box rect, median line, optional outlier circles
- `event-point` -> marker circles or symbols at time-value coordinates
- `event-range` -> shaded vertical region rectangles spanning the plot height
- `line` annotation -> axis-aligned guide line
- `range` annotation -> shaded overlay region
- `point` annotation -> highlighted point marker and optional label

### 14.6 Time-Domain Behavior

For time axes, the SVG adapter should use the same time coercion rules as other adapters.

- RFC3339 and epoch `s/ms/us/ns` must be supported
- epoch inputs may be normalized internally before coordinate mapping
- axis labels may render as readable RFC3339 text or another stable human-readable time label format

For Machbase Neo default usage, `timeformat=ns` should be treated as a first-class path.

### 14.7 Testing Strategy

Recommended tests:

- deterministic golden tests for SVG output strings
- targeted assertions for `viewBox`, layer presence, and key path/rect elements
- epoch nanosecond time-domain tests
- representation-specific snapshot tests for band, histogram, boxplot, and event-range

### 14.8 Recommended Implementation Files

Recommended initial Go files:

- `svg.go`
- `svg_test.go`
- optional shared helpers in `time.go` or `layout.go`

### 14.9 PNG Export Path

PNG export should be treated as a downstream rasterization path built on top of SVG, not as an independent renderer-specific semantic adapter.

Recommended staged path:

- Stage 1: `ToSVG()` is the canonical static renderer in the model layer
- Stage 2: a PNG export path consumes the SVG output and rasterizes it
- Stage 3: command-level UX may expose `viz export --format png` once a stable rasterizer is chosen

Recommended boundary:

- VIZ model remains responsible for semantic-to-SVG conversion
- rasterization remains a separate concern from chart layout and semantic mapping
- PNG export must not introduce a second, divergent visual mapping path for the same VIZ spec

Recommended API direction:

- keep `ToSVG(spec, options)` as the primary stable API
- add a later `ToPNG(spec, svgOptions, rasterOptions)` only after the raster backend is fixed
- prefer a backend that consumes SVG bytes directly rather than rebuilding chart primitives again

Recommended command strategy:

- near term: `viz export --format svg`
- next step: `viz export --format png` implemented as `spec -> SVG -> PNG`

Recommended V1.1 raster options:

- `scale`
- `dpi`
- `background`

This keeps semantic rendering deterministic and avoids duplicating layout logic across SVG and PNG code paths.

## 15. Machbase Neo Epoch Nanosecond Example

```json
{
  "version": 1,
  "domain": {
    "kind": "time",
    "timeformat": "ns",
    "from": 1775174400000000000,
    "to": 1775217600000000000,
    "tz": "UTC"
  },
  "series": [
    {
      "id": "latency-band",
      "name": "latency-band",
      "representation": {
        "kind": "time-bucket-band",
        "bucketWidth": "1m",
        "fields": ["time", "min", "max", "avg", "count"]
      },
      "data": [
        [1775174400000000000, 18.1, 21.7, 19.8, 60],
        [1775174460000000000, 18.0, 21.5, 19.7, 60]
      ]
    },
    {
      "id": "maintenance-window",
      "name": "maintenance-window",
      "representation": {
        "kind": "event-range",
        "fields": ["from", "to", "label"]
      },
      "data": [
        [1775210400000000000, 1775214000000000000, "maintenance"]
      ]
    }
  ],
  "annotations": [
    {
      "kind": "line",
      "axis": "x",
      "value": 1775212200000000000,
      "label": "checkpoint"
    }
  ],
  "meta": {
    "producer": "machbase-neo"
  }
}
```

## 16. Precision Notes

- JSON numbers and JavaScript numbers may not safely preserve epoch nanoseconds
- for JSH or JavaScript builder paths, string values are recommended for epoch `ns`
- file parsing paths should preserve large numeric values without precision loss

## 17. Recommended Package Layout

Common Go model:

- `neo-server/jsh/viz`

JSH binding:

- `neo-server/jsh/lib/vizspec`

Recommended files:

- `types.go`
- `validate.go`
- `json.go`
- `svg.go`
- `vizspec.go`
- `vizspec.js`
- `vizspec_test.go`

## 18. Public JSH Exposure

Native module:

- `@jsh/vizspec`

User-facing require path:

- `require('vizspec')`
