# geomapext

geomapext is a Goldmark extension for rendering Leaflet map fenced code blocks.

## Fence Language

Use geomap as the fence language.

## Fence Payload

The payload must be JSON and the root must be:

- an object: a single layer object or a GeoJSON object
- an array: multiple layer objects and/or GeoJSON objects

Supported core layer types:

- marker, circleMarker, circle, polyline, polygon
- GeoJSON types: FeatureCollection, Feature, Point, MultiPoint, LineString, MultiLineString, Polygon, MultiPolygon, GeometryCollection

## Fence Options

Use Hugo-style inline options.

```markdown
```geomap {width=100%,height=420px,tile=default,fit=auto,center=[37.5,127.0],zoom=11}
[{"type":"marker","coordinates":[37.5,127.0]}]
```
```

Supported options:

- width: map width (default: 100%)
- height: map height (default: 400px)
- tile: default or URL template with {z}, {x}, {y}
- tileOption: optional tile layer options as JSON string
- fit: auto, bounds, center
- center: [lat, lon]
- zoom: number
- grayscale: 0 to 1
- loader: none, local, auto
- leafletSrc: override local Leaflet JS URL
- leafletCss: override local Leaflet CSS URL
- cdnSrc: fallback CDN Leaflet JS URL
- cdnCss: fallback CDN Leaflet CSS URL

## Runtime Behavior

- Reuse an existing map instance when the same block is re-rendered.
- Remove non-tile layers before re-draw.
- Keep tile layer and apply grayscale filter.
- Show an inline error message when JSON parsing or rendering fails.

## Non-Goals

- custom CRS/proj4-based tile support is intentionally out of scope.
