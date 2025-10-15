# Machbase Neo 3D Bar Chart

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

## 1. 3D Bar with Dataset

3D bar chart using external dataset.

```js
CSV( file("https://docs.machbase.com/assets/example/life-expectancy-table.csv") )
// |   0        1                 2            3        4
// +-> income   life-expectancy   population   country  year
// |
DROP(1) // drop header
// |
MAPVALUE(0, value(4))
// |   0        1                 2            3        4
// +-> year     life-expectancy   population   country  year
// |
POPVALUE(4)
// |   0        1                 2            3
// +-> year     life-expectancy   population   country
// |
MAPVALUE(1, parseFloat(value(1)) )
MAPVALUE(2, parseFloat(value(2)) )
// |   0        1                 2            3 
// +-> year     life-expectancy   population   country
// |
MAPVALUE(0, list(value(0), value(1), value(2), value(3)))
POPVALUE(1,2,3)
// |   0 
// +-> [year, life-expectancy, population, country]
// |
CHART(
    plugins("gl"),
    chartOption({
        grid3D: {},
        tooltip: {},
        xAxis3D: { type: "category" },
        yAxis3D: { type: "category" },
        zAxis3D: {},
        visualMap: { max: 100000000, dimension: "Population"},
        dataset: {
            dimensions: [
                { name: "Year", type: "ordinal"},
                "Life Expectancy",
                "Population",
                "Country"
            ],
            source: column(0)
        },
        series: [
            {
                type: "bar3D",
                shading: "lambert",
                encode: {
                    x: "Year",
                    y: "Country",
                    z: "Lefe Expectancy",
                    tooltip: [0, 1, 2, 3]
                }
            }
        ]
    })
)
```

**Description**: 3D bar chart displaying life expectancy data by year and country using external CSV file. Uses `dataset` for data binding and `visualMap` for color mapping based on population.

---

## 2. Stacked 3D Bar

Stacked 3D bar chart with multiple series.

```js
FAKE( meshgrid(linspace(0, 10, 11), linspace(0, 10, 11)) )
MAPVALUE(2, list( value(0), value(1), simplex(10, value(0)/5, value(1)/5) * 2 + 4))
MAPVALUE(3, list( value(0), value(1), simplex(20, value(0)/5, value(1)/5) * 2 + 4))
MAPVALUE(4, list( value(0), value(1), simplex(30, value(0)/5, value(1)/5) * 2 + 4))
MAPVALUE(5, list( value(0), value(1), simplex(40, value(0)/5, value(1)/5) * 2 + 4))
POPVALUE(0,1)
CHART(
    plugins("gl"),
    chartOption({
        xAxis3D: { type: "value" },
        yAxis3D: { type: "value" },
        zAxis3D: { type: "value" },
        grid3D: {
            viewControl: {
                // autoRotate: true
            },
            light: {
                main: {
                    shadow: true,
                    quality: "ultra",
                    intensity: 1.5
                }
            }
        },
        series: [
            {
                type: "bar3D",
                data: column(0),
                stack: "stack",
                shading: "lambert",
                emphasis: {
                    label: { show: false }
                }
            },
            {
                type: "bar3D",
                data: column(1),
                stack: "stack",
                shading: "lambert",
                emphasis: {
                    label: { show: false }
                }
            },
            {
                type: "bar3D",
                data: column(2),
                stack: "stack",
                shading: "lambert",
                emphasis: {
                    label: { show: false }
                }
            },
            {
                type: "bar3D",
                data: column(3),
                stack: "stack",
                shading: "lambert",
                emphasis: {
                    label: { show: false }
                }
            }
        ]
    })
)
```

**Description**: Stacked 3D bar chart using simplex noise to generate multiple data layers. Features advanced lighting with shadows and high-quality rendering using lambert shading.

---

## 3. Transparent 3D Bar

3D bar chart with transparency and custom styling.

```js
FAKE( json({
    [0, 0, 5], [0, 1, 1], [0, 2, 0], [0, 3, 0], [0, 4, 0], [0, 5, 0], [0, 6, 0], [0, 7, 0],
    [0, 8, 0], [0, 9, 0], [0, 10, 0], [0, 11, 2], [0, 12, 4], [0, 13, 1], [0, 14, 1], [0, 15, 3],
    [0, 16, 4], [0, 17, 6], [0, 18, 4], [0, 19, 4], [0, 20, 3], [0, 21, 3], [0, 22, 2], [0, 23, 5],
    [1, 0, 7], [1, 1, 0], [1, 2, 0], [1, 3, 0], [1, 4, 0], [1, 5, 0], [1, 6, 0], [1, 7, 0],
    [1, 8, 0], [1, 9, 0], [1, 10, 5], [1, 11, 2], [1, 12, 2], [1, 13, 6], [1, 14, 9], [1, 15, 11],
    [1, 16, 6], [1, 17, 7], [1, 18, 8], [1, 19, 12], [1, 20, 5], [1, 21, 5], [1, 22, 7], [1, 23, 2],
    [2, 0, 1], [2, 1, 1], [2, 2, 0], [2, 3, 0], [2, 4, 0], [2, 5, 0], [2, 6, 0], [2, 7, 0],
    [2, 8, 0], [2, 9, 0], [2, 10, 3], [2, 11, 2], [2, 12, 1], [2, 13, 9], [2, 14, 8], [2, 15, 10],
    [2, 16, 6], [2, 17, 5], [2, 18, 5], [2, 19, 5], [2, 20, 7], [2, 21, 4], [2, 22, 2], [2, 23, 4],
    [3, 0, 7], [3, 1, 3], [3, 2, 0], [3, 3, 0], [3, 4, 0], [3, 5, 0], [3, 6, 0], [3, 7, 0],
    [3, 8, 1], [3, 9, 0], [3, 10, 5], [3, 11, 4], [3, 12, 7], [3, 13, 14], [3, 14, 13], [3, 15, 12],
    [3, 16, 9], [3, 17, 5], [3, 18, 5], [3, 19, 10], [3, 20, 6], [3, 21, 4], [3, 22, 4], [3, 23, 1],
    [4, 0, 1], [4, 1, 3], [4, 2, 0], [4, 3, 0], [4, 4, 0], [4, 5, 1], [4, 6, 0], [4, 7, 0],
    [4, 8, 0], [4, 9, 2], [4, 10, 4], [4, 11, 4], [4, 12, 2], [4, 13, 4], [4, 14, 4], [4, 15, 14],
    [4, 16, 12], [4, 17, 1], [4, 18, 8], [4, 19, 5], [4, 20, 3], [4, 21, 7], [4, 22, 3], [4, 23, 0],
    [5, 0, 2], [5, 1, 1], [5, 2, 0], [5, 3, 3], [5, 4, 0], [5, 5, 0], [5, 6, 0], [5, 7, 0],
    [5, 8, 2], [5, 9, 0], [5, 10, 4], [5, 11, 1], [5, 12, 5], [5, 13, 10], [5, 14, 5], [5, 15, 7],
    [5, 16, 11], [5, 17, 6], [5, 18, 0], [5, 19, 5], [5, 20, 3], [5, 21, 4], [5, 22, 2], [5, 23, 0],
    [6, 0, 1], [6, 1, 0], [6, 2, 0], [6, 3, 0], [6, 4, 0], [6, 5, 0], [6, 6, 0], [6, 7, 0],
    [6, 8, 0], [6, 9, 0], [6, 10, 1], [6, 11, 0], [6, 12, 2], [6, 13, 1], [6, 14, 3], [6, 15, 4],
    [6, 16, 0], [6, 17, 0], [6, 18, 0], [6, 19, 0], [6, 20, 1], [6, 21, 2], [6, 22, 2], [6, 23, 6]
}) )
MAPVALUE(0, list(value(1), value(0), value(2)))
POPVALUE(1,2)
CHART(
    plugins("gl"),
    chartJSCode({
        var hours = ['12a', '1a', '2a', '3a', '4a', '5a', '6a',
                    '7a', '8a', '9a', '10a', '11a',
                    '12p', '1p', '2p', '3p', '4p', '5p',
                    '6p', '7p', '8p', '9p', '10p', '11p'];
        var days = ['Saturday', 'Friday', 'Thursday',
                    'Wednesday', 'Tuesday', 'Monday', 'Sunday'];
    }),
    chartOption({
        tooltip: {},
        visualMap: {
            max: 20,
            inRange: {
                color: [
                    "#313695", "#4575b4", "#74add1", "#abd9e9", "#e0f3f8", "#ffffbf",
                    "#fee090", "#fdae61", "#f46d43", "#d73027", "#a50026"
                ]
            }
        },
        xAxis3D: { type: "category", data: hours },
        yAxis3D: { type: "category", data: days },
        zAxis3D: { type: "value" },
        grid3D: {
            boxWidth: 200,
            boxDepth: 80,
            light: {
                main: {
                    intensity: 1.2
                },
                ambient: {
                    intensity: 0.3
                }
            }
        },
        series: [
            {
                type: "bar3D",
                data: column(0),
                shading: "color",
                label: {
                    show: false,
                    fontSize: 16,
                    borderWidth: 1
                },
                itemStyle: {
                    opacity: 0.6
                },
                emphasis: {
                    label: {
                        fontSize: 20,
                        color: '#900'
                    },
                    itemStyle: {
                        color: '#900'
                    }
                }
            }
        ]
    })
)
```

**Description**: Transparent 3D bar chart showing activity by day and hour. Uses semi-transparent bars (opacity: 0.6) with color shading and a multi-color gradient via visualMap. Features custom emphasis styling on hover.