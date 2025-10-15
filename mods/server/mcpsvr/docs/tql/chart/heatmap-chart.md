# Machbase Neo Heatmap Chart

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

### Heatmap Data Format

Heatmap charts require data in the format: `[x, y, value]`

Example:
```js
[10, 5, 0.8]  // x=10, y=5, value=0.8
```

---

## 1. Basic Heatmap (20K Data)

Large-scale heatmap with continuous color mapping.

```js
FAKE( meshgrid( linspace(1, 200, 200), linspace(1, 100, 100)) )
MAPVALUE(2, simplex(4, value(0)/40, value(1)/20) + 0.8 )
MAPVALUE(2, list(value(0), value(1), value(2)))
CHART(
    chartOption({
        tooltip: {},
        xAxis: { type: "category", value: column(0) },
        yAxis: { type: "category", value: column(1) },
        visualMap: {
            min: 0,
            max: 1.6,
            calculable: true,
            realtime: false,
            inRange: {
                color: [
                    "#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
                    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"
                ]
            }
        },
        series: [
            { 
                name: "SimpleX Noise",
                type: "heatmap",
                data: column(2),
                emphasis: {
                    itemStyle: {
                        borderColor: "#333",
                        borderWidth: 1
                    }
                },
                progressive: 1000,
                animation: false
            }
        ]
    })
)
```

**Description**: Large heatmap with 20,000 data points (200x100 grid) using simplex noise for smooth patterns. Features continuous color gradient from blue (low) to red (high).

**Key Points**:

**Data Generation**:
- `meshgrid(linspace(...))` creates 2D grid coordinates
- `simplex(4, x/40, y/20)` generates smooth noise pattern
- Result: 200×100 = 20,000 data points
- Final format: `[x, y, value]` triplets

**Color Mapping**:
- `visualMap` maps values to color gradient
- `min: 0, max: 1.6` defines value range
- 11-color gradient: blue → cyan → yellow → red
- `calculable: true` enables interactive range adjustment
- `realtime: false` optimizes performance for large datasets

**Performance**:
- `progressive: 1000` renders in chunks (1000 points at a time)
- `animation: false` disables animations for faster rendering
- Progressive rendering prevents UI blocking with large data

**Emphasis**:
- Hover adds black border around cell
- `borderWidth: 1` creates clear cell boundaries

---

## 2. Discrete Mapping of Colors (20K Data)

Heatmap with discrete color categories instead of continuous gradient.

```js
FAKE( meshgrid( linspace(1, 200, 200), linspace(1, 100, 100)) )
MAPVALUE(2, simplex(4, value(0)/40, value(1)/20) + 0.8 )
MAPVALUE(2, list(value(0), value(1), value(2)))
CHART(
    chartOption({
        tooltip: {},
        grid: { right: "120px", left: "40px"},
        xAxis: { type: "category", value: column(0) },
        yAxis: { type: "category", value: column(1) },
        visualMap: {
            type: "piecewise",
            min: 0,
            max: 1.8,
            left: "right",
            top: "center",
            calculable: true,
            realtime: false,
            splitNumber: 8,
            inRange: {
                color: [
                    "#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
                    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"
                ]
            }
        },
        series: [
            { 
                name: "SimpleX Noise",
                type: "heatmap",
                data: column(2),
                emphasis: {
                    itemStyle: {
                        borderColor: "#333",
                        borderWidth: 1
                    }
                },
                progressive: 1000,
                animation: false
            }
        ]
    })
)
```

**Description**: Similar to basic heatmap but uses discrete color categories instead of continuous gradient. Divides value range into 8 distinct color bands.

**Key Differences from Basic Heatmap**:

**Discrete Color Mapping**:
- `type: "piecewise"` creates discrete categories
- `splitNumber: 8` divides range into 8 equal bands
- Each band gets distinct color from gradient
- Easier to identify value ranges visually

**Legend Positioning**:
- `left: "right"` places legend on right side
- `top: "center"` vertically centers legend
- `grid: { right: "120px" }` makes room for legend

**Use Cases**:
- When precise values matter less than ranges
- Classification tasks (low/medium/high)
- Clearer visual distinction between value bands
- Better for categorical interpretation

---

## 3. Calendar Heatmap (Year 2023)

Heatmap displayed on calendar layout showing daily values for a year.

```js
FAKE(linspace(1, 365, 365))
MAPVALUE(1, simplex(10, value(0))+0.9)
MAPVALUE(0, 1672444800+(value(0)*3600*24)) // 2023/01/01 00:00:00
MAPVALUE(0, time(value(0)*1000000000))
MAPVALUE(0, list(value(0), value(1)))
CHART(
    chartOption({
        title: {
            top: 30,
            left: "center",
            text: "Daily Measurements"
        },
        tooltip: {},
        visualMap: {
            min: 0,
            max: 2.0,
            type: "piecewise",
            orient: "horizontal",
            left: "center",
            top: 65
        },
        calendar: {
            top: 120,
            left: 30,
            right: 30,
            cellSize: ["auto", 13],
            range: "2023",
            itemStyle: {
                borderWidth: 0.5
            },
            yearLabel: {show:true}
        },
        series: {
            type: "heatmap",
            coordinateSystem: "calendar",
            data: column(0)
        }
    })
)
```

**Description**: Calendar-based heatmap showing daily values throughout 2023. Each day is a colored cell in traditional calendar layout (weeks as rows, days as columns).

**Key Points**:

**Data Preparation**:
- `linspace(1, 365, 365)` generates day numbers
- `1672444800` = Unix timestamp for 2023-01-01 00:00:00
- `+(value(0)*3600*24)` adds days in seconds
- `time(value(0)*1000000000)` converts to time object
- Final format: `[time, value]` pairs

**Calendar Configuration**:
- `coordinateSystem: "calendar"` uses calendar layout
- `range: "2023"` displays full year 2023
- `cellSize: ["auto", 13]` auto width, 13px height
- `itemStyle.borderWidth: 0.5` adds cell borders
- `yearLabel: {show:true}` displays year label

**Visual Mapping**:
- `type: "piecewise"` for discrete color bands
- `orient: "horizontal"` horizontal legend above calendar
- Positioned below title, above calendar grid

**Layout**:
- Title at top center
- Legend horizontally centered below title
- Calendar grid starts at `top: 120` with side margins

**Use Cases**:
- Activity tracking (GitHub contribution style)
- Daily metrics visualization
- Seasonal pattern identification
- Year-over-year comparisons
- Attendance or frequency data