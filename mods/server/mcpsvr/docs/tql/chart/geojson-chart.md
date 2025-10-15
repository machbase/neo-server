# Machbase Neo GeoJSON Chart

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
| `SCRIPT()` | JavaScript processing | `SCRIPT({}, { /* process */ }, {})` |

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

## GeoJSON Map Chart - Seoul Districts

Choropleth map visualization using GeoJSON data for Seoul districts.

```js
FAKE(json({
    ["노원구", 10], ["도봉구", 50], ["강북구", 90],
    ["성북구", 20], ["종로구", 10], ["서대문구", 40],
    ["은평구", 90], ["마포구", 60], ["강서구", 20],
    ["양천구", 55], ["구로구", 75], ["영등포구", 35],
    ["중구", 100], ["용산구", 20], ["성동구", 65],
    ["광진구", 25], ["동대문구", 10], ["중랑구", 70],
    ["강동구", 10], ["송파구", 30], ["강남구", 50],
    ["서초구", 50], ["동작구", 90], ["관악구", 70], ["금천구", 90]
}))
SCRIPT({
    data = [];
},{
    data.push({name:$.values[0], value:$.values[1]})
}, {
    $.yield(data)
})
CHART(
    chartOption({
        title:{ text: "GEOJSON - Seoul"},
        tooltip: { trigger: "item", formatter: "{b}<br/>{c} %"},
        visualMap: {
            min: 0,
            max: 100,
            text: ["100%", "0%"],
            realtime: false,
            calculable: true,
            inRange: {
                color: [ "#89b6fe", "#25529a"]
            },
        },
        series: []
    }),
    chartJSCode({
        fetch("https://docs.machbase.com/assets/example/seoul_gu.json"
        ).then( function(rsp) {
            return rsp.json();
        }).then( function(seoulJSON) {
            echarts.registerMap("seoul_gu", seoulJSON);
            _chartOption.geo = {
                map: "seoul_gu",
                zoom: 1.2,
                roam: true,
                itemStyle: {
                    areaColor: "#e7e8ea"
                }
            };
            _chartOption.series[0] ={
                type: "map",
                geoIndex: 0,
                data: column(0)[0]
            };
            _chart.setOption(_chartOption);
        }).catch(function(err){
            console.warn("geojson error", err)
        });
    })
)
```

**Description**: Choropleth map of Seoul's 25 districts (구) with color-coded percentage values. Loads GeoJSON geographic boundaries and maps data values to colors using a visual gradient.

**Key Points**:

**Data Preparation**:
- `FAKE(json({...}))` creates district name and value pairs
- `SCRIPT()` transforms data into `{name, value}` objects
- Three-stage SCRIPT: initialize, process each record, finalize

**GeoJSON Integration**:
- `fetch()` loads GeoJSON file containing Seoul district boundaries
- `echarts.registerMap()` registers the geographic data
- Map name "seoul_gu" links chartOption to registered GeoJSON

**Visual Mapping**:
- `visualMap` creates color legend (0-100%)
- `inRange.color` defines gradient from light blue (#89b6fe) to dark blue (#25529a)
- `calculable: true` enables interactive range adjustment

**Map Configuration**:
- `geo.map` references registered GeoJSON map
- `zoom: 1.2` sets initial zoom level
- `roam: true` enables pan and zoom interactions
- `itemStyle.areaColor` sets default area color

**Data Binding**:
- `series[0].type: "map"` creates map series
- `geoIndex: 0` links series to geo component
- `data: column(0)[0]` binds district values to map regions
- Region names in data must match GeoJSON feature names

**Usage Pattern**:
1. Prepare data with geographic region names
2. Fetch and register GeoJSON file
3. Configure visual mapping for value-to-color conversion
4. Link data to map regions by name matching