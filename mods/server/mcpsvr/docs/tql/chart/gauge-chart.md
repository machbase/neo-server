# Machbase Neo Gauge Chart

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

## 1. Basic Gauge

Simple gauge chart displaying a single value.

### Using SCRIPT

```js
SCRIPT({
    value = 55;
    $.yield({
      tooltip: { formatter: "{a} <br/>{b} : {c}%" },
      series: [
        {
          name: "Pressure",
          type: "gauge",
          detail: { formatter: "{value}" },
          data: [
              { value: value, name: "PRESSURE" }
          ]
        }
      ]
    })
})
CHART()
```

### Using FAKE

```js
FAKE(linspace(55, 60, 1))
CHART(
    chartOption({
        tooltip: { formatter: "{a} <br/>{b} : {c}%" },
        series: [
          {
            name: "Pressure",
            type: "gauge",
            detail: { formatter: "{value}" },
            data: [
                { value: column(0)[0], name: "PRESSURE" }
            ]
          }
        ]
    })
)
```

**Description**: Basic gauge chart showing pressure value. Displays a traditional dial-style gauge with needle pointer. Shows two approaches: using SCRIPT to directly define the value, or using FAKE with data pipeline.

**Key Points**:
- Gauge displays single numeric value
- `detail.formatter` controls value display format
- `tooltip.formatter` customizes hover information
- Default gauge range is 0-100

---

## 2. Speed Gauge

Modern gauge chart with progress bar styling.

```js
FAKE(json(`[70]`))
CHART(
    chartOption({
        series: [{
            type: "gauge",
            progress: {
                show: true,
                width: 18
            },
            axisLine: {
                lineStyle: {
                    width: 18
                }
            },
            axisTick: {
                show: false
            },
            splitLine: {
                length: 15,
                lineStyle: {
                    width: 2,
                    color: "#999"
                }
            },
            axisLabel: {
                distance: 25,
                color: "#999",
                fontSize: 20
            },
            anchor: {
                show: true,
                showAbove: true,
                size: 25,
                itemStyle: {
                    borderWidth: 10
                }
            },
            title: {
                show: false
            },
            detail: {
                valueAnimation: true,
                fontSize: 80,
                offsetCenter: [0, "70%"]
            },
            data: [
                {
                    value: column(0)
                }
            ]
        }]
    })
)
```

**Description**: Modern speedometer-style gauge with circular progress bar. Features large numeric display with smooth value animation and customized styling for a contemporary look.

**Key Points**:
- `progress.show: true` enables circular progress bar visualization
- `axisTick.show: false` hides tick marks for cleaner appearance
- `anchor` creates a prominent center pin/anchor point
- `detail.valueAnimation` enables smooth number transitions
- `detail.fontSize: 80` creates large, prominent value display
- `detail.offsetCenter` positions the value text

---

## 3. Update Gauge (Real-time)

Gauge chart with automatic real-time updates.

```js
FAKE(linspace(0, 1, 1))
CHART(
    chartOption({
        tooltip: {
            formatter: "{a} <br/>{b} : {c}%"
        },
        series: [
            {
                name: "Pressure",
                type: "gauge",
                progress: {
                    show: true
                },
                detail: {
                    valueAnimation: true,
                    formatter: "{value}"
                },
                data: [
                    {
                        value: 0,
                        name: "RANDOM"
                    }
                ]
            }
        ]
    }),
    chartJSCode({
        function updateGauge() {
            fetch("/db/tql", {
                method: "POST",
                body: `
                    FAKE(linspace(0, 1, 1))
                    MAPVALUE(0, floor(random() * 100))
                    JSON()
                `
            }).then(function(rsp){
                return rsp.json()
            }).then(function(obj){
                _chartOption.series[0].data[0].value = obj.data.rows[0][0]
                _chart.setOption(_chartOption)
                if (document.getElementById(_chartID) != null) {
                    setTimeout(updateGauge, 1000)
                }
            }).catch(function(err){
                console.warn("data fetch error", err)
            });
        };
        setTimeout(updateGauge, 10)
    })
)
```

**Description**: Animated gauge that updates every second with random values fetched from the database. Demonstrates real-time data visualization with automatic refresh using TQL API calls.

**Key Points**:
- `chartJSCode()` implements custom update logic
- Uses `fetch("/db/tql")` to execute TQL queries from client-side
- `setTimeout()` creates periodic updates (1 second interval)
- `valueAnimation: true` enables smooth value transitions
- Checks `document.getElementById(_chartID)` to stop updates when chart is removed
- `_chart.setOption()` updates chart with new data
- `random() * 100` generates values between 0-100

**Update Mechanism**:
1. Initial chart renders with value 0
2. After 10ms, first update executes
3. TQL query generates random value
4. Chart updates with new value
5. Process repeats every 1000ms
6. Stops automatically when chart element is removed from DOM