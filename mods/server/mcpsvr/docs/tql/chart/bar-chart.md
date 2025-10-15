# Machbase Neo Bar Chart

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

## 1. Basic Bar Chart

Basic bar chart example.

```js
FAKE( linspace(0, 360, 50))
MAPVALUE(2, sin((value(0)/180)*PI))
CHART(
    chartOption({
        xAxis:{ "data": column(0) },
        yAxis:{},
        series: [
            { type: "bar", data: column(1)}
        ]
    })
)
```

**Description**: Visualize sine function as a bar chart.

---

## 2. Category Bar (GROUP-by-lazy)

Grouped bar chart by category.

```js
FAKE( json({
    ["2011", "Brazil", 18203],
    ["2011", "Indonesia", 23489],
    ["2011", "USA", 29034],
    ["2011", "India", 104970],
    ["2011", "China", 131744],
    ["2011", "World", 630230],
    ["2022", "Brazil", 19325],
    ["2022", "Indonesia", 23438],
    ["2022", "USA", 31000],
    ["2022", "India", 121594],
    ["2022", "China", 134141],
    ["2022", "World", 681807]
}) )
// |   0      1         2
// +-> year   country   population
// |
MAPVALUE(3, value(0) == "2011" ? value(2) : 0)
// |   0      1         2            3
// +-> year   country   population   2011-population
// |
MAPVALUE(4, value(0) == "2022" ? value(2) : 0)
// |   0      1         2            3                  4
// +-> year   country   population   2011-population   2022-population
// |
POPVALUE(0, 2)
// |   0        1                  2
// +-> country  2011-population   2022-population
// |
GROUP( by(value(0)), max(value(1)), max(value(2)), lazy(true))
// |
CHART(
    chartOption({
        legend: { show:true},
        tooltip: {
            trigger: "axis",
            axisPointer: {
                type: "shadow"
            }
        },
        xAxis: { type: "category", data: column(0) },
        yAxis: { },
        series: [
            { type: "bar", name: "2011", data: column(1) },
            { type: "bar", name: "2022", data: column(2) }
        ]
    })
)
```

**Description**: Grouped bar chart comparing population data by country for 2011 and 2022.

---

## 3. Tangential Polar Bar

Bar chart using polar coordinate system.

```js
FAKE( json({
    ["A", 2],
    ["B", 1.2],
    ["C", 2.4],
    ["D", 3.6]
}) )

CHART(
    chartOption({
        title: {
            text: "Tangential Polar Bar Label Position (middle)"
        },
        polar: { radius: [30, "80%"] },
        radiusAxis: {
            type: "category",
            data: column(0)
        },
        angleAxis: { max: 4, startAngle: 90 },
        tooltip: {},
        series: {   
            type: "bar",
            coordinateSystem: "polar",
            data: column(1),
            label: {
                show: true,
                position: "middle",
                formatter: "{b}: {c}"
            }
        }
    })
)
```

**Description**: Chart with bars arranged in a circular pattern using polar coordinates.

---

## 4. Bar Chart with Negative Values

Bar chart including negative values.

```js
FAKE(csv(`day,profit,income,expenses
Mon,200,320,-120
Tue,170,302,-132
Wed,240,341,-101
Thu,244,374,-134
Fri,200,390,-190
Sat,220,450,-230
Sun,210,420,-210
`))

DROP(1) // drop header
MAPVALUE(1, parseFloat(value(1))) // parse float from string
MAPVALUE(2, parseFloat(value(2))) // parse float from string
MAPVALUE(3, parseFloat(value(3))) // parse float from string

CHART(
    chartOption({
        tooltip: {
            trigger: "axis",
            axisPointer: {
                type: "shadow"
            }
        },
        legend: {
            data: ["Profit", "Expenses", "Income"]
        },
        grid: {
            left: "3%",
            right: "4%",
            bottom: "3%",
            containLabel: true
        },
        xAxis: [
            {
                type: "value"
            }
        ],
        yAxis: [
            {
                type: "category",
                axisTick: {
                    show: false
                },
                data: column(0)
            }
        ],
        series: [
            {
                name: "Profit",
                type: "bar",
                label: {
                    show: true,
                    position: "inside"
                },
                emphasis: {
                    focus: "series"
                },
                data: column(1)
            },
            {
                name: "Income",
                type: "bar",
                stack: "Total",
                label: {
                    show: true
                },
                emphasis: {
                    focus: "series"
                },
                data: column(2)
            },
            {
                name: "Expenses",
                type: "bar",
                stack: "Total",
                label: {
                    show: true,
                    position: "left"
                },
                emphasis: {
                    focus: "series"
                },
                data: [-120, -132, -101, -134, -190, -230, -210]
            }
        ]
    })
)
```

**Description**: Chart showing profit, income, and expenses with negative values (expenses) displayed in stacked format.

---

## 5. Stacked Bar Normalization (Percentage)

Stacked bar chart normalized to percentages.

```js
FAKE(json({
    ["Day",  "Direct", "Mail Ad", "Affiliate Ad", "Video Ad", "Search Engine"],
    ["Mon", 100, 320, 220, 150, 820],
    ["Tue", 302, 132, 182, 212, 832],
    ["Wed", 301, 101, 191, 201, 901],
    ["Thu", 334, 134, 234, 154, 934],
    ["Fri", 390,  90, 290, 190, 1290],
    ["Sat", 330, 230, 330, 330, 1330],
    ["Sun", 320, 210, 310, 410, 1320]
}))
MAPVALUE(6, value(1)+value(2)+value(3)+value(4)+value(5), "Total")
CHART(
    chartOption({
        legend: {
            selectedMode: false
        },
        grid: {
            left: 100, right: 100, top: 50, bottom: 50
        },
        yAxis: { type: "value", show: false },
        xAxis: { type: "category", data: _column_0.slice(1) },
        series: [ ]
    }),
    chartJSCode({
        let total = _columns[6].slice(1)
        _columns.slice(1, 6).map((cols, cid) => {
            let name = cols[0];
            let data = cols.slice(1).map((v, did) => v / total[did]);
            _chartOption.series.push({
                name: name,
                type: 'bar',
                stack: 'total',
                barWidth: '60%',
                label: {
                    show: true,
                    formatter: (params) => Math.round(params.value*1000) / 10 + '%'
                },
                data: data
            })
        });
        _chart.setOption(_chartOption);
    })
)
```

**Description**: Normalized stacked chart displaying each category's proportion as a percentage.

---

## 6. Large Scale Bar Chart (500,000 Data)

Bar chart handling large-scale data.

```js
FAKE(linspace(0,1,1))
CHART(
    chartJSCode({
        const data = generateData(5e5);
        function generateData(count) {
            let baseValue = Math.random() * 1000;
            let time = +new Date(2011, 0, 1);
            let smallBaseValue;
            function next(idx) {
                smallBaseValue =
                    idx % 30 === 0
                        ? Math.random() * 700
                        : smallBaseValue + Math.random() * 500 - 250;
                baseValue += Math.random() * 20 - 10;
                return Math.max(0, Math.round(baseValue + smallBaseValue) + 3000);
            }
            const categoryData = [];
            const valueData = [];
            for (let i = 0; i < count; i++) {
                categoryData.push(
                    echarts.format.formatTime('yyyy-MM-dd\nhh:mm:ss', time, false)
                );
                valueData.push(next(i).toFixed(2));
                time += 1000;
            }
            return {
                categoryData: categoryData,
                valueData: valueData
            };
        }
    }),
    chartOption({
        title: {
            text: "500,000 Data",
            left: 10
        },
        toolbox: {
            feature: {
                dataZoom: {
                    yAxisIndex: false
                },
                saveAsImage: {
                    pixelRatio: 2
                }
            }
        },
        tooltip: {
            trigger: "axis",
            axisPointer: {
                type: "shadow"
            }
        },
        grid: {
            bottom: 90
        },
        dataZoom: [
            {
                "type": "inside"
            },
            {
                "type": "slider"
            }
        ],
        xAxis: {
            data: data.categoryData,
            silent: false,
            splitLine: {
                show: false
            },
            splitArea: {
                show: false
            }
        },
        yAxis: {
            splitArea: {
                show: false
            }
        },
        series: [
            {
                type: "bar",
                data: data.valueData,
                large: true
            }
        ]
    })
)
```

**Description**: Large-scale chart efficiently rendering 500,000 data points. Zoom in/out functionality available with `dataZoom`.

---

## 7. Bar Race

Animated bar chart that changes over time.

```js
CSV( file("https://docs.machbase.com/assets/example/life-expectancy-table.csv") )
CHART(
    chartOption({
        grid: {
            top: 10,
            bottom: 30,
            left: 150,
            right: 80
        },
        xAxis: {
            max: "dataMax",
            axisLabel: { }
        },
        dataset: {
            source: [],
        },
        yAxis: {
            type: "category",
            inverse: true,
            max: 10,
            axisLabel: {
                show: true,
                fontSize: 14,
                rich: {
                    flag: {
                        fontSize: 25,
                        padding: 5
                    }
                }
            },
            animationDuration: 300,
            animationDurationUpdate: 300
        },
        series: [
            {
                realtimeSort: true,
                type: "bar",
                seriesLayoutBy: "column",
                itemStyle: {
                    color:""
                },
                encode: {
                    x: 0, y: 3
                },
                label: {
                    show: true,
                    precision: 1,
                    position: "right",
                    valueAnimation: true,
                    fontFamily: "monospace"
                }
            }
        ],
        animationDuration: 0,
        animationDurationUpdate: 2000,
        animationEasing: "linear",
        animationEasingUpdate: "linear",
        graphic: {
            elements: [
                {
                    type: "text",
                    right: 40,
                    bottom: 60,
                    style: {
                        text: "loading...",
                        font: "bolder 50px monospace",
                        fill: "rgba(100, 100, 100, 0.25)"
                    },
                    z: 100
                }
            ]
        }
    }),
    chartJSCode({
        fetch("https://fastly.jsdelivr.net/npm/emoji-flags@1.3.0/data.json").then( function(rsp) {
            return rsp.json();
        }).then( function(flags) {
            const data = [];
            for (let i = 0; i < _columns[0].length; ++i) {
                var row = [];
                for (let c = 0; c < _columns.length; ++c) {
                    row.push(_columns[c][i]);
                }
                data.push(row);
            }

            const years = [];
            for (let i = 0; i < data.length; ++i) {
                if (years.length === 0 || years[years.length - 1] !== data[i][4]) {
                    years.push(data[i][4]);
                }
            }

            const updateFrequency = 2000;
            const countryColors = {
                "Australia": "#00008b",
                "Canada": "#f00",
                "China": "#ffde00",
                "Cuba": "#002a8f",
                "Finland": "#003580",
                "France": "#ed2939",
                "Germany": "#000",
                "Iceland": "#003897",
                "India": "#f93",
                "Japan": "#bc002d",
                "North Korea": "#024fa2",
                "South Korea": "#000",
                "New Zealand": "#00247d",
                "Norway": "#ef2b2d",
                "Poland": "#dc143c",
                "Russia": "#d52b1e",
                "Turkey": "#e30a17",
                "United Kingdom": "#00247d",
                "United States": "#b22234"
            };
            function getFlag(countryName) {
                if (!countryName) {
                    return '';
                }
                return (
                    flags.find(function (item) {
                        return item.name === countryName;
                    }) || {}
                ).emoji;
            }
            
            let startIndex = 10;
            let startYear = years[startIndex];
            let option = _chart.getOption()
            option.dataset.source = data.slice(1).filter(function (d) {
                return d[4] === startYear;
            });
            option.xAxis[0].axisLabel.formatter = function (n) {
                return Math.round(n) + '';
            };
            option.yAxis[0].axisLabel.formatter = function (value) {
                return value + "{flag|" + getFlag(value) + "}";
            };
            option.series[0].itemStyle.color = function (param) {
                return countryColors[param.value[3]] || "#5470c6";
            };
            option.graphic[0].elements[0].style.text = startYear;
            _chart.setOption(option);
            for (let i = startIndex; i < years.length - 1; ++i) {
                (function (i) {
                    setTimeout(function () {
                        updateYear(years[i + 1]);
                    }, (i - startIndex) * updateFrequency);
                })(i);
            }
            function updateYear(year) {
                let source = data.slice(1).filter(function (d) {
                    return d[4] === year;
                });
                option.series[0].data = source;
                option.graphic[0].elements[0].style.text = year;
                _chart.setOption(option);
            }
        }).catch(function(err){
            console.warn("data error, fetch resource", err)
        });
    })
)
```

**Description**: Bar race chart showing life expectancy data by country in chronological animation. Uses external CSV file and country flag emojis.