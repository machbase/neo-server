# Machbase Neo Candlestick Chart

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
| `MAP_MOVAVG()` | Calculate moving average | `MAP_MOVAVG(6, value(2), 5, "MA5")` |
| `MAP_DIFF()` | Calculate difference | `MAP_DIFF(7, value(2))` |

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

**chartDispatchAction()**
- `chartDispatchAction({ type, ...params })`
- Trigger chart actions like zoom, highlight, brush, etc.

---

### Candlestick Data Format

Candlestick charts require data in the format: `[open, close, lowest, highest]`

Example:
```js
[20, 34, 10, 38]  // open=20, close=34, lowest=10, highest=38
```

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

## 1. Basic Candlestick Chart

Simple candlestick chart showing basic OHLC (Open-High-Low-Close) data.

### Using SCRIPT

```js
SCRIPT({
  dates = [ "2017-10-24", "2017-10-25", "2017-10-26", "2017-10-27"];
  values = [ [20, 34, 10, 38], [40, 35, 30, 50],
             [31, 38, 33, 44], [38, 15,  5, 42] ];
  $.yield({
    legend: { show: false },
    xAxis: { data: dates },
    yAxis: {},
    series: [ { type: "candlestick", data: values } ]
  })
})

CHART()
```

### Using FAKE

```js
FAKE(json({
    ["2017-10-24", 20, 34, 10, 38 ], 
    ["2017-10-25", 40, 35, 30, 50 ],
    ["2017-10-26", 31, 38, 33, 44 ],
    ["2017-10-27", 38, 15,  5, 42 ]
}))

MAPVALUE(1, list(value(1), value(2), value(3), value(4)))
POPVALUE(2, 3, 4)
CHART(
    chartOption({
        legend: { show: false },
        xAxis: { data: column(0) },
        yAxis: {},
        series: [
            { type: "candlestick", data: column(1) }
        ]
    })
)
```

**Description**: Basic candlestick chart displaying 4 days of stock data. Shows two approaches: using SCRIPT to directly create chart options, or using FAKE with data transformation pipeline.

**Data Format**: Each candlestick requires `[open, close, lowest, highest]` values.

---

## 2. Stock Index with Moving Averages

Candlestick chart with multiple moving average indicators.

```js
//            open     close    lowest  highest
FAKE(json({
  ["2013/1/24", 2320.26, 2320.26, 2287.3, 2362.94],
  ["2013/1/25", 2300, 2291.3, 2288.26, 2308.38],
  ["2013/1/28", 2295.35, 2346.5, 2295.35, 2346.92],
  ["2013/1/29", 2347.22, 2358.98, 2337.35, 2363.8],
  ["2013/1/30", 2360.75, 2382.48, 2347.89, 2383.76],
  ["2013/1/31", 2383.43, 2385.42, 2371.23, 2391.82],
  ["2013/2/1",  2377.41, 2419.02, 2369.57, 2421.15],
  ["2013/2/4",  2425.92, 2428.15, 2417.58, 2440.38],
  ["2013/2/5",  2411, 2433.13, 2403.3, 2437.42],
  ["2013/2/6",  2432.68, 2434.48, 2427.7, 2441.73],
  ["2013/2/7",  2430.69, 2418.53, 2394.22, 2433.89],
  ["2013/2/8",  2416.62, 2432.4, 2414.4, 2443.03],
  ["2013/2/18", 2441.91, 2421.56, 2415.43, 2444.8],
  ["2013/2/19", 2420.26, 2382.91, 2373.53, 2427.07],
  ["2013/2/20", 2383.49, 2397.18, 2370.61, 2397.94],
  ["2013/2/21", 2378.82, 2325.95, 2309.17, 2378.82],
  ["2013/2/22", 2322.94, 2314.16, 2308.76, 2330.88],
  ["2013/2/25", 2320.62, 2325.82, 2315.01, 2338.78],
  ["2013/2/26", 2313.74, 2293.34, 2289.89, 2340.71],
  ["2013/2/27", 2297.77, 2313.22, 2292.03, 2324.63],
  ["2013/2/28", 2322.32, 2365.59, 2308.92, 2366.16],
  ["2013/3/1",  2364.54, 2359.51, 2330.86, 2369.65],
  ["2013/3/4",  2332.08, 2273.4, 2259.25, 2333.54],
  ["2013/3/5",  2274.81, 2326.31, 2270.1, 2328.14],
  ["2013/3/6",  2333.61, 2347.18, 2321.6, 2351.44],
  ["2013/3/7",  2340.44, 2324.29, 2304.27, 2352.02],
  ["2013/3/8",  2326.42, 2318.61, 2314.59, 2333.67],
  ["2013/3/11", 2314.68, 2310.59, 2296.58, 2320.96],
  ["2013/3/12", 2309.16, 2286.6, 2264.83, 2333.29],
  ["2013/3/13", 2282.17, 2263.97, 2253.25, 2286.33],
  ["2013/3/14", 2255.77, 2270.28, 2253.31, 2276.22],
  ["2013/3/15", 2269.31, 2278.4, 2250, 2312.08],
  ["2013/3/18", 2267.29, 2240.02, 2239.21, 2276.05],
  ["2013/3/19", 2244.26, 2257.43, 2232.02, 2261.31],
  ["2013/3/20", 2257.74, 2317.37, 2257.42, 2317.86],
  ["2013/3/21", 2318.21, 2324.24, 2311.6, 2330.81],
  ["2013/3/22", 2321.4, 2328.28, 2314.97, 2332],
  ["2013/3/25", 2334.74, 2326.72, 2319.91, 2344.89],
  ["2013/3/26", 2318.58, 2297.67, 2281.12, 2319.99],
  ["2013/3/27", 2299.38, 2301.26, 2289, 2323.48],
  ["2013/3/28", 2273.55, 2236.3, 2232.91, 2273.55],
  ["2013/3/29", 2238.49, 2236.62, 2228.81, 2246.87],
  ["2013/4/1",  2229.46, 2234.4, 2227.31, 2243.95],
  ["2013/4/2",  2234.9, 2227.74, 2220.44, 2253.42],
  ["2013/4/3",  2232.69, 2225.29, 2217.25, 2241.34],
  ["2013/4/8",  2196.24, 2211.59, 2180.67, 2212.59],
  ["2013/4/9",  2215.47, 2225.77, 2215.47, 2234.73],
  ["2013/4/10", 2224.93, 2226.13, 2212.56, 2233.04],
  ["2013/4/11", 2236.98, 2219.55, 2217.26, 2242.48],
  ["2013/4/12", 2218.09, 2206.78, 2204.44, 2226.26],
  ["2013/4/15", 2199.91, 2181.94, 2177.39, 2204.99],
  ["2013/4/16", 2169.63, 2194.85, 2165.78, 2196.43],
  ["2013/4/17", 2195.03, 2193.8, 2178.47, 2197.51],
  ["2013/4/18", 2181.82, 2197.6, 2175.44, 2206.03],
  ["2013/4/19", 2201.12, 2244.64, 2200.58, 2250.11],
  ["2013/4/22", 2236.4, 2242.17, 2232.26, 2245.12],
  ["2013/4/23", 2242.62, 2184.54, 2182.81, 2242.62],
  ["2013/4/24", 2187.35, 2218.32, 2184.11, 2226.12],
  ["2013/4/25", 2213.19, 2199.31, 2191.85, 2224.63],
  ["2013/4/26", 2203.89, 2177.91, 2173.86, 2210.58],
  ["2013/5/2",  2170.78, 2174.12, 2161.14, 2179.65],
  ["2013/5/3",  2179.05, 2205.5, 2179.05, 2222.81],
  ["2013/5/6",  2212.5, 2231.17, 2212.5, 2236.07],
  ["2013/5/7",  2227.86, 2235.57, 2219.44, 2240.26],
  ["2013/5/8",  2242.39, 2246.3, 2235.42, 2255.21],
  ["2013/5/9",  2246.96, 2232.97, 2221.38, 2247.86],
  ["2013/5/10", 2228.82, 2246.83, 2225.81, 2247.67],
  ["2013/5/13", 2247.68, 2241.92, 2231.36, 2250.85],
  ["2013/5/14", 2238.9, 2217.01, 2205.87, 2239.93],
  ["2013/5/15", 2217.09, 2224.8, 2213.58, 2225.19],
  ["2013/5/16", 2221.34, 2251.81, 2210.77, 2252.87],
  ["2013/5/17", 2249.81, 2282.87, 2248.41, 2288.09],
  ["2013/5/20", 2286.33, 2299.99, 2281.9, 2309.39],
  ["2013/5/21", 2297.11, 2305.11, 2290.12, 2305.3],
  ["2013/5/22", 2303.75, 2302.4, 2292.43, 2314.18],
  ["2013/5/23", 2293.81, 2275.67, 2274.1, 2304.95],
  ["2013/5/24", 2281.45, 2288.53, 2270.25, 2292.59],
  ["2013/5/27", 2286.66, 2293.08, 2283.94, 2301.7],
  ["2013/5/28", 2293.4, 2321.32, 2281.47, 2322.1],
  ["2013/5/29", 2323.54, 2324.02, 2321.17, 2334.33],
  ["2013/5/30", 2316.25, 2317.75, 2310.49, 2325.72],
  ["2013/5/31", 2320.74, 2300.59, 2299.37, 2325.53],
  ["2013/6/3",  2300.21, 2299.25, 2294.11, 2313.43],
  ["2013/6/4",  2297.1, 2272.42, 2264.76, 2297.1],
  ["2013/6/5",  2270.71, 2270.93, 2260.87, 2276.86],
  ["2013/6/6",  2264.43, 2242.11, 2240.07, 2266.69],
  ["2013/6/7",  2242.26, 2210.9, 2205.07, 2250.63],
  ["2013/6/13", 2190.1, 2148.35, 2126.22, 2190.1]
}))

//  date          open      close     lowest    highest
MAPVALUE(1, list(value(1), value(2), value(3), value(4)), "data")
MAP_MOVAVG(5, value(2), 5, "MA5")
MAP_MOVAVG(6, value(2), 10, "MA10")
MAP_MOVAVG(7, value(2), 20, "MA20")
MAP_MOVAVG(8, value(2), 30, "MA30")
POPVALUE(2, 3, 4)

CHART(
    chartOption({
        xAxis: { type: "category", data: column(0) },
        yAxis: { min: 2100, max: 2500 },
        dataZoom: [
            { type: "inside", start: 50, end: 100 },
            { show: true, type: "slider", top: "90%", start: 50, end: 100 }
        ],
        toolbox: {
            feature: {
                saveAsImage: { show: true, title: "save as image", name: "stock" }
            }
        },
        series: [
            {
                name: "日K",
                data: column(1),
                type: "candlestick",
                itemStyle: {
                    color: "#ec0000",
                    color0: "#00da3c",
                    borderColor: "#8A0000",
                    borderColor0: "#008F28"
                }
            },
            {
                name: "MA5",
                data: column(2),
                type: "line",
                smooth: true,
                lineStyle: { opacity: 0.5 }
            },
            {
                name: "MA10",
                data: column(3),
                type: "line",
                smooth: true,
                lineStyle: { opacity: 0.5 }
            },
            {
                name: "MA20",
                data: column(4),
                type: "line",
                smooth: true,
                lineStyle: { opacity: 0.5 }
            },
            {
                name: "MA30",
                data: column(5),
                type: "line",
                smooth: true,
                lineStyle: { opacity: 0.5 }
            }
        ]
    })
)
```

**Description**: Stock index candlestick chart with 4 moving average indicators (MA5, MA10, MA20, MA30). Features dataZoom for pan/zoom functionality and custom color styling for bullish (green) and bearish (red) candles.

**Key Points**:
- `MAP_MOVAVG()` calculates moving averages with different windows
- `dataZoom` enables interactive zoom with slider control
- Custom colors distinguish rising vs falling price movements
- Multiple line series overlay the candlestick for trend analysis

---

## 3. Dow-Jones Index with Volume

Advanced candlestick chart with volume bar chart and technical indicators.

```js
CSV( file("https://docs.machbase.com/assets/example/stock-DJI.csv") )
DROP(1) // drop header
MAPVALUE(0, value(0), "date")
MAPVALUE(1, parseFloat(value(1)), "open")
MAPVALUE(2, parseFloat(value(2)), "close")
MAPVALUE(3, parseFloat(value(3)), "lowest")
MAPVALUE(4, parseFloat(value(4)), "highest")
MAPVALUE(5, parseFloat(value(5)), "volume")

MAP_MOVAVG(6, value(2), 5, "MA5")
MAP_MOVAVG(7, value(2), 10, "MA10")
MAP_MOVAVG(8, value(2), 20, "MA20")
MAP_MOVAVG(9, value(2), 30, "MA30")

//make data shape 
MAPVALUE(1, list(value(1), value(2), value(3), value(4)), "DJI")
POPVALUE(2,3,4)
//  |    0    1                            2      3   4    5    6
//  +-> date [open,close,lowest,highest]  volume MA5 MA10 MA20 MA30

MAP_DIFF(7, value(2))
//  |    0    1                            2      3   4    5    6     7
//  +-> date [open,close,lowest,highest]  volume MA5 MA10 MA20 MA30  volumeDiff

MAPVALUE(7, value(7) == NULL || value(7) > 0 ? 1 : -1)
//  |    0    1                            2      3   4    5    6     7
//  +-> date [open,close,lowest,highest]  volume MA5 MA10 MA20 MA30  (1 or -1)

MAPVALUE(2, list(value(0), value(2), value(7)))
POPVALUE(7)
//  |    0    1                            2                        3   4    5    6    
//  +-> date [open,close,lowest,highest]  [date, volume, (1 or -1)] MA5 MA10 MA20 MA30

// chart
CHART(
    chartJSCode({
        function tooltipPosition(pos, params, el, elRect, size) {
            const obj = {
                top: 10
            };
            obj[['left', 'right'][+(pos[0] < size.viewSize[0] / 2)]] = 30;
            return obj;
        }
    }),
    chartOption({
        animation: false,
        legend: {
            bottom: 10,
            left: "center",
            data:["Dow-Jones Index", "MA5", "MA10", "MA20", "MA30"]
        },
        tooltip: {
            trigger: "axis",
            axisPointer: { "type": "cross" },
            borderWidth: 1,
            borderColor: "#ccc",
            padding: 10,
            textStyle: {
                color: "#000"
            },
            backgroundColor: "#ffff"
            // ,"position": tooltipPosition
        },
        axisPointer: {
            link: [
                { xAxisIndex: "all"}
            ],
            label: {
                backgroundColor: "#aaa"
            }
        },
        toolbox: {
            feature: {
                saveAsImage: { show: true, title: "save as image", name: "dji" },
                dataZoom: {
                    yAxisIndex: false,
                    title: { zoom: "zoom", back: "restore"}
                },
                brush: {
                    type: ["lineX", "clear"],
                    title: { lineX: "Horizontal selection", clear: "Clear selection"}
                }
            }
        },
        brush: {
            xAxisIndex: "all",
            brushLink: "all",
            outOfBrush: {
                colorAlpha: 0.1
            }
        },
        visualMap: {
            show: false,
            seriesIndex: 1,
            dimension: 2,
            pieces: [
                { value: 1, color: "#ec0000" }, 
                { value: -1, color: "#00da3c" }
            ]
        },
        grid: [
            {
                left: "10%",
                right: "8%",
                height: "50%"
            },
            {
                left: "10%",
                right: "8%",
                top: "65%",
                height: "16%"
            }
        ],
        dataZoom: [
            {
                type: "inside",
                xAxisIndex: [0, 1],
                start: 98,
                end: 100
            },
            {
                show: true,
                type: "slider",
                xAxisIndex: [0, 1],
                top: "85%",
                start: 98,
                end: 100
            }
        ],
        xAxis: [
            {
                type: "category",
                data: column(0),
                boundaryGap: false,
                axisLine: {onZero: false},
                splitLine: {show: false},
                min: "dataMin",
                max: "dataMax",
                axisPointer: {
                    z: 100
                }
            },
            {
                type: "category",
                gridIndex: 1,
                data: column(0),
                boundaryGap: false,
                axisLine: { onZero: false },
                axisTick: { show: false },
                splitLine: { show: false },
                axisLabel: { show: false},
                min: "dataMin",
                max: "dataMax",
                axisPointer: {
                    z: 100
                }
            }
        ],
        yAxis: [
            {
                scale: true,
                splitArea: {
                    show: true
                }
            },
            {
                scale: true,
                gridIndex: 1,
                splitNumber: 2,
                axisLabel: { show: false },
                axisLine: { show: false },
                axisTick: { show: false },
                splitLine: { show: false }
            }
        ],
        series: [
            {
                name: "Dow-Jones Index",
                type: "candlestick",
                data: column(1),
                smooth: true,
                itemStyle: {
                    color: "#00da3c",
                    color0: "#ec0000",
                    borderColor: "#00da3c",
                    borderColor0: "#ec0000"
                }
            },
            {
                name: "Volume",
                type: "bar",
                data: column(2),
                xAxisIndex: 1,
                yAxisIndex: 1
            },
            {
                name: "MA5",
                type: "line",
                data: column(3),
                lineStyle: {
                    opacity: 0.5
                }
            },
            {
                name: "MA10",
                type: "line",
                data: column(4),
                lineStyle: {
                    opacity: 0.5
                }
            },
            {
                name: "MA20",
                type: "line",
                data: column(5),
                lineStyle: {
                    opacity: 0.5
                }
            },
            {
                name: "MA30",
                type: "line",
                data: column(6),
                lineStyle: {
                    opacity: 0.5
                }
            }
        ]
    }),
    chartDispatchAction({
        type: "brush",
        areas:[
            {
                brushType: "lineX",
                coordRange: ["2016-06-02", "2016-06-20"],
                xAxisIndex: 0
            }
        ]
    })
)
```

**Description**: Professional stock chart combining candlestick price data with volume bars in a separate grid. Loads Dow-Jones Index data from external CSV file and displays multiple technical indicators with advanced interactive features.

**Key Points**:
- Dual grid layout: top for price, bottom for volume
- `MAP_DIFF()` calculates volume change direction
- `visualMap` colors volume bars based on price movement direction
- Brush tool for selecting specific time ranges
- `chartDispatchAction()` pre-selects a date range on load
- Linked zoom/pan across both grids
- Custom tooltip positioning function