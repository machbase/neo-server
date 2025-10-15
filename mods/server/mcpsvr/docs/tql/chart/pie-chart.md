# Machbase Neo Pie Chart

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
| `TRANSPOSE()` | Transpose rows to columns | `TRANSPOSE()` |

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

### Pie Chart Data Format

Pie charts accept two data formats:

**Array format**: `[["name", value], ["name2", value2]]`
**Object format**: `[{name: "name", value: value}, {name: "name2", value: value2}]`

---

## 1. Basic Pie Chart

Standard pie chart showing proportional distribution.

```js
FAKE( json({
    ["Search Engine", 1048 ],
    ["Direct"       ,  735 ],
    ["Email"        ,  580 ],
    ["Union Ads"    ,  484 ],
    ["Video Ads"    ,  300 ]
}) )
MAPVALUE(0, list(value(0), value(1)))
CHART(
    chartOption({
        tooltip: {
            trigger: "item"
        },
        legend: {
            orient: "vertical",
            left: "left"
        },
        dataset: [ { source: column(0) } ],
        series: [
            {
                name: "Access From",
                type: "pie",
                radius: "70%",
                datasetIndex: 0,
                emphasis: {
                    itemStyle: {
                        shadowBlur: 10,
                        shadowOffsetX: 0,
                        shadowColor: "rgba(0, 0, 0, 0.5)"
                    }
                }
            }
        ]
    })
)
```

**Description**: Basic pie chart displaying traffic source distribution. Each slice represents a category's proportion of the total.

**Key Points**:

**Data Pipeline**:
- `json({...})` creates category-value pairs
- `list(value(0), value(1))` wraps into arrays
- `dataset: [{source: column(0)}]` uses dataset for data binding
- Final format: `[["Search Engine", 1048], ["Direct", 735], ...]`

**Chart Configuration**:
- `type: "pie"` creates pie chart
- `radius: "70%"` sets chart size (70% of container)
- Single value = circular pie
- `datasetIndex: 0` references first dataset

**Legend**:
- `orient: "vertical"` stacks items vertically
- `left: "left"` positions on left side
- Shows category names with color indicators

**Tooltip**:
- `trigger: "item"` shows on slice hover
- Displays category name and value
- Automatically shows percentage

**Emphasis (Hover)**:
- `shadowBlur: 10` adds blur effect
- `shadowColor` creates depth
- Slice slightly enlarges on hover

---

## 2. Doughnut Chart

Pie chart with hollow center (doughnut/ring chart).

```js
FAKE( json({
    ["Search Engine", 1048 ],
    ["Direct"       ,  735 ],
    ["Email"        ,  580 ],
    ["Union Ads"    ,  484 ],
    ["Video Ads"    ,  300 ]
}) )
MAPVALUE(0, list(value(0), value(1)))
CHART(
    chartOption({
        tooltip: {
            trigger: "item"
        },
        legend: {
            orient: "vertical",
            left: "left"
        },
        dataset: [ { source: column(0) } ],
        series: [
            {
                name: "Access From",
                type: "pie",
                radius: ["40%", "70%"],
                datasetIndex: 0,
                emphasis: {
                    itemStyle: {
                        shadowBlur: 10,
                        shadowOffsetX: 0,
                        shadowColor: "rgba(0, 0, 0, 0.5)"
                    }
                }
            }
        ]
    })
)
```

**Description**: Doughnut chart with hollow center. Identical to pie chart except for the ring shape created by dual radius values.

**Key Difference**:
- **Pie**: `radius: "70%"` (single value = filled circle)
- **Doughnut**: `radius: ["40%", "70%"]` (array = ring shape)
  - First value (40%) = inner radius (creates hole)
  - Second value (70%) = outer radius (chart size)
  - Difference creates ring width

**Advantages**:
- Center space for additional text/labels
- Cleaner appearance
- Better for multiple series (nested rings)
- Reduces visual weight

**Common Uses**:
- Single metric in center
- Nested comparison (multiple rings)
- Modern dashboard aesthetics

---

## 3. Nightingale (Rose) Chart

Pie chart where slice radius varies by value (Nightingale/Rose chart).

```js
FAKE(csv(`rose 1,rose 2,rose 3,rose 4,rose 5,rose 6,rose 7,rose 8
40,38,32,30,28,26,22,18
`))

TRANSPOSE(header(true))
MAPVALUE(0, dict("name", value(0), "value", value(1)))

CHART(
    chartOption({
        legend: {
            "top": "bottom"
        },
        toolbox: {
            show: true,
            feature: {
                saveAsImage: { show: true, title: "save as image", name: "sample" }
            }
        },
        series: [
            {
                name: "Nightingale Chart",
                type: "pie",
                radius: ["50", "250"],
                center: ["50%", "50%"],
                roseType: "area",
                itemStyle: {
                    borderRadius: 8
                },
                data: column(0)
            }
        ]
    })
)
```

**Description**: Nightingale (Rose) chart where slice radius represents value magnitude. Named after Florence Nightingale who used this visualization for mortality data.

**Key Points**:

**Data Preparation**:
- `csv(...)` loads CSV with headers and values
- `TRANSPOSE(header(true))` pivots data with first row as headers
- `dict("name", ..., "value", ...)` creates object format
- Result: `[{name: "rose 1", value: 40}, {name: "rose 2", value: 38}, ...]`

**Rose Chart Configuration**:
- `roseType: "area"` enables Nightingale mode
  - Slice radius proportional to value
  - Larger values = longer slices
- `radius: ["50", "250"]` sets min/max radius in pixels
  - Inner: 50px
  - Outer: up to 250px (varies by value)
- `itemStyle.borderRadius: 8` rounds slice corners

**Visual Encoding**:
- **Angle**: Equal for all slices (like clock)
- **Radius**: Varies by value (key difference from pie)
- Emphasizes magnitude differences
- Better than pie for comparing values

**Toolbox**:
- `saveAsImage` enables download feature
- User can save chart as PNG
- Custom filename "sample"

**Rose Type Options**:
- `"area"`: Radius based on value
- `"radius"`: Radius based on square root of value
- Not set: Standard pie chart

**Advantages over Pie**:
- Easier value comparison
- Both angle and radius convey information
- Visually striking
- Better for data with large value ranges

**Use Cases**:
- Seasonal patterns (cyclical data)
- Rankings with categories
- Proportions where absolute values matter
- When visual impact is important