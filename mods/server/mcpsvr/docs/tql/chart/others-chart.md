# Machbase Neo Other Charts

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

**plugins()**
- `plugins(plugin...)`
- `plugin` *string* Pre-defined plugin name or URL of plugin module

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

## 1. Sankey Diagram

Flow diagram showing connections and quantities between nodes.

```js
FAKE(csv(
`a,a1,5
a,a2,3
b,b1,8
a,b1,3
b1,a1,1
b1,c,2
`))
SCRIPT({
    data = [];
},{
    data.push({source: $.values[0], target: $.values[1], value: $.values[2]})
},{
    $.yield({
        series: {
            type: "sankey",
            layout: "none",
            emphasis: {
                focus: "adjacency"
            },
            links: data,
            data: [
                {name: "a"}, {name: "b"}, {name: "a1"}, {name: "a2"}, {name: "b1"}, {name: "c"}
            ]
        }
    })
})
CHART()
```

**Description**: Sankey diagram visualizing flows between nodes. Width of connecting lines represents flow quantity.

**Key Points**:

**Data Format**:
- CSV: `source,target,value`
- Converted to objects: `{source: "a", target: "a1", value: 5}`
- Links array defines all connections

**Node Definition**:
- `data: [{name: "a"}, {name: "b"}, ...]` defines all nodes
- Must include all sources and targets
- Node positions auto-calculated

**Configuration**:
- `type: "sankey"` creates Sankey diagram
- `layout: "none"` uses default layout algorithm
- `emphasis.focus: "adjacency"` highlights connected nodes on hover
- `links` contains connection data
- `value` determines link width

**Use Cases**:
- Energy flow diagrams
- Material flow analysis
- Budget allocation
- User journey visualization
- Network traffic analysis

---

## 2. Word Cloud

Visual representation of word frequency with size-based encoding.

### From External Text File

```js
SCRIPT({
    counter = {};
    data = [];
},{
    const http = require("@jsh/http");
    req = http.request("https://docs.machbase.com/assets/example/wordcount.txt")
    req.do((rsp) => {
        content = rsp.text();
        words = content.split(/\s+/)
        for(w of words) {
            w = w.toLowerCase();
            if(counter[w]) {
                counter[w].count++;
            } else {
                counter[w] = {count:1}
            }
        }
    })
},{
    Object.keys(counter).forEach(w =>{
        data.push({name: w, value: counter[w].count})
    })
    $.yield({
        series: {
            type: "wordCloud",
            gridSize: 4,
            sizeRange: [12, 50],
            rotationRange: [-90, 90],
            shape: "circle",
            width: 580,
            height: 580,
            drawOutOfBound: false,
            left: "center",
            top: "center",
            data: data,
            emphasis: {
                focus: "self",
                textStyle: {
                    textShadowBlur: 10,
                    textShadowColor: "#333"
                }
            },
            layoutAnimation: true,
            textStyle: {
                fontFamily: "sans-serif",
                fontWeight: "bold",
            }
        }
    })
})
CHART(
    plugins("wordcloud"),
    chartOption({}),
    chartJSCode({
        _chartOption.series.textStyle.color = function() {
            let r = Math.round(Math.random() * 160);
            let g = Math.round(Math.random() * 160);
            let b = Math.round(Math.random() * 160);
            return `rgb(${r},${g},${b})`;
        }
        _chart.setOption(_chartOption);
    })
)
```

### From CSV Data

```js
FAKE(csv(
`Deep Learning,6181
Computer Vision,4386
Artificial Intelligence,4055
Neural Network,3500
Algorithm,3333
Model,2700
Supervised,2500
Unsupervised,2333
Natural Language Processing,1900
Chatbot,1800
Virtual Assistant,1500
Speech Recognition,1400
Convolutional Neural Network,1325
Reinforcement Learning,1300
Training Data,1250
Classification,1233
Regression,1000
Decision Tree,900
K-Means,875
N-Gram Analysis,850
Microservices,833
Pattern Recognition,790
APIs,775
Feature Engineering,700
Random Forest,650
Bagging,600
Anomaly Detection,575
Naive Bayes,500
Autoencoder,400
Backpropagation,300
TensorFlow,290
word2vec,280
Object Recognition,250
Python,235
Predictive Analytics,225
Predictive Modeling,215
Optical Character Recognition,200
Overfitting,190
JavaScript,185
Text Analytics,180
Cognitive Computing,175
Augmented Intelligence,160
Statistical Models,155
Clustering,150
Topic Modeling,145
Data Mining,140
Data Science,138
Semi-Supervised Learning,137
Artificial Neural Networks,125
`))
SCRIPT({
    data = [];
},{
    data.push({name: $.values[0], value: $.values[1]})
},{
    $.yield({
        series: {
            type: "wordCloud",
            gridSize: 8,
            sizeRange: [12, 50],
            rotationRange: [-90, 90],
            shape: "circle",
            width: 580,
            height: 580,
            drawOutOfBound: false,
            left: "center",
            top: "center",
            data: data,
            emphasis: {
                focus: "self",
                textStyle: {
                    textShadowBlur: 10,
                    textShadowColor: "#333"
                }
            },
            layoutAnimation: true,
            textStyle: {
                fontFamily: "sans-serif",
                fontWeight: "bold",
            }
        }
    })
})
CHART(
    plugins("wordcloud"),
    chartOption({}),
    chartJSCode({
        _chartOption.series.textStyle.color = function() {
            return 'rgb(' + [
                Math.round(Math.random() * 160),
                Math.round(Math.random() * 160),
                Math.round(Math.random() * 160)
            ].join(',') + ')';
        }
        _chart.setOption(_chartOption);
    })
)
```

**Description**: Word cloud visualization where word size represents frequency. Two approaches: counting words from text file, or using pre-counted CSV data.

**Key Points**:

**Data Preparation**:
- **Method 1**: Fetch text, split by whitespace, count occurrences
- **Method 2**: Load pre-counted data from CSV
- Result format: `{name: "word", value: count}`

**Configuration**:
- `plugins("wordcloud")` loads word cloud plugin
- `type: "wordCloud"` creates word cloud
- `gridSize: 4-8` spacing between words (smaller = tighter)
- `sizeRange: [12, 50]` min/max font sizes
- `rotationRange: [-90, 90]` word rotation angles
- `shape: "circle"` cloud shape (also: "square", "diamond", "pentagon")

**Layout**:
- `width: 580, height: 580` canvas size
- `left: "center", top: "center"` positioning
- `drawOutOfBound: false` prevents words outside boundary
- `layoutAnimation: true` animates word placement

**Styling**:
- Random RGB colors via `chartJSCode`
- Colors limited to 0-160 (darker colors)
- Bold sans-serif font
- Shadow effect on hover

**Use Cases**:
- Text analysis
- Tag clouds
- Survey responses
- Social media trends
- Document summarization

---

## 3. GEO SVG Lines

Animated path on custom SVG map.

```js
FAKE(json({
        [110.6189462165178, 456.64349563895087],
        [124.10988522879458, 450.8570048730469],
        [123.9272226116071, 389.9520693708147],
        [61.58708083147317, 386.87942320312504],
        [61.58708083147317, 72.8954315876116],
        [258.29514854771196, 72.8954315876116],
        [260.75457021484374, 336.8559607533482],
        [280.5277985253906, 410.2406672084263],
        [275.948185765904, 528.0254369698661],
        [111.06907909458701, 552.795792593471],
        [118.87138231445309, 701.365737015904],
        [221.36468155133926, 758.7870354617745],
        [307.86195445452006, 742.164737297712],
        [366.8489324762834, 560.9895157073103],
        [492.8750778390066, 560.9895157073103],
        [492.8750778390066, 827.9639780566406],
        [294.9255269587053, 827.9639780566406],
        [282.79803391043527, 868.2476088113839]
}))
// +-- [ x, y ]
// |
MAPVALUE(0, list(value(0), value(1))) // make coord pair
// |
// +--> [ (x, y), y ]
// | 
POPVALUE(1)  // remove y
// | 
// +--> [ (x, y) ]
CHART(
    chartJSCode({
        fetch("https://docs.machbase.com/assets/example/MacOdrum-LV5-floorplan-web.svg"
        ).then( function(rsp) {
            return rsp.text();
        }).then( function(svg) {
            // 'echarts' has been imported in TQL
            echarts.registerMap("MacOdrum-LV5-floorplan-web", {svg: svg});
            // 'chart' is defined by CHART() in TQL
            let opt = _chart.getOption()
            opt.geo = {
                map: "MacOdrum-LV5-floorplan-web",
                roam: true,
                emphasis: {
                    itemStyle: {
                        color: undefined
                    },
                    label: {
                        show: false
                    }
                }
            };
            _chart.setOption(opt);
        }).catch(function(err){
            console.warn("geomap error, fetch resource", err)
        });
    }),
    chartOption({
        series: [
            {
                type: "lines",
                coordinateSystem: "geo",
                geoIndex: 0,
                polyline: true,
                lineStyle: {
                    color: "#c46e54",
                    width: 5,
                    opacity: 1,
                    type: "dotted"
                },
                effect: {
                    show: true,
                    period: 8,
                    color: "#a10000",
                    constantSpeed: 80,
                    trailLength: 0,
                    symbolSize: [20, 12],
                    symbol: "path://M35.5 40.5c0-22.16 17.84-40 40-40s40 17.84 40 40c0 1.6939-.1042 3.3626-.3067 5H35.8067c-.2025-1.6374-.3067-3.3061-.3067-5zm90.9621-2.6663c-.62-1.4856-.9621-3.1182-.9621-4.8337 0-6.925 5.575-12.5 12.5-12.5s12.5 5.575 12.5 12.5a12.685 12.685 0 0 1-.1529 1.9691l.9537.5506-15.6454 27.0986-.1554-.0897V65.5h-28.7285c-7.318 9.1548-18.587 15-31.2715 15s-23.9535-5.8452-31.2715-15H15.5v-2.8059l-.0937.0437-8.8727-19.0274C2.912 41.5258.5 37.5549.5 33c0-6.925 5.575-12.5 12.5-12.5S25.5 26.075 25.5 33c0 .9035-.0949 1.784-.2753 2.6321L29.8262 45.5h92.2098z"
                },
                data: [ {coords: column(0) }] 
            }
        ]
    })
)
```

**Description**: Animated path visualization on custom SVG floorplan. Shows moving vehicle icon following a dotted route.

**Key Points**:

**Data Preparation**:
- Array of `[x, y]` coordinates
- `list(value(0), value(1))` creates coordinate pairs
- Final format: `[[x1, y1], [x2, y2], ...]`

**SVG Map Loading**:
- `fetch()` loads external SVG file
- `echarts.registerMap()` registers SVG as map
- `coordinateSystem: "geo"` uses registered map
- `roam: true` enables pan/zoom

**Line Configuration**:
- `type: "lines"` draws polyline
- `polyline: true` connects all points in sequence
- `lineStyle`: dotted brown line, 5px width
- Coordinates map to SVG coordinate system

**Animation Effect**:
- `effect.show: true` enables moving symbol
- `period: 8` animation cycle duration (seconds)
- `constantSpeed: 80` movement speed
- `trailLength: 0` no trail behind symbol
- `symbol: "path://..."` custom SVG vehicle icon
- Red vehicle (#a10000) moves along brown path

**Use Cases**:
- Indoor navigation
- Robot path visualization
- Tour routes
- Evacuation plans
- Asset tracking on floorplans
- Custom map overlays

**Technical Details**:
- SVG coordinates must match data coordinates
- Custom SVG shapes via path data
- Works with any SVG file (maps, floorplans, diagrams)
- Interactive zoom/pan with `roam: true`