# Machbase Neo Scatter Chart

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

### Scatter Chart Data Format

Scatter charts typically use:
- Single values: `[y1, y2, y3, ...]` with x from xAxis.data
- Coordinate pairs: `[[x1, y1], [x2, y2], ...]`

---

## 1. Basic Scatter Chart

Simple scatter plot showing sine wave pattern.

```js
FAKE( linspace(0, 360, 100) )
MAPVALUE( 2, sin((value(0)/180)*PI) )
CHART(
    chartOption({
        xAxis:{ data: column(0) },
        yAxis:{},
        series:[
            { type:"scatter", data: column(1) }
        ]
    })
)
```

**Description**: Basic scatter chart plotting 100 points following a sine wave pattern. Uses category x-axis with single y values.

**Key Points**:
- `linspace(0, 360, 100)` generates 100 evenly spaced x values
- `sin((value(0)/180)*PI)` calculates sine for each x
- X values provided via `xAxis.data`
- Y values provided as single array `column(1)`
- Default scatter symbol size and color

---

## 2. Anscombe's Quartet

Four scatter plots with identical statistics but different patterns.

```js
FAKE( json({
    [1701059601000000000,  4.26, 3.1 ,  5.39, 12.5],
    [1701059602000000000,  5.68, 4.74,  5.73, 6.89],
    [1701059603000000000,  7.24, 6.13,  6.08, 5.25],
    [1701059604000000000,  4.82, 7.26,  6.42, 7.91],
    [1701059605000000000,  6.95, 8.14,  6.77, 5.76],
    [1701059606000000000,  8.81, 8.77,  7.11, 8.84],
    [1701059607000000000,  8.04, 9.14,  7.46, 6.58],
    [1701059608000000000,  8.33, 9.26,  7.81, 8.47],
    [1701059609000000000, 10.84, 9.13,  8.15, 5.56],
    [1701059610000000000,  7.58, 8.74, 12.74, 7.71],
    [1701059611000000000,  9.96, 8.1 ,  8.84, 7.04]
}) )

MAPVALUE(0, time(value(0)))
MAPVALUE(1, list(value(0), value(1)))
MAPVALUE(2, list(value(0), value(2)))
MAPVALUE(3, list(value(0), value(3)))
MAPVALUE(4, list(value(0), value(4)))
CHART(
    chartOption({
        title: {
            text: "Anscombe's quartet",
            left: "center",
            top: 0
        },
        grid: [
            { left:  "7%", top: "7%", width: "38%", height: "38%" },
            { right: "7%", top: "7%", width: "38%", height: "38%" },
            { left:  "7%", bottom: "7%", width: "38%", height: "38%" },
            { right: "7%", bottom: "7%", width: "38%", height: "38%" }
        ],
        xAxis: [
            { gridIndex: 0, type:"time", min: 1701059598000, max: 1701059614000 },
            { gridIndex: 1, type:"time", min: 1701059598000, max: 1701059614000 },
            { gridIndex: 2, type:"time", min: 1701059598000, max: 1701059614000 },
            { gridIndex: 3, type:"time", min: 1701059598000, max: 1701059614000 }
        ],
        yAxis: [
            { gridIndex: 0, min: 0, max: 15 },
            { gridIndex: 1, min: 0, max: 15 },
            { gridIndex: 2, min: 0, max: 15 },
            { gridIndex: 3, min: 0, max: 15 }
        ],
        series: [
            {   name: "I",
                type: "scatter",
                data: column(1),
                xAxisIndex: 0,
                yAxisIndex: 0,
                markLine: {
                    animation:false,
                    data: [
                        [ {coord: [1701059598000, 3], symbol: "none"}, {coord: [1701059614000, 13], symbol: "none"} ]
                    ]
                }
            },
            {   name: "II",
                type: "scatter",
                data: column(2),
                xAxisIndex: 1,
                yAxisIndex: 1,
                markLine: {
                    animation:false,
                    data: [
                        [ {coord: [1701059598000, 3], symbol: "none"}, {coord: [1701059614000, 13], symbol: "none"} ]
                    ]
                }
            },
            {   name: "III",
                type: "scatter",
                data: column(3),
                xAxisIndex: 2,
                yAxisIndex: 2,
                markLine: {
                    animation:false,
                    data: [
                        [ {coord: [1701059598000, 3], symbol: "none"}, {coord: [1701059614000, 13], symbol: "none"} ]
                    ]
                }
            },
            {   name: "IV",
                type: "scatter",
                data: column(4),
                xAxisIndex: 3,
                yAxisIndex: 3,
                markLine: {
                    animation: false,
                    data: [
                        [ {coord: [1701059598000, 3], symbol: "none"}, {coord: [1701059614000, 13], symbol: "none"} ]
                    ]
                }
            }
        ]
    })
)
```

**Description**: Classic Anscombe's quartet visualization showing four datasets with identical statistical properties (mean, variance, correlation) but vastly different distributions. Demonstrates why visualization matters beyond statistics.

**Key Points**:

**Data Preparation**:
- Raw data: `[timestamp, y1, y2, y3, y4]`
- Convert timestamp to time object
- Create coordinate pairs: `[time, y_value]` for each series
- Four columns for four scatter plots

**Multi-Grid Layout**:
- `grid: [...]` creates 4 chart areas
- 2×2 layout: top-left, top-right, bottom-left, bottom-right
- Each grid: 38% width × 38% height
- Positioned with left/right/top/bottom properties

**Axis Configuration**:
- 4 x-axes, one per grid (`gridIndex: 0-3`)
- 4 y-axes, one per grid (`gridIndex: 0-3`)
- All use same time range and y-range
- `xAxisIndex` and `yAxisIndex` link series to correct axes

**MarkLine (Regression)**:
- Each series has identical regression line
- Coordinates: `(1701059598000, 3)` to `(1701059614000, 13)`
- `symbol: "none"` removes endpoint markers
- `animation: false` for static lines
- All four have same slope and intercept

**Statistical Insight**:
- All four datasets have:
  - Same mean x and y
  - Same variance
  - Same correlation coefficient
  - Same regression line
- But completely different patterns:
  - I: Linear relationship
  - II: Non-linear (curved) relationship
  - III: Linear with outlier
  - IV: Vertical line with outlier

**Lesson**: Statistics alone can be misleading - visualization reveals true patterns.

---

## 3. 1 Million Points

Large-scale scatter chart with 1 million data points and zoom functionality.

```js
FAKE( linspace(0, 10, 500000) )

MAPVALUE(1, random()*10)
MAPVALUE(2, sin(value(0)) - value(0)*(0.1*random()) + 1)
MAPVALUE(3, cos(value(0)) - value(0)*(0.1*random()) - 1)
POPVALUE(1)

CHART(
    chartOption({
        legend: { show: false },
        xAxis: { data: column(0) },
        yAxis: {},
        dataZoom: [
            { type: "inside" },
            { type: "slider" }
        ],
        animation: false,
        series: [
            {
                name: "A",
                type: "scatter",
                data: column(1),
                symbolSize: 3,
                itemStyle: {
                    color: "#9ECB7F",
                    opacity: 0.5
                },
                large: true
            },
            {
                name: "B",
                type: "scatter",
                data: column(2),
                symbolSize: 3,
                itemStyle: {
                    color: "#5872C0",
                    opacity: 0.5
                },
                large: true
            }
        ]
    })
)
```

**Description**: High-performance scatter chart with 1 million points (500k per series). Demonstrates efficient rendering with zoom capabilities.

**Key Points**:

**Data Generation**:
- `linspace(0, 10, 500000)` creates 500k x values
- Series A (green): `sin(x)` with downward drift and noise
- Series B (blue): `cos(x)` with downward drift and noise
- Total: 1,000,000 data points

**Performance Optimization**:
- `large: true` enables large dataset mode
  - Uses WebGL rendering
  - Simplified drawing algorithms
  - No individual hover/select
- `animation: false` disables animations
- Small `symbolSize: 3` reduces visual clutter
- Semi-transparent (`opacity: 0.5`) shows density

**Data Zoom**:
- `type: "inside"` mouse wheel/trackpad zoom
- `type: "slider"` visual slider control
- Enables exploration of dense regions
- Essential for large datasets

**Visual Design**:
- Green (#9ECB7F) for upper pattern
- Blue (#5872C0) for lower pattern
- Semi-transparency reveals overlapping regions
- Small symbols prevent over-plotting

**Performance Tips**:
```js
large: true              // WebGL acceleration
animation: false         // Disable transitions
symbolSize: 2-5         // Small symbols
opacity: 0.3-0.6        // See through overlaps
progressive: 1000       // Progressive rendering
```

**Use Cases**:
- Sensor data visualization
- Scientific datasets
- Time-series analysis
- Pattern detection in large data
- IoT device data

**Technical Limits**:
- With `large: true`: millions of points
- Without: ~10,000 points comfortably
- Browser memory dependent
- Performance degrades with complexity