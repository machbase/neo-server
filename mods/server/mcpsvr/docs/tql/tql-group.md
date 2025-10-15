# Machbase Neo TQL GROUP Aggregation

*Version 8.0.7 or later*

## Syntax

```
GROUP( [lazy(boolean)] [, by()] [, aggregator...] )
```

**Parameters:**
- `lazy(boolean)` - Set lazy mode (default: false)
- `by(value [, timewindow()] [, name])` - Specify how to make group with given value. `by()` was mandatory in `GROUP()` but it has become optional (version 8.0.14 or later) to apply aggregator on the whole data in a time.
- `aggregator` - List of aggregator functions, comma separated multiple functions are possible.

**Example:**

```js
FAKE(json({
    ["A", 1],
    ["A", 2],
    ["B", 3],
    ["B", 4]
}))
GROUP(
    by( value(0), "CATEGORY" ),
    avg( value(1), "AVG" ),
    sum( value(1), "SUM"),
    first( value(1) * 10, "x10")
)
CSV( header(true) )
```

## Options

### by()

`by()` takes value as the first argument and optionally `timewindow()` and `name`.

**Syntax**: `by( value [, timewindow] [, label] )`

**Parameters:**
- `value` - Grouping value, usually it might be time.
- `timewindow(from, until, period)` - Timewindow option.
- `label` - String, set new column label (default "GROUP")

### lazy()

**Syntax**: `lazy(boolean)`

If it set `false` which is default, `GROUP()` works comparing the value of `by()` of the current record to the other value of previous record. If it finds the value has been changed, then produces new record. As result it can make a group only if the continuous records have a same value of `by()`.

If `lazy(true)` is set, `GROUP()` waits the end of the input stream before yield any record to collecting all records, so that un-sorted `by()` value can be grouped, but it causes heavy memory consumption.

### timewindow()

**Syntax**: `timewindow( from, until, period )`

*Version 8.0.13 or later*

**Parameters:**
- `from`, `until` - Time range. Note that `from` is inclusive and `until` is exclusive. Regardless of the existence of actual data, you can specify the desired time range.
- `period` - Duration, represents the time interval between `from` and `until`.

See the [timewindow example](#timewindow-example)

Analyzing and visualizing data stored in a database can be cumbersome, especially when there is no data in the desired time range or when there are multiple data points.

For example, when displaying a time-value chart at fixed intervals, simply querying the data with a SELECT statement and inputting it into the chart library may result in time intervals between records that do not align with the chart's time axis. This misalignment can occur due to missing intermediate data or densely packed data within a time period, making it difficult to format the data as desired.

Typically, application developers create an array of fixed time intervals and iteratively fill the elements (slots) of the array by traversing the query result records. When a slot already contains a value, it is maintained as a single value through a specific operation (e.g., min, max, first, last). At the end, slots without values are filled with arbitrary values (e.g., 0 or NULL).

## Aggregators

If no aggregator is specified `GROUP` make new array of the raw records for a each group by default.

Takes multiple continuous records that have same value of `by()`, then produces a new record which have value array contains all individual values. For example, if an original records was `{key:k, value:[v1, v2]}`, `{key:k, value:{v3, v4}}`...`{key:k, value:{vx, vy}}`, `GROUP( by(key()) )` produces the new record as `{key:k, value:[[v1,v2],[v3,v4],...,[vx,vy]]}`.

**Syntax**: `function_name( value [, value...] [, where()] [, nullValue()] [, predict()] [, label])`

**Parameters:**
- `value` - One or more values depends on the function.
- `where( predicate )` - Take boolean expression for the predication.
- `nullValue(alternative)` - Specify alternative value to use instead of `NULL` when the aggregator has no value to produce.
- `predict(algorithm)` - Specify an algorithm to predict value to use instead of `NULL` when the aggregator has no value to produce.
- `label` - String, set the label of the column (default is the name of aggregator function).

There are two types of aggregator functions:
- **Type 1** functions only keep the final candidate value for the result.
- **Type 2** functions hold the whole data of a group, it uses the data to produce the result of the aggregation then release memory for the next group.

If `GROUP()` use `lazy(true)` and Type 2 functions together, it holds the entire input data of the related columns.

### Aggregator Options

`where()`, `nullValue()`, `predict()` and `label` arguments are optional and it represents the `option` of the each function syntax description below.

#### where()

**Syntax**: `where(predicate)`

*Version 8.0.13 or later*

See the [where example](#where-example)

#### nullValue()

**Syntax**: `nullValue(alternative)`

*Version 8.0.13 or later*

See the [nullValue example](#nullvalue-example)

#### predict()

**Syntax**: `predict(algorithm)`

*Version 8.0.13 or later*

See the [predict example](#predict-example)

**Available algorithms:**

| Algorithm | Description |
| :--- | :--- |
| `PiecewiseConstant` | A left-continuous, piecewise constant 1-dimensional interpolator. |
| `PiecewiseLinear` | A piecewise linear 1-dimensional interpolator |
| `AkimaSpline` | A piecewise cubic 1-dimensional interpolation with continuous value and first derivative. See https://www.iue.tuwien.ac.at/phd/rottinger/node60.html |
| `FritschButland` | A piecewise cubic 1-dimensional interpolation with continuous value and first derivative. See Fritsch, F. N. and Butland, J., "A method for constructing local monotone piecewise cubic interpolants" (1984), SIAM J. Sci. Statist. Comput., 5(2), pp. 300-304. |
| `LinearRegression` | Linear regression with nearby values |

## Aggregator Functions

### avg()

**Type 1**

**Syntax**: `avg(x [, option...])`

Average of the values in a group.

### sum()

**Type 1**

**Syntax**: `sum(x [, option...])`

Total sum of the values in a group.

### count()

**Type 1**

**Syntax**: `count(x [, option...])`

*Version 8.0.13 or later*

Count of the values in a group.

### first()

**Type 1**

**Syntax**: `first(x [, option...])`

The first value of the group.

### last()

**Type 1**

**Syntax**: `last(x [, option...])`

The last value of the group.

### min()

**Type 1**

**Syntax**: `min(x [, option...])`

The smallest value of the group.

### max()

**Type 1**

**Syntax**: `max(x [, option...])`

The largest value of the group.

### rss()

**Type 1**

**Syntax**: `rss(x [, option...])`

Root sum square

### rms()

**Type 1**

**Syntax**: `rms(x [, option...])`

Root mean square

### list()

**Type 2**

**Syntax**: `list(x [, option...])`

*Version 8.0.15 or later*

**Parameters:**
- `x` - Float value

`list()` aggregates the all x values and produce a single list which contains the individual values.

#### Example 1: JSON Output

```js
FAKE(json({["A",1], ["A",2], ["B",3], ["B",4], ["C",5]}))
GROUP(
    by(value(0)),
    list(value(1))
)
JSON()
```

**Output:**

```json
{
    "data":{
        "columns":["GROUP","LIST"],
        "types":["string","float64"],
        "rows":[
            ["A",[1,2]],
            ["B",[3,4]],
            ["C",[5]]
        ]
    },
    "success":true,
    "reason":"success",
    "elapse":"220.375µs"
}
```

#### Example 2: JSON(rowsArray) Output

```js
FAKE(json({["A",1], ["A",2], ["B",3], ["B",4], ["C",5]}))
GROUP(
    by(value(0),"name"),
    avg(value(1), "avg"),
    list(value(1), "values")
)
JSON(rowsArray(true))
```

**Output:**

```json
{
    "data": {
        "columns": ["name", "values", "avg"],
        "types": [ "string", "list", "float64" ],
        "rows": [
            {  "name": "A", "avg": 1.5, "values": [ 1, 2 ] },
            {  "name": "B", "avg": 3.5, "values": [ 3, 4 ] },
            {  "name": "C", "avg": 5,  "values": [ 5 ] }
        ]
    },
    "success": true,
    "reason": "success",
    "elapse": "270.25µs"
}
```

#### Example 3: FLATTEN Output

```js
FAKE(json({["A",1], ["A",2], ["B",3], ["B",4], ["C",5]}))
GROUP(
    by(value(0)),
    list(value(1))
)
POPVALUE(0)
FLATTEN()
JSON()
```

**Output:**

```json
{
    "data": {
        "columns": ["LIST"],
        "types": ["list"],
        "rows": [
            [1,2],
            [3,4],
            [5]
        ]
    },
    "success": true,
    "reason": "success",
    "elapse": "252.625µs"
}
```

### lrs()

**Type 2**

**Syntax**: `lrs(x, y [, weight(w)] [, option...])`

*Version 8.0.13 or later*

**Parameters:**
- `x` - Float or time
- `y` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

Linear Regression Slope, assuming x-y is a point on a orthogonal coordinate system. x can be number or time type.

### mean()

**Type 2**

**Syntax**: `mean(x [, weight(w)] [, option...])`

**Parameters:**
- `x` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

`mean()` computes the weighted mean of the grouped values. If all of the weights are 1, use the lightweight `avg()` for the performance.

**Formula**: mean(x, weight(w)) = (Σ w<sub>i</sub> x<sub>i</sub>) / (Σ w<sub>i</sub>)

### cdf()

**Type 2**

**Syntax**: `cdf(x, q [, weight(w)] [, option...])`

*Version 8.0.14 or later*

**Parameters:**
- `x` - Float
- `q` - Float
- `weight(w)` - If omitted then all of the weights are 1.

`cdf()` returns the empirical cumulative distribution function value of x, that is the fraction of the samples less than or equal to q. `cdf()` is theoretically the inverse of the `quantile()` function, though it may not be the actual inverse for all values q.

### correlation()

**Type 2**

**Syntax**: `correlation(x, y [, weight(w)] [, option...])`

*Version 8.0.14 or later*

**Parameters:**
- `x`, `y` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

`correlation()` returns the weighted correlation between the samples of x and y.

**Formula**: correlation(x, y, weight(w)) = (Σ w<sub>i</sub> (x<sub>i</sub> - x̄) (y<sub>i</sub> - ȳ)) / (stdX * stdY), where x̄ = mean x, ȳ = mean y

### covariance()

**Type 2**

**Syntax**: `covariance(x, y [, weight(w)] [, option...])`

*Version 8.0.14 or later*

**Parameters:**
- `x`, `y` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

`covariance()` returns the weighted covariance between the samples of x and y.

**Formula**: covariance(x, y, weight(w)) = (Σ w<sub>i</sub> (x<sub>i</sub> - x̄) (y<sub>i</sub> - ȳ)) / (Σ w<sub>i</sub> - 1), where x̄ = mean x, ȳ = mean y

### quantile()

**Type 2**

**Syntax**: `quantile(x, p [, weight(w)] [, option...])`

*Version 8.0.13 or later*

**Parameters:**
- `x` - Float value
- `p` - Float fraction
- `weight(w)` - If omitted then all of the weights are 1.

`quantile()` returns the sample of x such that x is greater than or equal to the fraction p of samples, p should be a number between 0 and 1.

It returns the lowest value q for which q is greater than or equal to the fraction p of samples.

### quantileInterpolated()

**Type 2**

**Syntax**: `quantileInterpolated(x, p [, weight(w)] [, option...])`

*Version 8.0.13 or later*

**Parameters:**
- `x` - Float value
- `p` - Float fraction
- `weight(w)` - If omitted then all of the weights are 1.

`quantileInterpolated()` returns the sample of x such that x is greater than or equal to the fraction p of samples, p should be a number between 0 and 1.

The return value is the linearly interpolated.

### median()

**Type 2**

**Syntax**: `median(x [, weight(w)] [, option...])`

**Parameters:**
- `x` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

Equivalent to `quantile(x, 0.5 [, option...])` 

### medianInterpolated()

**Type 2**

**Syntax**: `medianInterpolated(x [, weight(w)] [, option...])`

**Parameters:**
- `x` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

Equivalent to `quantileInterpolated(x, 0.5 [, option...])`

### stddev()

**Type 2**

**Syntax**: `stddev(x [, weight(w)] [, option...])`

**Parameters:**
- `weight(w)` - If omitted then all of the weights are 1.

`stddev()` returns the sample standard deviation.

### stderr()

**Type 2**

**Syntax**: `stderr(x [, weight(w)] [, option...])`

**Parameters:**
- `weight(w)` - If omitted then all of the weights are 1.

`stderr()` returns the standard error in the mean with stddev of the given values.

### entropy()

**Type 2**

**Syntax**: `entropy(x [, option...])`

Shannon entropy of a distribution. The natural logarithm is used.

### mode()

**Type 2**

**Syntax**: `mode(x [, weight(w)] [, option...])`

**Parameters:**
- `weight(w)` - If omitted then all of the weights are 1.

`mode()` returns the most common value in the dataset specified by value and the given weights. Strict float64 equality is used when comparing values, so users should take caution. If several values are the mode, any of them may be returned.

### moment()

**Type 2**

**Syntax**: `moment(x, n [, weight(w)] [, option...])`

*Version 8.0.14 or later*

**Parameters:**
- `x` - Float64 value
- `n` - Float64 moment
- `weight(w)` - If omitted then all of the weights are 1.

`moment()` computes the weighted n-th moment of the samples.

### variance()

**Type 2**

**Syntax**: `variance(x [, weight(w)] [, option...])`

*Version 8.0.14 or later*

**Parameters:**
- `x` - Float value
- `weight(w)` - If omitted then all of the weights are 1.

`variance()` computes the unbiased weighted variance of the grouped values. When weights sum to 1 or less, a biased variance estimator should be used.

**Example:**

```js
FAKE(json({[8,2], [2,2], [-9,6], [15,7], [4,1]}))
GROUP(
    variance(value(0), "VARIANCE"),
    variance(value(0), weight(value(1)), "WEIGHTED VARIANCE")
)
CSV(heading(true), precision(4))
```

## Examples

### timewindow() Example

`FAKE()` generates time-value at every 1ms, so there are 1,000 records within 1 second. Executing the below TQL produces data at 1-second intervals (`period("1s")` in `timewindow()`), and if there is no actual data (record) in the desired time period, it is filled with the default value NULL.

```js
FAKE(
    oscillator(
        freq(10, 1.0), freq(35, 2.0), 
        range('now', '10s', '10ms')) 
)
GROUP(
    by( value(0),
        timewindow(time('now - 2s'), time('now + 13s'), period("1s")),
        "TIME"
    ),
    last( value(1),
          "LAST"
    )
)
CSV(sqlTimeformat('YYYY-MM-DD HH24:MI:SS'), heading(true))
```

### nullValue() Example

Let’s add nullValue(100) and execute again. The NULL values are replaced with the given value 100.

```js
FAKE(
    oscillator(
        freq(10, 1.0), freq(35, 2.0), 
        range('now', '10s', '10ms')) 
)
GROUP(
    by( value(0),
        timewindow(time('now - 2s'), time('now + 13s'), period("1s")),
        "TIME"
    ),
    last( value(1),
          nullValue(100),
          "LAST"
    )
)
CSV(sqlTimeformat('YYYY-MM-DD HH24:MI:SS'), heading(true))
```

### predict() Example

It is possible to obtain interpolated data by referring to adjacent values beyond filling empty values (NULL) with a constant specified by `nullValue()`. Using the example above, add `predict("LinearRegression")` to `last()` and execute it again. You can see that the value predicted by Linear Regression is filled in the records where NULL was returned because there was no value.

The `predict()` may fail to produce interpolation value when there are not enough values nearby to predict, then the `nullValue()` is applied instead. If `nullValue()` is not given, then `NULL` is returned.

```js
FAKE(
    oscillator(
        freq(10, 1.0), freq(35, 2.0), 
        range('now', '10s', '10ms')) 
)
GROUP(
    by( value(0),
        timewindow(time('now - 2s'), time('now + 13s'), period("1s")),
        "TIME"
    ),
    last( value(1),
          predict("LinearRegression"),
          nullValue(100),
          "LAST"
    )
)
CSV(sqlTimeformat('YYYY-MM-DD HH24:MI:SS'), heading(true))
```

### where() Example

Let's say there are two sensors that measure temperature and humidity, each one store the data per every 1 sec.
In real world, there is always time difference among the sensor systems. So the stored data might be below samples.

Record #5 humidity data store earlier than expect and it happens on record #9 again. Let's normalize the data in second precision.

```js
FAKE( json({
    ["temperature", 1691800174010, 16],
    ["humidity",    1691800174020, 64],
    ["temperature", 1691800175001, 17],
    ["humidity",    1691800175010, 63],
    ["humidity",    1691800176999, 66],
    ["temperature", 1691800176020, 18],
    ["temperature", 1691800177125, 18],
    ["humidity",    1691800177293, 66],
    ["humidity",    1691800177998, 66],
    ["temperature", 1691800178184, 18]
}) )
MAPVALUE(1, parseTime(value(1), "ms"))

GROUP(
    by( roundTime(value(1), "1s")),
    avg( value(2) )
)

CSV( timeformat("Default"), header(true) )
```

`roundTime(..., "1s")` makes time value aligned in second, then make records grouped that has same time. `avg(...)` produces the average value of a group.

But it lost the first column information that indicates if the value is temperature or humidity, so the result values become meaningless. To solve this problem use `where()`. Aggregator functions accept values only when `where()` has the predicate `true`.

```js
GROUP(
    by( roundTime(value(1), "1s"), "TIME"),
    avg( value(2),
         where( value(0) == 'temperature' ),
         "TEMP" ),
    avg( value(2),
         where( value(0) == 'humidity' ),
         "HUMI" )
)
```

It is also possible to interpolate the missing data of the last record with `predict()` and `nullValue()`

```js
GROUP(
    by( roundTime(value(1), "1s"), "TIME"),
    avg( value(2),
         where( value(0) == 'temperature' ),
         "TEMP" ),
    avg( value(2),
         where( value(0) == 'humidity' ),
         predict("PiecewiseLinear"),
         "HUMI" )
)
```

### Chart Example

```js
CSV(file("https://docs.machbase.com/assets/example/iris.csv"))
FILTER( strToUpper(value(4)) == "IRIS-SETOSA")
GROUP( by(value(4)), 
    min(value(0), "Min"),
    median(value(0), "Median"),
    avg(value(0), "Avg"),
    max(value(0), "Max"),
    stddev(value(0), "StdDev.")
)
CHART(
    chartOption({
        "xAxis": { "type": "category", "data": ["iris-setosa"]},
        "yAxis": {},
        "legend": {"show": "true"},
        "series": [
            {"type":"bar", "name": "Min", "data": column(1)},
            {"type":"bar", "name": "Median", "data": column(2)},
            {"type":"bar", "name": "Avg", "data": column(3)},
            {"type":"bar", "name": "Max", "data": column(4)},
            {"type":"bar", "name": "StdDev.", "data": column(5)}
        ]
    })
)
```