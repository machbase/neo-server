# Machbase Neo JavaScript Filter Module

## Avg

**Usage example**

```js
const { arrange } = require("@jsh/generator");
const m = require("@jsh/filter")
const avg = new m.Avg();
for( x of arrange(10, 30, 10) ) {
    console.log(x,  avg.eval(x).toFixed(2));
}

// 10 10.00
// 20 15.00
// 30 20.00
```

## MovAvg

**Usage example**

```js
const { linspace } = require("@jsh/generator");
const m = require("@jsh/filter")
const movAvg = new m.MovAvg(10);
for( x of linspace(0, 100, 100) ) {
    console.log(""+x.toFixed(4)+","+movAvg.eval(x).toFixed(4));
}

// 0.0000,0.0000
// 1.0101,0.5051
// 2.0202,1.0101
// 3.0303,1.5152
// ... omit ...
// 96.9697,92.4242
// 97.9798,93.4343
// 98.9899,94.4444
// 100.0000,95.4545
```

## Lowpass

**Usage example**

```js
const { arrange, Simplex } = require("@jsh/generator");
const m = require("@jsh/filter")
const lpf = new m.Lowpass(0.3);
const simplex = new Simplex(1);

for( x of arrange(1, 10, 1) ) {
    v = x + simplex.eval(x) * 3;
    console.log(x, v.toFixed(2), lpf.eval(v).toFixed(2));
}

// 1 1.48 1.48
// 2 0.40 1.15
// 3 3.84 1.96
// 4 2.89 2.24
// 5 5.47 3.21
// 6 5.29 3.83
// 7 7.22 4.85
// 8 10.31 6.49
// 9 8.36 7.05
// 10 8.56 7.50
```

## Kalman

**Usage example**

```js
const m = require("@jsh/filter");
const kalman = new m.Kalman(1.0, 1.0, 2.0);
var ts = 1745484444000; // ms

for( x of [1.3, 10.2, 5.0, 3.4] ) {
    ts += 1000; // add 1 sec.
    console.log(kalman.eval(new Date(ts), x).toFixed(3));
}

// 1.300
// 5.750
// 5.375
// 4.388
```

## KalmanSmoother

**Usage example**

```js
const m = require("@jsh/filter");
const kalman = new m.KalmanSmoother(1.0, 1.0, 2.0);
var ts = 1745484444000; // ms
for( x of [1.3, 10.2, 5.0, 3.4] ) {
    ts += 1000; // add 1 sec.
    console.log(kalman.eval(new Date(ts), x).toFixed(2));
}

// 1.30
// 5.75
// 3.52
// 2.70
```

## Kalman Filter vs. Smoother

```js
SCRIPT({
    const { now } = require("@jsh/system");
    const { arrange, Simplex } = require("@jsh/generator");
    const m = require("@jsh/filter")
    const simplex = new Simplex(1234);
    const kalmanFilter = new m.Kalman(0.1, 0.001, 1.0);
    const kalmanSmoother = new m.KalmanSmoother(0.1, 0.001, 1.0);
    const real = 14.4;
}, {
    s_x = []; s_values = []; s_filter = []; s_smooth = [];
    for( x of arrange(0, 10, 0.1)) {
        x = Math.round(x*100)/100;
        measure = real + simplex.eval(x) * 4;
        s_x.push(x);
        s_values.push(measure);
        s_filter.push(kalmanFilter.eval(now(), measure));
        s_smooth.push(kalmanSmoother.eval(now(), measure));
    }
    $.yield({
        title:{text:"Kalman filter vs. smoother"},
        xAxis:{type:"category", data:s_x},
        yAxis:{ min:10, max: 18 },
        series:[
            {type:"line", data:s_values, name:"values"},
            {type:"line", data:s_filter, name:"filter"},
            {type:"line", data:s_smooth, name:"smoother"}
        ],
        tooltip: {show: true, trigger:"axis"},
        legend: { bottom: 10},
        animation: false
    });
})
CHART(size("600px", "400px"))
```