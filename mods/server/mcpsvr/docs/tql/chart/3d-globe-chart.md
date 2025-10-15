# Machbase Neo 3D Globe Chart

## Quick Reference

### TQL Pipeline Structure

TQL operates in a **data flow (pipeline)** manner:

```
SRC (Data Source) → MAP (Transform) → SINK (Output)
```

---

### SRC - Data Sources

Functions that **generate or fetch data** (pipeline start)

| Function | Purpose | Example |
|----------|---------|---------|
| `FAKE()` | Generate test data | `FAKE(linspace(0, 100, 10))` |
| `SQL()` | Database query | `SQL('SELECT time, value FROM example')` |
| `CSV()` | Read CSV file | `CSV(file('/path/to/data.csv'))` |
| `SCRIPT()` | JavaScript code | `SCRIPT({ $.yield(1, 2, 3) })` |

---

### MAP - Data Transformation

Functions that **process and transform data** (pipeline middle)

| Function | Purpose | Example |
|----------|---------|---------|
| `MAPVALUE()` | Add/modify column | `MAPVALUE(1, value(0) * 2)` |
| `MAPKEY()` | Modify key | `MAPKEY(strUpper(key()))` |
| `PUSHVALUE()` | Insert column at front | `PUSHVALUE(0, "new_value")` |
| `POPVALUE()` | Remove column | `POPVALUE(2)` |
| `GROUP()` | Group/aggregate | `GROUP(by(value(0)), avg(value(1)))` |
| `DROP()` | Drop first N records | `DROP(1)` |

---

### SINK - Data Output

Functions that **output or save data** (pipeline end)

| Function | Purpose | Example |
|----------|---------|---------|
| `CHART()` | Create chart | `CHART(chartOption({...}))` |
| `CSV()` | CSV output | `CSV()` |
| `JSON()` | JSON output | `JSON()` |
| `INSERT()` | DB insert | `INSERT(...)` |
| `APPEND()` | DB append | `APPEND(table('example'))` |

---

### CHART() Function Basic Usage

**Syntax**: `CHART(chartOption() [,size()] [, theme()] [, chartJSCode()])`

*Available since version 8.0.8*

#### Main Options

**chartOption()**
- `chartOption( { json in apache echarts options } )`
- Pass Apache ECharts options in JSON format.

**size()**
- `size(width, height)`
- `width` *string* Chart width in HTML syntax e.g., `'800px'`
- `height` *string* Chart height in HTML syntax e.g., `'800px'`

**theme()**
- `theme(name)`
- `name` *string* Theme name
- Available themes: `white`, `dark`, `chalk`, `essos`, `infographic`, `macarons`, `purple-passion`, `roma`, `romantic`, `shine`, `vintage`, `walden`, `westeros`, `wonderland`

**chartJSCode()**
- `chartJSCode( { user javascript code } )`
- Execute custom JavaScript code.

**plugins()**
- `plugins(plugin...)`
- `plugin` *string* Pre-defined plugin name or URL of plugin module
- For 3D charts, use: `plugins("gl")`

---

### Key Functions

#### value(index)
Access values of the **current record** (used in pipeline middle)

- `value(0)` = First value of current record
- `value(1)` = Second value of current record
- `value()` = Entire value array

---

#### column(index)
Collect specific column from **all records** as array (CHART() only)

- `column(0)` = First values from all records → array
- `column(1)` = Second values from all records → array
- **⚠️ Only usable inside CHART()**

**Comparison**:

| Function | Location | Returns | Example |
|----------|----------|---------|---------|
| `value(0)` | Pipeline middle | Single value | `10` |
| `column(0)` | Inside CHART() | Array | `[1,2,3]` |

---

#### latlon(longitude, latitude)
Create geographic coordinate for globe visualization

- `latlon(lon, lat)` returns coordinate object for globe charts
- Used for mapping geographic data to 3D globe surface

---

## 1. Hello World Globe

Basic 3D globe with realistic textures and lighting.

```js
FAKE( json({
    ["world.topo.bathy.200401.jpg", "starfield.jpg", "pisa.hdr"]
}) )

MAPVALUE(0, "https://docs.machbase.com/assets/example/"+value(0))
MAPVALUE(1, "https://docs.machbase.com/assets/example/"+value(1))
MAPVALUE(2, "https://docs.machbase.com/assets/example/"+value(2))

CHART(
    plugins("gl"),
    chartOption({
        backgroundColor: "#000",
        globe: {
            baseTexture: column(0)[0],
            heightTexture: column(0)[0],
            displacementScale: 0.04,
            shading: "realistic",
            environment: column(1)[0],
            realisticMaterial: {
                roughness: 0.9
            },
            postEffect: {
                enable: true
            },
            light: {
                main: {
                    intensity: 5,
                    shadow: true
                },
                ambientCubemap: {
                    texture: column(2)[0],
                    diffuseIntensity: 0.2
                }
            }
        }
    })
)
```

**Description**: Photorealistic 3D globe with Earth bathymetry texture, starfield environment, and HDR lighting. Features displacement mapping for terrain elevation and realistic material rendering with shadows.

**Key Points**:
- `plugins("gl")` enables ECharts GL for 3D rendering
- `baseTexture` provides Earth surface imagery
- `heightTexture` creates 3D terrain using displacement mapping
- `displacementScale: 0.04` controls terrain height exaggeration
- `shading: "realistic"` uses physically-based rendering
- `environment` adds starfield background
- `ambientCubemap` provides HDR ambient lighting
- `postEffect` enables screen-space effects
- `light.main` configures directional light with shadows

**Textures Used**:
- Base/Height: `world.topo.bathy.200401.jpg` (Earth bathymetry)
- Environment: `starfield.jpg` (space background)
- Lighting: `pisa.hdr` (HDR ambient light)

---

## 2. Airline Routes on Globe

3D globe displaying airline flight routes between cities.

```js
CSV(file("https://docs.machbase.com/assets/example/flights.csv"))
DROP(1) // skip header
// |   0         1     2    3      4     5    6
// +-> flights   name1 lon1 lat1   name2 lon2 lat2
// |
MAPVALUE(0, latlon( parseFloat(value(2)), parseFloat(value(3))))
MAPVALUE(1, latlon( parseFloat(value(5)), parseFloat(value(6))))
// |   0     1      2    3      4     5    6
// +-> loc1  loc2   lon1 lat1   name2 lon2 lat2
// |
MAPVALUE(0, list(value(0), value(1)))
// |   0            1         2    3      4     5    6
// +-> [loc1,loc2]  (lat,lon) lon1 lat1   name2 lon2 lat2
// |
POPVALUE(1, 2, 3, 4, 5, 6)
// |   0
// +-> [loc1,loc2]
// |
CHART(
    plugins("gl"),
    chartOption({
        backgroundColor: "#000",
        globe: {
            baseTexture: "https://docs.machbase.com/assets/example/world.topo.bathy.200401.jpg",
            heightTexture: "https://docs.machbase.com/assets/example/bathymetry_bw_composite_4k.jpg",
            shading: "lambert",
            light: {
                ambient: {
                    intensity: 0.4
                },
                main: {
                    intensity: 0.4
                }
            },
            viewControl: {
                autoRotate: false
            }
        },
        series: {
            type: "lines3D",
            coordinateSystem: "globe",
            blendMode: "lighter",
            lineStyle: {
                width: 0.5,
                color: "rgb(50, 50, 150)",
                opacity: 0.1
            },
            data: column(0)
        }
    })
)
```

**Description**: Visualizes airline flight routes on a 3D globe. Loads CSV with origin/destination coordinates and draws curved lines connecting cities on the globe surface.

**Key Points**:

**Data Pipeline**:
- `CSV(file(...))` loads flight route data with coordinates
- `DROP(1)` skips CSV header row
- `latlon(lon, lat)` converts numeric coordinates to globe coordinate objects
- `list(loc1, loc2)` creates line segment endpoints
- Final data format: array of `[[lon1, lat1], [lon2, lat2]]` pairs

**Globe Configuration**:
- `shading: "lambert"` uses simple diffuse lighting (faster than realistic)
- `viewControl.autoRotate: false` disables automatic rotation
- Balanced ambient and main light intensities (0.4 each)

**Line Rendering**:
- `type: "lines3D"` renders curved lines on globe surface
- `coordinateSystem: "globe"` maps lines to 3D sphere
- `blendMode: "lighter"` creates additive blending for overlapping lines
- Semi-transparent blue lines (`opacity: 0.1`) show route density
- Thin lines (`width: 0.5`) reduce visual clutter

**Use Case**: Ideal for visualizing:
- Flight routes and airline networks
- Migration patterns
- Trade routes
- Communication networks
- Any point-to-point geographic connections