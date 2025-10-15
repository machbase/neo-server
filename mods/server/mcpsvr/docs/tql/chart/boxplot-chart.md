# Machbase Neo Boxplot Chart

## Quick Reference

### TQL Pipeline Structure

TQL operates in a **data flow (pipeline)**0 manner:

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
| `BOXPLOT()` | Calculate boxplot statistics | `BOXPLOT(value(0), category(value(1)))` |

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

### BOXPLOT() Function

**Syntax**: `BOXPLOT(value, [category()], [order()], [boxplotInterp()], [boxplotOutput()])`

*Available since version 8.0.15*

Calculate boxplot statistics (Q1, Q3, median, min, max, outliers) for the given values.

#### Options

**category()**
- `category(categoryValue)` - Group data by category

**order()**
- `order(cat1, cat2, ...)` - Specify category order

**boxplotInterp()**
- `boxplotInterp(lowerFence, upperFence, outlier)` - Boolean flags for interpolation
- Controls how min/max and outliers are calculated

**boxplotOutput()**
- `boxplotOutput("chart")` - Format output for ECharts
- Produces data structure compatible with ECharts boxplot series

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

## 1. Michelson-Morley Experiment

Boxplot chart showing the Michelson-Morley experiment data.

```js
FAKE(json({
    ["A", 850, 740, 900, 1070, 930, 850, 950, 980, 980, 880, 1000, 980, 930, 650, 760, 810, 1000, 1000, 960, 960],
    ["B", 960, 940, 960, 940, 880, 800, 850, 880, 900, 840, 830, 790, 810, 880, 880, 830, 800, 790, 760, 800],
    ["C", 880, 880, 880, 860, 720, 720, 620, 860, 970, 950, 880, 910, 850, 870, 840, 840, 850, 840, 840, 840],
    ["D", 890, 810, 810, 820, 800, 770, 760, 740, 750, 760, 910, 920, 890, 860, 880, 720, 840, 850, 850, 780],
    ["E", 890, 840, 780, 810, 760, 810, 790, 810, 820, 850, 870, 870, 810, 740, 810, 940, 950, 800, 810, 870]
}))
TRANSPOSE(fixed(0))
BOXPLOT(
    value(1),
    category(value(0)),
    boxplotInterp(true, false, true),
    boxplotOutput("chart")
)
CHART(
    chartOption({
        grid: { bottom: "15%" },
        xAxis:{ type:"category", boundaryGap: true, data: column(0) },
        yAxis:{ type:"value", name: "km/s minus 299,000", min:400, splitArea:{ show: true } },
        series:[
            { name: "boxplot", type:"boxplot", data: column(1)},
            { name: "outlier", type:"scatter", data: column(2).flat()},
        ],
        tooltip: { trigger: 'item', axisPointer: { type: 'shadow' } },
        title:[
            {
                text: "Michelson-Morley Experiment",
                left: "center"
            },
            {
                text: "max: Q3 + 1.5 * IQR \nmin: Q1 - 1.5 * IQR",
                borderColor: "#999",
                borderWidth: 1,
                textStyle: { fontWeight: "normal", fontSize: 12, lineHeight: 16 },
                left: "10%", top: "92%"
            }
        ]
    })
)
```

**Description**: Boxplot chart visualizing the Michelson-Morley experiment data across 5 experimental runs (A-E). The chart shows the distribution of speed measurements with quartiles, median, and outliers. Uses `BOXPLOT()` with `boxplotInterp(true, false, true)` for custom outlier calculation and displays outliers as scatter points.

**Key Points**:
- `TRANSPOSE(fixed(0))` transforms the data structure for category-based analysis
- `boxplotOutput("chart")` formats data for ECharts compatibility
- Two series: boxplot for the main distribution and scatter for outliers
- `column(2).flat()` flattens the outlier array for proper rendering

---

## 2. Iris Sepal Length

Boxplot chart analyzing iris flower sepal length by species.

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
MAPVALUE(4, strToUpper(strTrimPrefix(value(4), "Iris-")))

BOXPLOT(
    value(0),
    category(value(4)),
    order("SETOSA", "VERSICOLOR", "VIRGINICA"),
    boxplotOutput("chart")
)

CHART(
    chartOption({
        grid: { bottom: "15%" },
        xAxis:{ type:"category", boundaryGap: true, data: column(0) },
        yAxis:{ type:"value", name: "sepal length", min:4, max:8, splitArea:{ show: true } },
        series:[
            { name: "sepal length", type:"boxplot", data: column(1)},
            { name: "outlier", type:"scatter", data: column(2).flat()},
        ],
        tooltip: { trigger: 'item', axisPointer: { type: 'shadow' } },
        legend: {show: true, bottom:'2%'},
        title:[ { text: "Iris Sepal Length", left: "center" } ]
    })
)
```

**Description**: Boxplot chart comparing sepal length distribution across three iris species (Setosa, Versicolor, Virginica). Loads data from external CSV file and analyzes the statistical distribution of sepal measurements for each species.

**Key Points**:
- `CSV(file(...))` loads iris dataset from external source
- `strTrimPrefix()` cleans species names by removing "Iris-" prefix
- `order("SETOSA", "VERSICOLOR", "VIRGINICA")` ensures consistent category ordering
- `value(0)` represents sepal length measurements
- `value(4)` represents species category
- Displays both boxplot distribution and outlier points as separate series