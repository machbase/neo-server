# Machbase Neo Radar Chart

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

### Radar Chart Data Format

Radar charts require:
- **Indicators**: Axes definitions with names and max values
- **Data**: Arrays of values matching indicator count

Example:
```js
radar: {
    indicator: [
        { name: "Axis1", max: 100 },
        { name: "Axis2", max: 200 }
    ]
}
data: [
    { name: "Series1", value: [80, 150] }
]
```

---

## 1. Basic Radar Chart

Standard radar chart comparing budget allocation vs actual spending.

```js
//                       sales, admin, it,  cs,   dev,   mkt
FAKE(json({
    ["Allocated Budget", 4200, 3000, 20000, 35000, 50000, 18000],
    ["Actual Spending"  , 5000, 14000, 28000, 26000, 42000, 21000]
}))

MAPVALUE(1, list(value(1), value(2), value(3), value(4), value(5), value(6)))
MAPVALUE(1, dict("name", value(0), "value", value(1)))
POPVALUE(2,3,4,5,6)
CHART(
    chartOption({
        title: { "text": "Basic Radar Chart" },
        legend: {
            data: column(0),
            top: "95%"
        },
        radar: {
            indicator: [
                { name: "Sales", max: 6500 },
                { name: "Administration", max: 16000 },
                { name: "Information Technology", max: 30000 },
                { name: "Customer Support", max: 38000 },
                { name: "Development", max: 52000 },
                { name: "Marketing", max: 25000 }
            ]
        },
        series: [
            {
                name: "Budget vs spending",
                type: "radar",
                data: column(1)
            }
        ]
    })
)
```

**Description**: Basic radar chart comparing allocated budget vs actual spending across 6 departments. Each axis represents a department, and two polygons show budget and spending patterns.

**Key Points**:

**Data Preparation**:
- Raw data: `["Category", val1, val2, val3, val4, val5, val6]`
- `list(value(1)...value(6))` collects values into array
- `dict("name", ..., "value", ...)` creates object format
- Result: `[{name: "Allocated Budget", value: [4200, 3000, ...]}, ...]`

**Radar Configuration**:
- `indicator`: Defines each axis
  - `name`: Axis label (department name)
  - `max`: Maximum value for that axis (sets scale)
- 6 indicators = 6-sided polygon
- Each axis has independent max value

**Data Mapping**:
- Value array length must match indicator count
- First value → first indicator (Sales)
- Second value → second indicator (Administration)
- Order matters!

**Visualization**:
- Two overlapping polygons
- Blue: Allocated Budget
- Red: Actual Spending
- Overlapping areas show alignment
- Gaps show over/under spending

**Use Cases**:
- Multi-dimensional comparisons
- Performance metrics
- Skills assessment
- Product feature comparison
- Quality attributes

---

## 2. Custom Radar Chart

Highly customized radar chart with circular shape and styled appearance.

```js
FAKE( json({
    [100,   8, 0.4,  -80, 2000],
    [ 60,   5, 0.3, -100, 1500]
}))

CHART(
    chartOption({
        color: ["#67F9D8", "#FFE434", "#56A3F1", "#FF917C"],
        title: {
            text: "Customized Radar Chart"
        },
        legend: {},
        radar: [
            {
                indicator: [
                    { text: "Indicator1" },
                    { text: "Indicator2" },
                    { text: "Indicator3" },
                    { text: "Indicator4" },
                    { text: "Indicator5" }
                ],
                center: ["50%", "50%"],
                radius: 200,
                startAngle: 90,
                splitNumber: 4,
                shape: "circle",
                axisName: {
                    formatter: "【{value}】",
                    color: "#428BD4"
                },
                splitArea: {
                    areaStyle: {
                        color: ["#77EADF", "#26C3BE", "#64AFE9", "#428BD4"],
                        shadowColor: "rgba(0, 0, 0, 0.2)",
                        shadowBlur: 10
                    }
                },
                axisLine: {
                    lineStyle: {
                        color: "rgba(211, 253, 250, 0.8)"
                    }
                },
                splitLine: {
                    lineStyle: {
                        color: "rgba(211, 253, 250, 0.8)"
                    }
                }
            },
        ],
        series: [
            {
                type: "radar",
                emphasis: {
                    lineStyle: {
                        width: 4
                    }
                },
                data: [
                    {
                        value: [100, 8, 0.4, -80, 2000],
                        name: "Data A"
                    },
                    {
                        value: [60, 5, 0.3, -100, 1500],
                        name: "Data B",
                        areaStyle: {
                            color: "rgba(255, 228, 52, 0.6)"
                        }
                    }
                ]
            }
        ]
    })
)
```

**Description**: Heavily customized radar chart with circular shape, gradient background, and styled axes. Demonstrates extensive styling options for radar charts.

**Key Customizations**:

**Shape and Layout**:
- `shape: "circle"` creates circular web (vs default polygon)
- `center: ["50%", "50%"]` positions at container center
- `radius: 200` sets size in pixels
- `startAngle: 90` rotates starting position (90° = top)
- `splitNumber: 4` creates 4 concentric circles

**Axis Styling**:
- `axisName.formatter: "【{value}】"` wraps labels in brackets
- `axisName.color: "#428BD4"` blue axis labels
- `axisLine` styles radial lines from center
- `splitLine` styles concentric circles
- Semi-transparent cyan colors

**Background (Split Areas)**:
- `splitArea.areaStyle.color` gradient array
- 4 colors for 4 concentric bands
- Outer to inner: teal → cyan → blue → dark blue
- `shadowBlur: 10` adds depth effect

**Data Series**:
- Direct value arrays (no name in data structure)
- Each series can have custom `areaStyle`
- "Data B" has yellow semi-transparent fill
- `emphasis.lineStyle.width: 4` thickens on hover

**Indicator Configuration**:
- Uses `text` instead of `name` + `max`
- Auto-scales based on data values
- No explicit max values defined

**Differences from Basic**:
- Circular vs polygonal shape
- Gradient background vs plain
- Custom colors throughout
- No explicit max values (auto-scaled)
- Inline data vs dataset approach

**Advanced Options**:
```js
radar: {
    shape: "polygon" | "circle",  // Shape type
    startAngle: 90,                // Rotation (degrees)
    splitNumber: 4,                // Concentric circles
    name: {                        // Axis label styling
        formatter: "【{value}】",
        textStyle: {...}
    },
    axisLine: {...},               // Radial lines
    splitLine: {...},              // Concentric circles
    splitArea: {...}               // Background bands
}
```

**Use Cases**:
- Branded dashboards (custom colors)
- Aesthetic presentations
- Multi-metric analysis with varying scales
- When visual impact matters
- Circular patterns preferred over polygons