# Machbase Neo 3D Line Chart

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

### 3D Line Data Format

3D line charts require data in the format: `[x, y, z]`

Example:
```js
[1.5, 2.3, 4.7]  // x=1.5, y=2.3, z=4.7
```

---

## Orthographic Projection 3D Line

3D parametric line with orthographic projection and color gradient.

```js
FAKE(linspace(0, 24.999, 25000))
MAPVALUE(1, (1 + 0.25 * cos(75 * value(0))) * cos(value(0)))
MAPVALUE(2, (1 + 0.25 * cos(75 * value(0))) * sin(value(0)))
MAPVALUE(3, value(0) + 2.0 * sin(75 * value(0)))
MAPVALUE(0, list(value(1), value(2), value(3)))
POPVALUE(1,2,3)
CHART(
    plugins("gl"),
    chartOption({
        tooltip: {},
        backgroundColor: "#fff",
        visualMap: {
            show: false,
            dimension: 2,
            min: 0,
            max: 30,
            inRange: {
                color: [
                    "#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
                    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"
                ]
            }
        },
        xAxis3D: { type: "value" },
        yAxis3D: { type: "value" },
        zAxis3D: { type: "value" },
        grid3D: {
            viewControl: {
                projection: "orthographic"
            }
        },
        series: [
            {
                type: "line3D",
                data: column(0),
                lineStyle: { width: 4}
            }
        ]
    })
)
```

**Description**: 3D parametric spiral with 25,000 points rendered using orthographic projection. The line color varies along the z-axis from blue (low) to red (high), creating a rainbow effect.

**Key Points**:

**Parametric Equations**:
Creates a 3D spiral with modulated radius:
- `t` ranges from 0 to ~25 (25,000 points)
- `x = (1 + 0.25*cos(75t)) * cos(t)` - radial modulation in x
- `y = (1 + 0.25*cos(75t)) * sin(t)` - radial modulation in y  
- `z = t + 2.0*sin(75t)` - vertical position with oscillation

**Mathematical Properties**:
- Base radius: 1 unit
- Radius modulation: ±0.25 (oscillates 75 times)
- Vertical progression: linear with sinusoidal waves
- Total vertical range: ~25 units
- Creates tight helical coil with ripples

**Data Pipeline**:
- `linspace(0, 24.999, 25000)` generates parameter t
- Calculate x, y, z coordinates separately
- `list(x, y, z)` combines into 3D points
- `POPVALUE(1,2,3)` removes intermediate columns
- Result: single column of `[x, y, z]` arrays

**Projection**:
- `projection: "orthographic"` uses parallel projection
- Unlike perspective: parallel lines remain parallel
- No distance-based size changes
- Better for scientific visualization
- Preserves relative sizes at all depths

**Color Mapping**:
- `visualMap.dimension: 2` maps color to z-coordinate
- `show: false` hides legend
- 11-color gradient: blue → cyan → yellow → red
- Color indicates height along spiral
- Min/max (0-30) covers full z-range

**Performance**:
- 25,000 points efficiently rendered
- `lineStyle.width: 4` creates visible line
- ECharts GL handles large datasets smoothly

**Use Cases**:
- Parametric curve visualization
- Mathematical functions (spirals, knots, attractors)
- Trajectory data (particle paths, orbits)
- Time-series with 3 dimensions
- Scientific data visualization where perspective distortion should be avoided

**Orthographic vs Perspective**:
- **Orthographic**: Parallel projection, no depth distortion, technical/CAD style
- **Perspective**: Realistic depth, objects shrink with distance, natural viewing