# Machbase Neo Line Chart

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
| `HTTP()` | HTTP request | `HTTP('GET https://example.com/data.csv')` |
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
| `FILTER()` | Filter records | `FILTER(value(0) > 10)` |
| `DROP()` | Drop first N records | `DROP(1)` |
| `SCRIPT()` | JavaScript processing | `SCRIPT({}, { /* process */ }, {})` |

---

### SINK - Data Output

Functions that **output or save data** (pipeline end)

| Function | Purpose | Example |
|----------|---------|---------|
| `CHART()` | Create chart | `CHART(chartOption({...}))` |
| `CSV()` | CSV output | `CSV()` |
| `JSON()` | JSON output | `JSON()` |
| `HTML()` | HTML output | `HTML(template({...}))` |
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

## 1. Basic Line Chart

Simple line chart with multiple data source approaches.

### Using FAKE

```js
FAKE( linspace(0, 360, 100))
// |   0
// +-> x
// |
MAPVALUE(1, sin((value(0)/180)*PI))
// |   0   1
// +-> x   sin(x)
// |
CHART(
    chartOption({
        xAxis: {
            type: "category",
            data: column(0)
        },
        yAxis: {},
        series: [
            {
                type: "line",
                data: column(1)
            }
        ]
    })
)
```

### Using SCRIPT

```js
SCRIPT({
    for( x = 0; x < 360; x+=3.6) {
        $.yield(x, Math.sin(x/180*Math.PI));
    }
})
CHART(
    chartOption({
        xAxis: {
            type: "category",
            data: column(0)
        },
        yAxis: {},
        series: [
            {
                type: "line",
                data: column(1)
            }
        ]
    })
)
```

### Using SQL

**Prepare data first:**

```js
FAKE( arrange(1, 100, 1))
// |   0
// +-> seq
// |
MAPVALUE(1, sin((2*PI*value(0)/100)))
// |   0       1
// +-> seq     value
// |
MAPVALUE(0, timeAdd("now-100s", strSprintf("+%.fs", value(0))))
// |   0       1
// +-> time    value
// |
PUSHVALUE(0, "chart-line")
// |   0       1       2
// +-> name    time    value
// |
APPEND(table("example"))
```

**Prepare data with SCRIPT**

```js
SCRIPT({
    const gen = require("@jsh/generator");
    const sys = require("@jsh/system");
    ts = (new Date()).getTime() - 100 * 1000; // now - 100s.
    for(x of gen.arrange(1, 100, 1)) {
        y = Math.sin(x/100*2*Math.PI)
        ts += 1000; // add 1 sec.
        $.yield("chart-line", sys.parseTime(ts, "ms"), y);
    }
})
APPEND(table("example"))
```

**Query and visualize:**

```js
SQL(`select time, value from example where name = 'chart-line'`)
SCRIPT({
    $.yield([$.values[0], $.values[1]])
})
CHART(
    chartOption({
        xAxis: { type: "time" },
        yAxis: {},
        tooltip: { trigger:"axis" },
        series: [
            {
                type: "line",
                data: column(0)
            }
        ]
    })
)
```

### Using SCRIPT with DB Client

```js
SCRIPT({
    db = require("@jsh/db");
    cli = new db.Client();
    conn = cli.connect();
    rows = conn.query(`select time, value from example where name = 'chart-line'`)
    data = [];
    for( r of rows) {
        data.push([r.time, r.value]);
    }
    rows.close();
    conn.close();
    $.yield({
        xAxis: { type: "time" },
        yAxis: {},
        tooltip: { trigger:"axis" },
        series: [
            {
                type: "line",
                data: data,
            }
        ]
    })
})
CHART()
```

### Using HTML Template

```html
SQL(`select time, value from example where name = 'chart-line'`)
SCRIPT({
    data = [];
}, {
    data.push([$.values[0], $.values[1]]);
}, {
    $.yield(data); 
})
HTML(template({
    <span>
        <script src="/web/echarts/echarts.min.js"></script>
        <div id='xyz' style="width:600px;height:400px;"></div>
        <script>
            var data = {{ .Values }};
            var chartDom = document.getElementById('xyz');
            var myChart = echarts.init(chartDom);
            var option = {
                xAxis: { type: "time" },
                yAxis: {},
                tooltip: { trigger:"axis" },
                series: [
                    {type: 'line',  data: data[0], symbol:"none"}
                ]
            };
            option && myChart.setOption(option);
        </script>
    </span>
}))
```

**Description**: Basic line chart demonstrating five different approaches:
- **FAKE**: Generate test data
- **SCRIPT**: Custom JavaScript logic
- **SQL**: Database query with pipeline processing
- **SCRIPT with DB Client**: Direct database access using `@jsh/db` module
- **HTML Template**: Standalone HTML output with embedded ECharts

---

## 2. Basic Area Chart

Line chart with filled area under the curve.

```js
FAKE( json({
    ["Mon", 820],
    ["Tue", 932],
    ["Wed", 901],
    ["Thu", 934],
    ["Fri", 1290],
    ["Sat", 1330],
    ["Sun", 1320]
}) )
// |   0      1
// +-> day    value
// |
CHART(
    chartOption({
        legend:{ show:false },
        xAxis: { type:"category", data: column(0) },
        yAxis: {},
        series:[
            { type: "line", smooth:false, color:"#7585CE", areaStyle:{}, data: column(1) }
        ]
    })
)
```

**Description**: Area chart showing weekly data. The `areaStyle:{}` option fills the area below the line.

---

## 3. Stacked Line Chart

Multiple line series stacked on top of each other.

```js
FAKE( json({
    ["Mon", 120, 220, 150, 320, 820],
    ["Tue", 132, 182, 232, 332, 932],
    ["Wed", 101, 191, 201, 301, 901],
    ["Thu", 134, 234, 154, 334, 934],
    ["Fri",  90, 290, 190, 390, 1290],
    ["Sat", 230, 330, 330, 330, 1330],
    ["Sun", 210, 310, 410, 320, 1320]
}) )
// |   0      1       2      3      4       5
// +-> day    email   ads    video  direct  search
// |
CHART(
    chartOption({
        xAxis: { data: column(0) },
        yAxis: {},
        series: [
            {type: "line", data: column(1), smooth:false, name: "email", stack: "total"},
            {type: "line", data: column(2), smooth:false, name: "ads", stack: "total"},
            {type: "line", data: column(3), smooth:false, name: "video", stack: "total"},
            {type: "line", data: column(4), smooth:false, name: "direct", stack: "total"},
            {type: "line", data: column(5), smooth:false, name: "search", stack: "total"}
        ]
    })
)
```

**Description**: Stacked line chart showing multiple traffic sources. All series share `stack: "total"` to stack values.

---

## 4. Stacked Area Chart

Stacked area chart with labels on top series.

```js
FAKE( json({
    ["Mon", 120, 220, 150, 320, 820],
    ["Tue", 132, 182, 232, 332, 932],
    ["Wed", 101, 191, 201, 301, 901],
    ["Thu", 134, 234, 154, 334, 934],
    ["Fri",  90, 290, 190, 390, 1290],
    ["Sat", 230, 330, 330, 330, 1330],
    ["Sun", 210, 310, 410, 320, 1320]
}) )
// |   0      1       2      3      4       5
// +-> day    email   ads    video  direct  search
// |
CHART(
    chartOption({
        xAxis: {data: column(0)},
        yAxis: {},
        animation: false,
        series: [
            {type: "line", data:column(1), name: "email", stack: "total", areaStyle:{} },
            {type: "line", data:column(2), name: "ads", stack: "total", areaStyle:{} },
            {type: "line", data:column(3), name: "video", stack: "total", areaStyle:{} },
            {type: "line", data:column(4), name: "direct", stack: "total", areaStyle:{} },
            {type: "line", data:column(5), name: "search", stack: "total", areaStyle:{},
                label: {show: true, position: "top"}
            }
        ]
    })
)
```

**Description**: Similar to stacked line but with filled areas. Top series shows values with labels.

---

## 5. Area Pieces

Area chart with colored segments using visualMap.

```js
SCRIPT({
    data = [
        ["2019-10-10", 200], ["2019-10-11", 560], ["2019-10-12", 750],
        ["2019-10-13", 580], ["2019-10-14", 250], ["2019-10-15", 300],
        ["2019-10-16", 450], ["2019-10-17", 300], ["2019-10-18", 100]
    ];
    $.yield({
      title: { text: "Area Pieces" },
      xAxis: { type: "category", boundaryGap: false },
      yAxis: { type: "value", boundaryGap: [0, "30%"] },
      visualMap:{
        type: "piecewise",
        show: false,
        dimension: 0,
        seriesIndex: 0,
        pieces: [
          { gt: 1, lt: 3, color: "rgba(0, 0, 180, 0.4)" },
          { gt: 5, lt: 7, color: "rgba(0, 0, 180, 0.4)" }
        ]
      },
      series: [
        {
          type: "line",
          smooth: 0.6,
          symbol: "none",
          data: data,
          lineStyle: { color: "#5470C6", width: 5 },
          areaStyle:{},
          markLine: {
            symbol: ["none", "none"],
            label: { show: false },
            data: [{ xAxis: 1 }, { xAxis: 3 }, { xAxis: 5 }, { xAxis: 7 }]
          }
        }   
      ]
    })
})
CHART()
```

```js
FAKE(
  json({
        ["2019-10-10", 200], ["2019-10-11", 560], ["2019-10-12", 750],
        ["2019-10-13", 580], ["2019-10-14", 250], ["2019-10-15", 300],
        ["2019-10-16", 450], ["2019-10-17", 300], ["2019-10-18", 100]
  })
)
// |   0      1
// +-> date   value
// |
MAPVALUE(0, list(value(0), value(1)))
// |   0               1
// +-> [date, value]   value
// |
POPVALUE(1)
// |   0
// +-> [date, value]
// |
CHART(
  chartOption({
    title: { text: "Area Pieces" },
    xAxis: { type: "category", boundaryGap: false },
    yAxis: { type: "value", boundaryGap: [0, "30%"] },
    visualMap:{
      type: "piecewise",
      show: false,
      dimension: 0,
      seriesIndex: 0,
      pieces: [
        { gt: 1, lt: 3, color: "rgba(0, 0, 180, 0.4)" },
        { gt: 5, lt: 7, color: "rgba(0, 0, 180, 0.4)" }
      ]
    },
    series: [
      {
        type: "line",
        smooth: 0.6,
        symbol: "none",
        data: column(0),
        lineStyle: {
          color: "#5470C6",
          width: 5
        },
        areaStyle:{},
        markLine: {
          symbol: ["none", "none"],
          label: { show: false },
          data: [{ xAxis: 1 }, { xAxis: 3 }, { xAxis: 5 }, { xAxis: 7 }]
        }
      }   
    ]
  })
)
```

**Description**: Area chart with specific segments colored differently using `visualMap.pieces`. MarkLines highlight specific x-axis positions.

---

## 6. Step Line

Line chart with step interpolation.

```js
SCRIPT({
  days    = ["Mon","Tue","Wed","Thu","Fri","Sat","Sun"];
  starts  = [120,132,101,134,90,230,210];
  middles = [220,282,201,234,290,430,410];
  ends    = [450,432,401,454,590,530,510];

  $.yield({
    legend: { show:true },
    grid: [{
      left: "3%",
      right: "4%",
      bottom: "3%",
      containLabel: true
    }],
    xAxis: { type: "category", data: days },
    yAxis: {},
    series: [
      {type: "line", data: starts, step: "start", name: "Step Start"},
      {type: "line", data: middles, step: "middle", name: "Step Middle"},
      {type: "line", data: ends, step: "end", name: "Step End"}
    ]
  })
})
CHART()
```

```js
FAKE( json({
  ["Mon", 120, 220, 450],
  ["Tue", 132, 282, 432],
  ["Wed", 101, 201, 401],
  ["Thu", 134, 234, 454],
  ["Fri", 90,  290, 590],
  ["Sat", 230, 430, 530],
  ["Sun", 210, 410, 510]
}) )
CHART(
  chartOption({
    legend: { show:true },
    grid: [{
      left: "3%",
      right: "4%",
      bottom: "3%",
      containLabel: true
    }],
    xAxis: { type: "category", data: column(0) },
    yAxis: {},
    series: [
      {type: "line", data: column(1), step: "start", name: "Step Start"},
      {type: "line", data: column(2), step: "middle", name: "Step Middle"},
      {type: "line", data: column(3), step: "end", name: "Step End"}
    ]
  })
)
```

**Description**: Step line chart showing three step interpolation modes: start, middle, and end.

---

## 7. Multiple X-Axes

Chart with two x-axes for comparing different time periods.

```js
FAKE(csv(`2015-1,2.6
2015-2,5.9
2015-3,9.0
2015-4,26.4
2015-5,28.7
2015-6,70.7
2015-7,175.6
2015-8,182.2
2015-9,48.7
2015-10,18.8
2015-11,6.0
2015-12,2.3
2016-1,3.9
2016-2,5.9
2016-3,11.1
2016-4,18.7
2016-5,48.3
2016-6,69.2
2016-7,231.6
2016-8,46.6
2016-9,55.4
2016-10,18.4
2016-11,10.3
2016-12,0.7
`))
PUSHVALUE(1, value(0))
// | 0        1         2
// + YYYY-M   YYYY-M    value
// |
MAPVALUE(1, strHasPrefix(value(1), "2015-") ? "2015" : value(1))
MAPVALUE(1, strHasPrefix(value(1), "2016-") ? "2016" : value(1))
// | 0        1         2
// + YYYY-M   YYYY      value
// |
PUSHVALUE(2, strSub(value(0), 5) )
// | 0        1         2         3
// + YYYY-M   YYYY      Month     value
// |
GROUP(
    by(parseFloat(value(2))),
    max(value(3), where(value(1) == "2016")),
    max(value(3), where(value(1) == "2015")),
    lazy(true)
)
// | 0        1              2
// + Month    2015-value     2016-value
// |
MAPVALUE(1, list(strSprintf("2015-%.f",value(0)), value(1)))
MAPVALUE(2, list(strSprintf("2016-%.f",value(0)), value(2)))
// | 0        1                         2
// + Month    ["2015-M", 2015-value]    ["2015-M", 2016-value]
// |
CHART(
    chartJSCode({
        function colors() {
            return ['#5470C6', '#EE6666'];
        }
        function labelformat (params) {
            return (
                'Precipitation  ' +
                params.value +
                (params.seriesData.length ? '：' + params.seriesData[0].data : '')
            );
        }
    }),
    chartOption({
        color: colors(),
        tooltip: {
            trigger: "none",
            axisPointer: {
                type: "cross"
            }
        },
        legend: {},
        grid: {
            top: 70,
            bottom: 50
        },
        xAxis: [
            {
                type: "category",
                axisTick: {
                    alignWithLabel: true
                },
                axisLine: {
                    onZero: false,
                    lineStyle: {
                        color: colors()[1]
                    }
                },
                axisPointer: {
                    label: {
                        formatter: labelformat
                    }
                }
            },
            {
                type: "category",
                axisTick: {
                    alignWithLabel: true
                },
                axisLine: {
                    onZero: false,
                    lineStyle: {
                        color: colors()[0]
                    }
                },
                axisPointer: {
                    label: {
                        formatter: labelformat
                    }
                }
            }
        ],
        yAxis: [
            { type: "value" }
        ],
        series: [
            {
                name: "Precipitation(2015)",
                type: "line",
                xAxisIndex: 1,
                smooth: true,
                emphasis: {
                    focus: "series"
                },
                data: column(1)
            },
            {
                name: "Precipitation(2016)",
                type: "line",
                xAxisIndex: 0,
                smooth: true,
                emphasis: {
                    focus: "series"
                },
                data: column(2)
            }
        ]
    })
)
```

**Description**: Dual x-axes comparing 2015 vs 2016 monthly data. Each series uses a different x-axis index.

---

## 8. Multiple Y-Axes

Chart with three y-axes for different measurement units.

```js
FAKE(json({
    ["Month", "Evaporation", "Precipitation", "Temperature"],
    ["Jan", 2.0,   2.6,   2.0],
    ["Feb", 4.9,   5.9,   2.2],
    ["Mar", 7.0,   9.0,   3.3],        
    ["Apr", 23.2,  26.4,  4.5],         
    ["May", 25.6,  28.7,  6.3],         
    ["Jun", 76.7,  70.7,  10.2],        
    ["Jul", 135.6, 175.6, 20.3],         
    ["Aug", 162.2, 182.2, 23.4],         
    ["Sep", 32.6,  48.7,  23.0],         
    ["Oct", 20.0,  18.8,  16.5],         
    ["Nov", 6.4,   6.0,   12.0],        
    ["Dec", 3.3,   2.3,   6.2]  
}))

CHART(
  chartJSCode({
    const colors = ['#5470C6', '#91CC75', '#EE6666'];
  }),
  chartOption({
    color: colors,
    tooltip: {
      trigger: "axis",
      axisPointer: {
        type: "cross"
      }
    },
    grid: { right: "23%" },
    toolbox: {
      feature: {
        dataView: { show: true, readOnly: false },
        restore: { show: true },
        saveAsImage: { show: true }
      }
    },
    legend: { bottom: 10, data: [ column(1)[0], column(2)[0], column(3)[0]] },
    xAxis: [
        {
          type: "category",
          axisTick: {
            alignWithLabel: true
          },
          data: column(0).slice(1)
        }
      ],
    yAxis: [
      {
        type: "value",
        name: "Evaporation",
        position: "right",
        alignTicks: true,
        axisLine: { show: true, lineStyle: { color: colors[0] } },
        axisLabel: { formatter: "{value} ml" }
      },
      {
        type: "value",
        name: "Precipitation",
        position: "right",
        alignTicks: true,
        offset: 80,
        axisLine: { show: true, lineStyle: { color: colors[1] } },
        axisLabel: { formatter: "{value} ml" }
      },
      {
        type: "value",
        name: "Temperature",
        position: "left",
        alignTicks: true,
        axisLine: { show: true, lineStyle: { color: colors[2] } },
        axisLabel: { formatter: "{value} °C" }
      }
    ],
    series: [
      {
        name: "Evaporation",
        type: "bar",
        data: column(1).slice(1)
      },
      {
        name: "Precipitation",
        type: "bar",
        yAxisIndex: 1,
        data: column(2).slice(1)
      },
      {
        name: "Temperature",
        type: "line",
        yAxisIndex: 2,
        data: column(3).slice(1)
      }
    ]
  })
)
```

**Description**: Combines bar and line charts with three y-axes for different units (ml and °C). Each series references a specific y-axis via `yAxisIndex`.

---

## 9. Basic Mix (Line and Bar)

Combined line and bar chart.

```js
FAKE( linspace(0, 360, 50))
MAPVALUE(1, sin((value(0)/180)*PI))
MAPVALUE(2, cos((value(0)/180)*PI))
CHART(
    chartOption({
        xAxis: { data: column(0) },
        yAxis: {},
        series: [
            {type: "bar", name: "SIN", data: column(1)},
            {type: "line", name: "COS", color:"#093", data: column(2)}
        ]
    })
)
```

**Description**: Mix of bar and line series showing sine and cosine waves.

---

## 10. Large Area Chart

Efficiently render 20,000 data points using LTTB algorithm.

```js
FAKE(linspace(0,19999,20000))
// |   0
// +-- n
// |         -42109200000000000 = epoch "1968/09/01" and add a day
PUSHVALUE(0, -42109200000000000 + value(0)*3600*24*1000000000)
// |   0              1
// +-- daily-epoch    n
// |         convert from epoch to time
MAPVALUE(0, time(value(0)))
// |   0         1
// +-- time      n
// |         convert time to date string
MAPVALUE(0, strTime(value(0), sqlTimeformat("YYYY/MM/DD"), tz("Local")))
// |   0         1
// +-- date      n
// |         random values
MAPVALUE(1, sin(value(1)/20000 * 3*PI) * 300 + (100*random())+50)
// |   0         1
// +-- date      value
// |   
CHART(
    chartJSCode({
        function position(pt) {
            return [pt[0], '10%'];
        }
        function areaColor() {
            return new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            {
                offset: 0,
                color: 'rgb(255, 158, 68)'
            },
            {
                offset: 1,
                color: 'rgb(255, 70, 131)'
            }
            ]);
        }
    }),
    chartOption({
        tooltip: {
            trigger: "axis",
            position: position
        },
        title: {
            left: "center",
            text: "Large Area Chart"
        },
        toolbox: {
            feature: {
                dataZoom: {
                    yAxisIndex: "none"
                },
                restore: {},
                saveAsImage: {}
            }
        },
        xAxis: {
            type: "category",
            boundaryGap: false,
            data: column(0)
        },
        yAxis: {
            type: "value",
            boundaryGap: [0, "100%"]
        },
        dataZoom: [
            {
                type: "inside",
                start: 0,
                end: 10
            },
            {
                start: 0,
                end: 10
            }
        ],
        series: [
            {
                name: "Fake Data",
                type: "line",
                symbol: "none",
                sampling: "lttb",
                itemStyle: {
                    color: "rgb(255, 70, 131)"
                },
                areaStyle: {
                    color: areaColor()
                },
                data: column(1)
            }
        ]
    })
)
```

**Description**: Large dataset (20K points) efficiently rendered using `sampling: "lttb"` (Largest-Triangle-Three-Buckets algorithm). DataZoom enables exploration.

---

## 11. Data Transform

Transform and filter external CSV data.

```js
CSV( file("https://docs.machbase.com/assets/example/life-expectancy-table.csv") )
DROP(1) // skip header line
// |   0          1                 2            3         4
// +-> income     life expectancy   population   Country   Year
// |
POPVALUE(1,2)
// |   0          1         2
// +-> income     Country   Year
// |
FILTER( value(1) in ("Germany", "France") )
// |   0          1         2
// +-> income     Country   Year
// |
MAPVALUE(0, parseFloat(value(0)))
// |   0          1         2
// +-> income     Country   Year
// |
GROUP(
    by(value(2)),
    max(value(0), where( value(1) == "Germany" )),
    max(value(0), where( value(1) == "France" ))
)
// |   0      1               2
// +-> Year   Germany-income  France-income
// |
CHART(
    chartOption({
        xAxis: { name: "Year", type: "category", data: column(0) },
        yAxis: { name: "Income"},
        legend: { show: true },
        tooltip: {
            trigger: "axis",
            formatter:"{b}<br/> {a0}:{c0}<br/> {a1}:{c1}"
        },
        series: [
            {
                type: "line",
                name: "Germany",
                showSymbol: false,
                data: column(1),
                tooltip: ["income"]
            },
            {
                type: "line",
                name: "France",
                showSymbol: false,
                data: column(2),
                tooltip: ["income"]
            }
        ]
    })
)
```

**Description**: Load external CSV, filter for Germany and France, group by year, and compare income trends.

---

## 12. Air Passengers

Process time-series data from external CSV.

```js
CSV (file("https://docs.machbase.com/assets/example/AirPassengers.csv"))

// drop header : rownames,time,value
DROP(1) 
// drop rownames column
POPVALUE(0)

// year float to "year/month"
MAPVALUE(0,
  strSprintf("%.f/%.f",
    floor(parseFloat(value(0))),
    1+round(12 * (mod(round(parseFloat(value(0))*100), 100)/100)) 
  )
)
// passengers
MAPVALUE(1, parseFloat(value(1)))

CHART(
  chartOption({
    xAxis: { data: column(0) },
    yAxis: {},
    series: [
        {type: "line", name: "passengers", smooth: false, data: column(1)}
    ]
  })
)
```

**Description**: Convert decimal year format (e.g., 1949.08) to "YYYY/M" format and plot air passenger data.

---

## 13. Cartesian Coordinate System

Line chart using [x, y] coordinate pairs.

```js
FAKE(json({
    [10, 40],
    [50, 100],
    [40, 20]
}))
MAPVALUE(0, list(value(0), value(1)))
POPVALUE(1)
CHART(
    chartOption({
        title: { text: "Line Chart in Cartesian Coordinate System"},
        xAxis: {},
        yAxis: {},
        series: [
            { type: "line", data: column(0)}
        ]
    })
)
```

**Description**: Simple line chart using [x, y] coordinate pairs instead of separate x-axis categories.