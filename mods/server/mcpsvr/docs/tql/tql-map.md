# Machbase Neo TQL MAP Functions

*MAP* functions are the core of the transforming data.

## TAKE()

*Syntax*: `TAKE( [offset,] n )`

Takes first *n* records and stop the stream.

- `offset` *number* optional, take records from the offset. (default 0 when omitted) (since v8.0.6)
- `n` *number* specify how many records to be taken.

**Example: TAKE(n)**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
TAKE(2)
CSV()
```

```csv
TAG0,1628694000000000000,10
TAG0,1628780400000000000,11
```

**Example: TAKE(offset, n)**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
TAKE(3, 2)
CSV()
```

```csv
TAG0,1628953200000000000,13
TAG0,1629039600000000000,14
```

## DROP()

*Syntax*: `DROP( [offset,] n  )`

Ignore first *n* records, it simply drops the *n* records.

- `offset` *number* optional, drop records from the offset. (default 0 when omitted) (since v8.0.6)
- `n` *number* specify how many records to be dropped.

**Example: DROP(n)**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
DROP(3)
CSV()
```

```csv
TAG0,1628953200000000000,13
TAG0,1629039600000000000,14
TAG0,1629126000000000000,15
```

**Example: DROP(offset, n)**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
DROP(2, 3)
CSV()
```

```csv
TAG0,1628694000000000000,10
TAG0,1628780400000000000,11
TAG0,1629126000000000000,15
```

## FILTER()

*Syntax*: `FILTER( condition )`

Apply the condition statement on the incoming record, then it pass the record only if the *condition* is *true*.

For example, if an original record was `{key: k1, value[v1, v2]}` and apply `FILTER(count(V) > 2)`, it simply drop the record. If the condition was `FILTER(count(V) >= 2)`, it pass the record to the next function.

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
FILTER( value(2) < 12 )
CSV()
```

```csv
TAG0,1628694000000000000,10
TAG0,1628780400000000000,11
```

## FILTER_CHANGED()

*Syntax*: `FILTER_CHANGED( value [, retain(time, duration)] [, useFirstWithLast()] )` (since v8.0.15)

- `retain(time, duration)`
- `useFirstWithLast(boolean)`

It passes only the `value` has been changed from the previous.
The first record is always passed, use `DROP(1)` after `FILTER_CHANGED()` to discard the first record.

If `retain()` option is specified, the records that keep the changed `value` for the given `duration` based `time`, are passed.

**Example: Basic usage**

```js
FAKE(json({
    ["A", 1692329338, 1.0],
    ["A", 1692329339, 2.0],
    ["B", 1692329340, 3.0],
    ["B", 1692329341, 4.0],
    ["B", 1692329342, 5.0],
    ["B", 1692329343, 6.0],
    ["B", 1692329344, 7.0],
    ["B", 1692329345, 8.0],
    ["C", 1692329346, 9.0],
    ["D", 1692329347, 9.1]
}))
MAPVALUE(1, parseTime(value(1), "s"))
FILTER_CHANGED(value(0))
CSV(timeformat("s"))
```

```csv
A,1692329338,1
B,1692329340,3
C,1692329346,9
D,1692329347,9.1
```

**Example: With retain()**

```js
FAKE(json({
    ["A", 1692329338, 1.0],
    ["A", 1692329339, 2.0],
    ["B", 1692329340, 3.0],
    ["B", 1692329341, 4.0],
    ["B", 1692329342, 5.0],
    ["B", 1692329343, 6.0],
    ["B", 1692329344, 7.0],
    ["B", 1692329345, 8.0],
    ["C", 1692329346, 9.0],
    ["D", 1692329347, 9.1]
}))
MAPVALUE(1, parseTime(value(1), "s"))
FILTER_CHANGED(value(0), retain(value(1), "2s"))
CSV(timeformat("s"))
```

```csv
A,1692329338,1
B,1692329342,5
```

**Example: With retain() and useFirstWithLast(false)**

```js
FAKE(json({
    ["A", 1692329338, 1.0],
    ["A", 1692329339, 2.0],
    ["B", 1692329340, 3.0],
    ["B", 1692329341, 4.0],
    ["B", 1692329342, 5.0],
    ["B", 1692329343, 6.0],
    ["B", 1692329344, 7.0],
    ["B", 1692329345, 8.0],
    ["C", 1692329346, 9.0],
    ["D", 1692329347, 9.1]
}))
MAPVALUE(1, parseTime(value(1), "s"))
FILTER_CHANGED(value(0), retain(value(1), "2s"), useFirstWithLast(false))
CSV(timeformat("s"))
```

```csv
A,1692329338,1
B,1692329340,3
```

**Example: With useFirstWithLast(true)**

```js
FAKE(json({
    ["A", 1692329338, 1.0],
    ["A", 1692329339, 2.0],
    ["B", 1692329340, 3.0],
    ["B", 1692329341, 4.0],
    ["B", 1692329342, 5.0],
    ["B", 1692329343, 6.0],
    ["B", 1692329344, 7.0],
    ["B", 1692329345, 8.0],
    ["C", 1692329346, 9.0],
    ["D", 1692329347, 9.1]
}))
MAPVALUE(1, parseTime(value(1), "s"))
FILTER_CHANGED(value(0), useFirstWithLast(true))
CSV(timeformat("s"))
```

```csv
A,1692329338,1
A,1692329339,2
B,1692329340,3
B,1692329345,8
C,1692329346,9
C,1692329346,9
D,1692329347,9.1
D,1692329347,9.1
```

## SET()

*Syntax*: `SET(name, expression)` (since v8.0.12)

- `name` *keyword* variable name
- `expression` *expression* value

*SET* defines a record-scoped variable with given name and value. If a new variable `var` is defined as `SET(var, 10)`, it can be referred as `$var`. Because the variables are not a part of the values, it is not included in the final result of SINK.

```js
FAKE( linspace(0, 1, 3))
SET(temp, value(0) * 10)
SET(temp, $temp + 1)
MAPVALUE(1, $temp)
CSV()
```

```csv
0,1
0.5,6
1,11
```

## GROUP()

*Syntax*: `GROUP( [lazy(boolean),] by [, aggregators...] )` (since v8.0.7)

- `lazy(boolean)` If it set `false` which is default, *GROUP()* yields new aggregated record when the value of `by()` has changed from previous record. If it set `true`, *GROUP()* waits the end of the input stream before yield any record.

- `by(value [, label])` The value how to group the values.

- `aggregators` *array of aggregator* Aggregate functions

Group aggregation function, please refer to the GROUP() section for the detail description.

## PUSHVALUE()

*Syntax*: `PUSHVALUE( idx, value [, name] )` (since v8.0.5)

- `idx` *number* Index where newValue insert at. (0 based)
- `value` *expression* New value
- `name` *string* column's name (default 'column')

Insert the given value (new column) into the current values.

```js
FAKE( linspace(0, 1, 3))
PUSHVALUE(1, value(0) * 10)
CSV()
```

```csv
0.0,0
0.5,5
1.0,10
```

## POPVALUE()

*Syntax*: `POPVALUE( idx [, idx2, idx3, ...] )` (since v8.0.5)

- `idx` *number* array of indexes that will removed from values

It removes column of values that specified by `idx`es from value array.

```js
FAKE( linspace(0, 1, 3))
PUSHVALUE(1, value(0) * 10)
POPVALUE(0)
CSV()
```

```csv
0
5
10
```

## MAPVALUE()

*Syntax*: `MAPVALUE( idx, newValue [, newName] )`

- `idx` *number*  Index of the value tuple. (0 based)
- `newValue` *expression* New value
- `newName` *string* change column's name with given string

`MAPVALUE()` replaces the value of the element at the given index. For example, `MAPVALUE(0, value(0)*10)` replaces a new value that is 10 times of the first element of value tuple.

If the `idx` is out of range, it works as `PUSHVALUE()` does. `MAPVALUE(-1, value(1)+'_suffix')` inserts a new string value that concatenates '_suffix' with the 2nd element of value.

```js
FAKE( linspace(0, 1, 3))
MAPVALUE(0, value(0) * 10)
CSV()
```

```csv
0
5
10
```

An example use of mathematic operation with `MAPVALUE`.

```js
FAKE(
    meshgrid(
        linspace(-4,4,100),
        linspace(-4,4, 100)
    )
)
MAPVALUE(2, sin(pow(value(0), 2) + pow(value(1), 2)) / (pow(value(0), 2) + pow(value(1), 2)))
MAPVALUE(0, list(value(0), value(1), value(2)))
POPVALUE(1, 2)
CHART(
    plugins("gl"),
    size("600px", "600px"),
    chartOption({
        grid3D:{},
        xAxis3D:{}, yAxis3D:{}, zAxis3D:{},
        series:[
            {type: "line3D", data: column(0)},
        ]
    })
)
```

## MAP_DIFF()

*Syntax*: `MAP_DIFF( idx, value [, newName] )` (since v8.0.8)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `newName` *string* change column's name with given string

`MAP_DIFF()` replaces the value of the element at the given index with difference between current and previous values (*current - previous*). 

```js
FAKE( linspace(0.5, 3, 10) )
MAPVALUE(0, log(value(0)), "VALUE")
MAP_DIFF(1, value(0), "DIFF")
CSV( header(true), precision(3) )
```

```csv
VALUE,DIFF
-0.693,NULL
-0.251,0.442
0.054,0.305
0.288,0.234
0.477,0.189
0.636,0.159
0.773,0.137
0.894,0.121
1.001,0.108
1.099,0.097
```

## MAP_ABSDIFF()

*Syntax*: `MAP_ABSDIFF( idx, value [, label]  )` (since v8.0.8)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `label` *string* change column's label with given string

`MAP_ABSDIFF()` replaces the value of the element at the given index with absolute difference between current and previous value abs(*current - previous*).

## MAP_NONEGDIFF()

*Syntax*: `MAP_NONEGDIFF( idx, value [, label]  )` (since v8.0.8)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `label` *string* change column's label with given string

`MAP_NONEGDIFF()` replaces the value of the element at the given index with difference between current and previous value (*current - previous*). 
If the difference is less than zero it applies zero instead of a negative value.

## MAP_AVG()

*Syntax*: `MAP_AVG(idx, value [, label] )`  (since v8.0.15)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `label` *string* change column's label with given string

`MAP_AVG` sets the value of the element at the given index with a average of values which is the averaging filter.

When $k$ is number of data.

Let $\alpha = \frac{1}{k}$

$\overline{x_k} = (1 - \alpha) \overline{x_{k-1}} + \alpha x_k$

```js
FAKE(arrange(0, 1000, 1))
MAPVALUE(1, sin(2 * PI *10*value(0)/1000))
MAP_AVG(2, value(1))
CHART(
    chartOption({
        xAxis:{ type:"category", data:column(0)},
        yAxis:{},
        series:[
            { type:"line", data:column(1), name:"RAW" },
            { type:"line", data:column(2), name:"AVG" }
        ],
        legend:{ bottom:10}
    })
)
```

## MAP_MOVAVG()

*Syntax*: `MAP_MOVAVG(idx, value, window [, label] )`  (since v8.0.8)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `window` *number* specifies how many records it accumulates.
- `label` *string* change column's label with given string

`MAP_MOVAVG` sets the value of the element at the given index with a moving average of values by given window count.
If values are not accumulated enough to the `window`, it applies `sum/count_of_values` instead.
If all incoming values are `NULL` (or not a number) for the last `window` count, it applies `NULL`.
If some accumulated values are `NULL` (or not a number), it makes average value from only valid values excluding the `NULL`s.

```js
FAKE(arrange(1,5,0.03))
MAPVALUE(0, round(value(0)*100)/100)
SET(sig, sin(1.2*2*PI*value(0)) )
SET(noise, 0.09*cos(9*2*PI*value(0)) + 0.15*sin(12*2*PI*value(0)))
MAPVALUE(1, $sig + $noise)
MAP_MOVAVG(2, value(1), 10)
CHART(
    chartOption({
        xAxis:{ type: "category", data: column(0)},
        yAxis:{ max:1.5, min:-1.5 },
        series:[
            { type: "line", data: column(1), name:"value+noise" },
            { type: "line", data: column(2), name:"MA(10)" },
        ],
        legend: { bottom: 10 }
    })
)
```

## MAP_LOWPASS()

*Syntax*: `MAP_LOWPATH(idx, value, alpha [, label] )` (since v8.0.15)

- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `alpha` *number*, 0 < alpha < 1
- `label` *string* change column's label with given string

`MAP_LOWPASS` sets the value of the elment at the given index with exponentially weighted moving average.

When $ 0 < \alpha < 1$

$\overline{x_k} = (1 - \alpha) \overline{x_{k-1}} + \alpha x_k$

```js
FAKE(arrange(1,5,0.03))
MAPVALUE(0, round(value(0)*100)/100)
SET(sig, sin(1.2*2*PI*value(0)) )
SET(noise, 0.09*cos(9*2*PI*value(0)) + 0.15*sin(12*2*PI*value(0)))
MAPVALUE(1, $sig + $noise)
MAP_LOWPASS(2, $sig + $noise, 0.40)
CHART(
    chartOption({
        xAxis:{ type: "category", data: column(0)},
        yAxis:{ max:1.5, min:-1.5 },
        series:[
            { type: "line", data: column(1), name:"value+noise" },
            { type: "line", data: column(2), name:"lpf" },
        ],
        legend: { bottom: 10 }
    })
)
```

## MAP_KALMAN()

*Syntax*: `MAP_KALMAN(idx, value, model() [, label])` (since v8.0.15)
- `idx` *number*  Index of the value tuple. (0 based)
- `value` *number*
- `model` *model(initial, progress, observation)* Set system matrices
- `label` *string* change column's label with given string

```js
FAKE(arrange(0, 10, 0.1))
MAPVALUE(0, round(value(0)*100)/100 )

SET(real, 14.4)
SET(noise, 4 * simplex(1234, value(0)))
SET(measure, $real + $noise)

MAPVALUE(1, $real )
MAPVALUE(2, $measure)
MAP_KALMAN(3, $measure, model(0.1, 0.001, 1.0))
CHART(
    chartOption({
        title:{text:"Kalman filter"},
        xAxis:{type:"category", data:column(0)},
        yAxis:{ min:10, max: 18 },
        series:[
            {type:"line", data:column(1), name:"real"},
            {type:"line", data:column(2), name:"measured"},
            {type:"line", data:column(3), name:"filtered"}
        ],
        tooltip: {show: true, trigger:"axis"},
        legend: { bottom: 10},
        animation: false
    })
)
```

## HISTOGRAM()

There are two types of `HISTOGRAM()`. The first type is "fixed bins" which is useful when the input value range (min to max) is predictable or fixed. The second type is "dynamic bins" which is useful when the value ranges are unknown.

### Fixed Bins

*Syntax*: `HISTOGRAM(value, bins [, category] [, order] )`  (since v8.0.15)

- `value` *number*
- `bins` *bins(min, max, step)* histogram bin configuration.
- `category` *category(name_value)*
- `order` *order(name...string)* category order

`HISTOGRAM()` takes values and count the distribution of the each bins, the bins are configured by min/max range of the value and the count of bins.
If the actual value comes in the out of the min/max range, `HISTOGRAM()` adds lower or higher bins automatically.

**Example: CSV output**

```js
FAKE( arrange(1, 100, 1) )
MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
HISTOGRAM(value(0), bins(0, 200, 40))
CSV( precision(0), header(true) )
```

```csv
low,high,count
0,40,2
40,80,31
80,120,47
120,160,16
160,200,4
```

**Example: CHART output**

```js
FAKE( arrange(1, 100, 1) )
MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
HISTOGRAM(value(0), bins(0, 200, 40))
MAPVALUE(0, strSprintf("%.f~%.f", value(0), value(1)))
CHART(
    chartOption({
        xAxis:{ type:"category", data:column(0)},
        yAxis:{},
        tooltip:{trigger:"axis"},
        series:[
            {type:"bar", data: column(2)}
        ]
    })
)
```

**Example: With CATEGORY**

```js
FAKE( arrange(1, 100, 1) )
MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
PUSHVALUE(0, key() % 2 == 0 ? "Cat.A" : "Cat.B")
HISTOGRAM(value(1), bins(0, 200, 40), category(value(0)))
MAPVALUE(0, strSprintf("%.f~%.f", value(0), value(1)))
CHART(
    chartOption({
        xAxis:{ type:"category", data:column(0)},
        yAxis:{},
        tooltip:{trigger:"axis"},
        legend:{bottom:5},
        series:[
            {type:"bar", data: column(2), name:"Cat.A"},
            {type:"bar", data: column(3), name:"Cat.B"},
        ]
    })
)
```

### Dynamic Bins

*Syntax*: `HISTOGRAM(value [, bins(maxBins)] )`  (since v8.0.46)

- `value` *number*
- `bins` *number* specifies the maximum number of bins. The default is 100 if not specified.

`HISTOGRAM()` takes values and a maximum number of bins.
The bins are dynamically adjusted based on the input values and can expand up to the specified `bins(maxBins)`.
The resulting `value` column represents the average value of each bin,
while the `count` column indicates the number of values within that range.
Thus, the product of `value` and `count` for a bin equals the sum of the values within that bin.

```js
FAKE( arrange(1, 100, 1) )
MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
HISTOGRAM(value(0), bins(5))
CSV( precision(0), header(true) )
```

```csv
value,count
47,12
75,29
99,29
119,18
156,12
```

## BOXPLOT()

*Syntax*: `BOXPLOT(value, category [, order] [, boxplotInterp] [, boxplotOutput])` (since v8.0.15)

- `value` *number*
- `category` *category(name_value)*
- `order` *order(name...string)* category order
- `boxplotOutput` *boxplotOutput( "" | "chart" | "dict" )*
- `boxplotInterp` *boxplotInterop(Q1 boolean, Q2 boolean, Q3 boolean)*

## TRANSPOSE()

*Syntax*: `TRANSPOSE( [fixed(columnIdx...) | columnIdx...] [, header(boolean)] )` (since v8.0.8)

When TQL loads data from CSV or external RDBMS via 'bridge'd SQL query, it may require to transpose columns to fit the record shape to a MACHBASE TAG table.
`TRANSPOSE` produce multiple records from a record that has multiple columns.

- `fixed(columnIdx...)` specify which columns are "fixed", this can not mix-use with transposed columns.
- `columnIdx...` specify multiple columns which are "transposed", this can not mix-use with "fixed()".
- `header(boolean)` if it set `header(true)`, `TRANSPOSE` consider the first record is the header record. And it produce the header of the transposed column records as a new column.

**Example: TRANSPOSE with header**

```js
FAKE(csv(`CITY,DATE,TEMPERATURE,HUMIDITY,NOISE
Tokyo,2023/12/07,23,30,40
Beijing,2023/12/07,24,50,60
`))
TRANSPOSE( header(true), 2, 3, 4 )
MAPVALUE(0, strToUpper(value(0)) + "-" + value(2))
MAPVALUE(1, parseTime(value(1), sqlTimeformat("YYYY/MM/DD")))
MAPVALUE(3, parseFloat(value(3)))
POPVALUE(2)
CSV(timeformat("s"))
```

This example is a common use case.

```csv
TOKYO-TEMPERATURE,1701907200,23
TOKYO-HUMIDITY,1701907200,30
TOKYO-NOISE,1701907200,40
BEIJING-TEMPERATURE,1701907200,24
BEIJING-HUMIDITY,1701907200,50
BEIJING-NOISE,1701907200,60
```

**Example: TRANSPOSE all columns**

```js
FAKE(csv(`CITY,DATE,TEMPERATURE,HUMIDITY,NOISE
Tokyo,2023/12/07,23,30,40
Beijing,2023/12/07,24,50,60
`))
TRANSPOSE()
CSV()
```

It transposes all columns into rows if there is no options.

```csv
CITY
DATE
TEMPERATURE
HUMIDITY
NOISE
Tokyo
2023/12/07
23
30
40
Beijing
2023/12/07
24
50
60
```

**Example: TRANSPOSE with header()**

```js
FAKE(csv(`CITY,DATE,TEMPERATURE,HUMIDITY,NOISE
Tokyo,2023/12/07,23,30,40
Beijing,2023/12/07,24,50,60
`))
TRANSPOSE( header(true) )
CSV()
```

It treats the first record as the header and add a new column for each transposed record.

```csv
CITY,Tokyo
DATE,2023/12/07
TEMPERATURE,23
HUMIDITY,30
NOISE,40
CITY,Beijing
DATE,2023/12/07
TEMPERATURE,24
HUMIDITY,50
NOISE,60
```

**Example: TRANSPOSE with fixed()**

```js
FAKE(csv(`CITY,DATE,TEMPERATURE,HUMIDITY,NOISE
Tokyo,2023/12/07,23,30,40
Beijing,2023/12/07,24,50,60
`))
TRANSPOSE( header(true), fixed(0, 1) )
// Equiv. with
// TRANSPOSE( header(true), 2, 3, 4 )
CSV()
```

It keeps the "fixed" columns for the new records.

```csv
Tokyo,2023/12/07,TEMPERATURE,23
Tokyo,2023/12/07,HUMIDITY,30
Tokyo,2023/12/07,NOISE,40
Beijing,2023/12/07,TEMPERATURE,24
Beijing,2023/12/07,HUMIDITY,50
Beijing,2023/12/07,NOISE,60
```

## FFT()

*Syntax*: `FFT([minHz(value), maxHz(value)])`
- `minHz(value`) *minimum Hz for analysis*
- `maxHz(value`) *maximum Hz for analysis*

It assumes value of the incoming record is an array of *time,amplitude* tuples, then applies *Fast Fourier Transform* on the array and replaces the value with an array of *frequency,amplitude* tuples. The key remains same.

For example, if the incoming record was `{key: k, value[ [t1,a1],[t2,a2],...[tn,an] ]}`, it transforms the value to `{key:k, value[ [F1,A1], [F2,A2],...[Fm,Am] ]}`.

```js
FAKE(
    oscillator(
        freq(15, 1.0), freq(24, 1.5),
        range('now', '10s', '1ms')
    ) 
)
MAPKEY('sample')
GROUPBYKEY()
FFT()
CHART_LINE(
    xAxis(0, 'Hz'),
    yAxis(1, 'Amplitude'),
    dataZoom('slider', 0, 10) 
)
```

Please refer to the FFT() section for the more information including 3D sample codes

## WHEN()

*Syntax*: `WHEN(condition, doer)` (since v8.0.7)

- `condition` *boolean*
- `doer` *doer*

`WHEN` runs `doer` action if the given condition is `true`.
This function does not affects the flow of records, it just executes the defined *side effect* work.

### doLog()

*Syntax*: `doLog(args...)` (since v8.0.7)

Prints out log message on the web console.

```js
FAKE( linspace(1, 2, 2))
WHEN( mod(value(0), 2) == 0, doLog(value(0), "is even."))
CSV()
```

### doHttp()

*Syntax*: `doHttp(method, url, body [, header...])` (since v8.0.7)

- `method` *string*
- `url` *string*
- `body` *string*
- `header` *string* optional

`doHttp` requests the http endpoints with given method, url, body and headers.

**Use cases**

- Notify an event to the specific HTTP endpoint.

```js
FAKE( linspace(1, 4, 4))
WHEN(
    mod(value(0), 2) == 0,
    doHttp("GET", strSprintf("http://127.0.0.1:8888/notify?value=%.0f", value(0)), nil)
)
CSV()
```

- Post the current record to the specific HTTP endpoint in CSV which is default format of `doHttp`.

```js
FAKE( linspace(1, 4, 4))
WHEN(
    mod(value(0), 2) == 0,
    doHttp("POST", "http://127.0.0.1:8888/notify", value())
)
CSV()
```

- Post the current record in a custom JSON format to the specific HTTP endpoint.

```js
FAKE( linspace(1, 4, 4))
WHEN(
    mod(value(0), 2) == 0,
    doHttp("POST", "http://127.0.0.1:8888/notify", 
        strSprintf(`{"message": "even", "value":%f}`, value(0)),
        "Content-Type: application/json",
        "X-Custom-Header: notification"
    )
)
CSV()
```

### do()

*Syntax*: `do(args..., { sub-flow-code })` (since v8.0.7)

`do` executes the given sub flow code with passing `args...` arguments.

It is important to keep in mind that `WHEN()` is only for executing a side effect job on a certain condition.
`WHEN-do` sub flow cannot affects to the main flow, which means it cannot use SINKs that produce result on output stream like `CSV`, `JSON`, and `CHART_*`. The output of a sub flow will be ignored silently, any writing attempts from a sink are ignored and showing warning messages.

Effective SINKs in a sub flow may be `INSERT` and `APPEND` which is not related with output stream, so that it can write the specific values on a different table from main TQL flow. Otherwise use `DISCARD()` sink, it silently discards any records in the sub flow without warning messages.

```js
FAKE( json({
    [ 1, "hello" ],
    [ 2, "你好" ],
    [ 3, "world" ],
    [ 4, "世界" ]
}))
WHEN(
    value(0) % 2 == 0,
    do( "Greetings:", value(0), value(1), {
        ARGS()
        WHEN( true, doLog( value(0), value(2), "idx:", value(1) ) )
        DISCARD()
    })
)
CSV()
```

The log messages of the above code shows the two important points.

1. The main flow is blocked and waits until its sub flow finishes the job.
2. The sub flow is executed every time for a record that matches the condition.

```sh
2023-12-02 07:54:42.160 TRACE 0xc000bfa580 Task compiled FAKE() → WHEN() → CSV()
2023-12-02 07:54:42.160 TRACE 0xc000bfa840 Task compiled ARGS() → WHEN() → DISCARD()
2023-12-02 07:54:42.160 INFO  0xc000bfa840 Greetings: 你好 idx: 2
2023-12-02 07:54:42.160 DEBUG 0xc000bfa840 Task elapsed 254.583µs
2023-12-02 07:54:42.161 TRACE 0xc000bfa9a0 Task compiled ARGS() → WHEN() → DISCARD()
2023-12-02 07:54:42.161 INFO  0xc000bfa9a0 Greetings: 世界 idx: 4
2023-12-02 07:54:42.161 DEBUG 0xc000bfa9a0 Task elapsed 190.552µs
2023-12-02 07:54:42.161 DEBUG 0xc000bfa580 Task elapsed 1.102681ms
```

**Use cases**

When sub flow retrieves data from other than its arguments, it can access the arguments with `args([idx])` option function.

- Execute query with sub flow's arguments.

```js
// pseudo code
// ...
WHEN( condition,
    do(value(0), {
        SQL(`select time, value from table where name = ?`, args(0))
        // ... some map functions...
        INSERT(...)
    })
)
// ...
```

- Retrieve csv file from external web server

```js
// pseudo code
// ...
WHEN( condition,
    do(value(0), value(1), {
        CSV( file( strSprintf("https://exmaple.com/data_%s.csv?id=%s", args(0), escapeParam(args(1)) )))
        WHEN(true, doHttp("POST", "http://my_server", value()))
        DISCARD()
    })
)
// ...
```

## FLATTEN()

*Syntax*: `FLATTEN()`

It works the opposite way of *GROUPBYKEY()*. Take a record whose value is multi-dimension tuple, produces multiple records for each elements of the tuple reducing the dimension.

For example, if an original record was `{key:k, value:[[v1,v2],[v3,v4],...,[vx,vy]]}`, it produces the new multiple records as `{key:k, value:[v1, v2]}`, `{key:k, value:{v3, v4}}`...`{key:k, value:{vx, vy}}`.

## MAPKEY()

*Syntax*: `MAPKEY( newkey )`

Replace current key value with the given newkey.

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
MAPKEY(time("now"))
PUSHKEY("do-not-see")
CSV()
```

```csv
1701343504143299000,TAG0,1628694000000000000,10
1701343504143303000,TAG0,1628780400000000000,11
1701343504143308000,TAG0,1628866800000000000,12
1701343504143365000,TAG0,1628953200000000000,13
1701343504143379000,TAG0,1629039600000000000,14
1701343504143383000,TAG0,1629126000000000000,15
```

## PUSHKEY()

*Syntax*: `PUSHKEY( newkey )`

Apply new key on each record. The original key is push into value tuple.

For example, if an original record was `{key: 'k1', value: [v1, v2]}` and applied `PUSHKEY(newkey)`, it produces the updated record as `{key: newkey, values: [k1, v1, v1]}`.

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
MAPKEY(time("now"))
PUSHKEY("do-not-see")
CSV()
```

```csv
1701343504143299000,TAG0,1628694000000000000,10
1701343504143303000,TAG0,1628780400000000000,11
1701343504143308000,TAG0,1628866800000000000,12
1701343504143365000,TAG0,1628953200000000000,13
1701343504143379000,TAG0,1629039600000000000,14
1701343504143383000,TAG0,1629126000000000000,15
```

## POPKEY()

*Syntax*: `POPKEY( [idx] )`

Drop current key of the record, then promote *idx*th element of *tuple* as a new key.

For example, if an original record was `{key: k, value: [v1, v2, v3]}` and applied `POPKEY(1)`, it produces the updated record as `{key: v2, value:[v1, v3]}`.

if use `POPKEY()` without argument it is equivalent with `POPKEY(0)` which is promoting the first element of the value tuple as the key.

**Example: POPKEY()**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
POPKEY()
CSV()
```

```csv
1628694000000000000,10
1628780400000000000,11
1628866800000000000,12
1628953200000000000,13
1629039600000000000,14
1629126000000000000,15
```

**Example: POPKEY(idx)**

```js
FAKE( json({
    [ "TAG0", 1628694000000000000, 10],
    [ "TAG0", 1628780400000000000, 11],
    [ "TAG0", 1628866800000000000, 12],
    [ "TAG0", 1628953200000000000, 13],
    [ "TAG0", 1629039600000000000, 14],
    [ "TAG0", 1629126000000000000, 15]
}))
POPKEY(1)
CSV()
```

```csv
TAG0,10
TAG0,11
TAG0,12
TAG0,13
TAG0,14
TAG0,15
```

## GROUPBYKEY()

*Syntax*: `GROUPBYKEY( [lazy(boolean)] )`

- `lazy(boolean)` If it set `false` which is default, *GROUPBYKEY()* yields new grouped record when the key of incoming record has changed from previous record. If it set `true`, *GROUPBYKEY()* waits the end of the input stream before yield any record. 

`GROUPBYKEY` is equivalent expression with `GROUP( by( key() ) )`.

## THROTTLE()

*Syntax*: `THROTTLE(tps)` (since v8.0.8)

- `tps` *number* specify in number of records per a second.

`THROTTLE` relays a record to the next step with delay to fit to the specified *tps*.
It makes data flow which has a certain period from stored data (e.g a CSV file), 
so that *simulates* a sensor device that sends measurements by periods.

```js
FAKE(linspace(1,5,5))
THROTTLE(5.0)
WHEN(true, doLog("===>tick", value(0)))
CSV()
```

- At console log, each log time of "tick" message has *200ms.* difference (5 per a second).

```
2023-12-07 09:33:30.131 TRACE 0x14000f88b00 Task compiled FAKE() → THROTTLE() → WHEN() → CSV()
2023-12-07 09:33:30.332 INFO  0x14000f88b00 ===>tick 1
2023-12-07 09:33:30.533 INFO  0x14000f88b00 ===>tick 2
2023-12-07 09:33:30.734 INFO  0x14000f88b00 ===>tick 3
2023-12-07 09:33:30.935 INFO  0x14000f88b00 ===>tick 4
2023-12-07 09:33:31.136 INFO  0x14000f88b00 ===>tick 5
2023-12-07 09:33:31.136 DEBUG 0x14000f88b00
Task elapsed 1.005070167s
```

## SCRIPT()

Supporting user defined script language.

See SCRIPT section for the details with examples.