# Machbase Neo Liquidfill Chart

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

**plugins()**
- `plugins(plugin...)`
- `plugin` *string* Pre-defined plugin name or URL of plugin module
- For liquidfill charts, use: `plugins("liquidfill")`

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

### Liquidfill Data Format

Liquidfill charts accept percentage values (0.0 to 1.0):
- Single value: `[0.6]` = 60% filled
- Multiple waves: `[0.6, 0.5, 0.4, 0.3]` = 4 overlapping waves

---

## 1. Basic Liquidfill

Simple liquid fill gauge showing single animated wave.

```js
FAKE(json({
    [0.6]
}))
CHART(
    plugins("liquidfill"),
    chartOption({
        series: [
            { type: "liquidFill", data: column(0) }
        ]
    })
)
```

**Description**: Basic liquid fill gauge displaying 60% fill level with single animated wave. The wave oscillates smoothly to create realistic water effect.

**Key Points**:
- `plugins("liquidfill")` loads the liquidfill plugin
- `type: "liquidFill"` creates liquid fill chart
- Single value `[0.6]` = 60% fill level
- Default animation with wave motion
- Circular container shape
- Blue color gradient by default

**Default Behavior**:
- Wave amplitude: moderate oscillation
- Wave animation: continuous horizontal movement
- Auto-calculated wave period
- Label shows percentage in center

---

## 2. Multiple Waves

Liquid fill with multiple overlapping animated waves.

```js
FAKE(json({
    [0.6, 0.5, 0.4, 0.3]
}))
TRANSPOSE()
CHART(
    plugins("liquidfill"),
    chartOption({
        series: [
            { type: "liquidFill", data: column(0) }
        ]
    })
)
```

**Description**: Liquid fill gauge with 4 overlapping waves at different heights, creating layered water effect with varying transparency.

**Key Points**:
- `TRANSPOSE()` converts row to column format
- Multiple values create multiple waves: `[0.6, 0.5, 0.4, 0.3]`
- Each wave has different:
  - Fill level (60%, 50%, 40%, 30%)
  - Animation phase (offset timing)
  - Opacity (automatic gradient)
- Waves animate independently
- Creates realistic multi-layer water effect

**Visual Effect**:
- Top wave (0.6): Most opaque
- Lower waves: Progressively more transparent
- Overlapping creates depth perception
- Different phases prevent synchronization
- More dynamic appearance than single wave

---

## 3. Still Waves (No Animation)

Liquid fill with static waves (no animation).

```js
FAKE(json({
    [0.6, 0.5, 0.4, 0.3]
}))
TRANSPOSE()
CHART(
    plugins("liquidfill"),
    chartOption({
        series: [
            {
                type: "liquidFill",
                data: column(0),
                amplitude: 0,
                waveAnimation: 0
            }
        ]
    })
)
```

**Description**: Liquid fill gauge with multiple static waves. No wave motion or oscillation - purely static visualization.

**Key Points**:
- `amplitude: 0` removes vertical oscillation
- `waveAnimation: 0` disables horizontal wave movement
- Waves remain completely still
- Multiple fill levels still visible
- Useful for static reports or screenshots

**Use Cases**:
- **Animated (default)**: Dynamic dashboards, real-time monitoring
- **Still (amplitude/animation = 0)**: 
  - Static reports
  - Print materials
  - Performance optimization
  - Reduced distraction

**Configuration Options**:
```js
{
    amplitude: 0,          // Wave height (0 = flat)
    waveAnimation: 0,      // Animation speed (0 = still)
    direction: 'right',    // Wave direction: 'left'/'right'
    period: 2000,          // Animation period in ms
    phase: 0,              // Initial wave phase (0-360)
    color: ['#294D99'],    // Wave color(s)
    backgroundStyle: {...}, // Container background
    outline: {...}         // Container outline
}
```

**Value Ranges**:
- Data values: 0.0 to 1.0 (0% to 100%)
- Amplitude: 0 to ~20 (wave height)
- Period: milliseconds (lower = faster)
- Phase: 0 to 360 degrees

**Common Patterns**:
```js
// Percentage display
data: [0.75]  // Shows "75%"

// Multiple waves
data: [0.8, 0.7, 0.6]  // 3 overlapping waves

// Custom colors
color: ['#FF6B6B', '#4ECDC4', '#45B7D1']

// Fast animation
period: 1000, waveAnimation: 1
```