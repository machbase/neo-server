# Machbase Neo JavaScript Analysis Module

## sort()

The `sort` function sorts the elements of an array in ascending order.
It is useful for organizing data or preparing it for further analysis.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.sort([1.3, 1.2, 1.1])) // [1.1, 1.2, 1.3]
```

## sum()

The `sum` function calculates the total sum of all numbers in an array.
It is commonly used in statistical and mathematical computations.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.sum([3, 1, 2]))       // 6
console.log(ana.sum([1.3, 1.2, 1.1])) // 3.6
```

## cdf()

The `cdf` function calculates the cumulative distribution function (CDF) for a given dataset `x` 
that is the fraction of the samples less than or equal to `q`.
It represents the probability that a random variable takes on a value less than or equal to a specified value. 
This function is commonly used in statistical analysis and probability theory to understand the distribution of data.

**Syntax**

```js
cdf(q, x, weights)
```

- `q` `Number`
- `x` `Number[]` The `x` data must be sorted in increasing order.
- `weights` `Number[]` If weights is not specified then all of the weights are 1. If weights is specified, then length of `x` must equal length of `weights`.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [];
for( i=1; i<=100; i++) {
    x.push(i);
}
x = ana.sort(x);
console.log(ana.cdf(1.0, x)); // 0.01
```

## mean()

The `mean` function calculates the arithmetic mean (average) of a given array of numbers. 
It is computed by summing all the elements in the array and dividing by the total number of elements. 
This function is commonly used in statistical analysis to determine the central tendency of a dataset.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.mean([1, 2, 3, 4, 5]))  // 3
console.log(ana.mean([10, 20, 30]))     // 20
```

## circularMean()

The `circularMean` function calculates the mean of angles measured in radians, taking into account the circular nature of angles. 
It is particularly useful for datasets where values wrap around, such as angles or time of day. 
Optionally, weights can be provided to compute a weighted circular mean.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [0, 0.25 * Math.PI, 0.75 * Math.PI];
w = [1, 2, 2.5];
console.log(ana.circularMean(x).toFixed(4))     // 0.9553
console.log(ana.circularMean(x, w).toFixed(4))  // 1.3704
```

## correlation()

The `correlation` function calculates the Pearson correlation coefficient between two datasets.
It measures the linear relationship between the datasets,
with values ranging from -1 (perfect negative correlation) to 1 (perfect positive correlation).
Optionally, weights can be provided to compute a weighted correlation.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [8, -3, 7, 8, -4];
y = [10, 5, 6, 3, -1];
w = [2, 1.5, 3, 3, 2];
console.log(ana.correlation(x, y).toFixed(5))     // 0.61922
console.log(ana.correlation(x, y, w).toFixed(5))  // 0.59915
```

## covariance()

The `covariance` function calculates the covariance between two datasets.
Covariance is a measure of how much two random variables vary together.
A positive covariance indicates that the variables tend to increase together,
while a negative covariance indicates that one variable tends to increase as the other decreases.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [8, -3, 7, 8, -4];
y1 = [10, 2, 2, 4, 1];
y2 = [12, 1, 11, 12, 0];
console.log(ana.covariance(x, y1).toFixed(4)) // 13.8000
console.log(ana.covariance(x, y2).toFixed(4)) // 37.7000
console.log(ana.variance(x).toFixed(4))       // 37.7000
```

## entropy()

The `entropy` function calculates the Shannon entropy of a probability distribution.
Entropy is a measure of uncertainty or randomness in the distribution.
It is commonly used in information theory and statistics.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.entropy([0.05, 0.1, 0.9, 0.05]).toFixed(4)); // 0.6247
console.log(ana.entropy([0.2, 0.4, 0.25, 0.15]).toFixed(4)); // 1.3195
console.log(ana.entropy([0.2, 0, 0, 0.5, 0, 0.2, 0.1, 0, 0, 0]).toFixed(4)); // 1.2206
console.log(ana.entropy([0, 0, 1, 0]).toFixed(4));           // 0.0000
```

## geometricMean()

The `geometricMean` function calculates the geometric mean of a given array of positive numbers. 
It is computed by multiplying all the elements in the array and then taking the nth root, where n is the total number of elements. 
This function is commonly used in financial and statistical analysis to determine the average rate of return or growth.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.geometricMean([1, 3, 9]).toFixed(4))  // 3.0000
console.log(ana.geometricMean([2, 8, 32]).toFixed(4)) // 8.0000
```

## harmonicMean()

The `harmonicMean` function calculates the harmonic mean of a given array of positive numbers. 
It is computed as the reciprocal of the arithmetic mean of the reciprocals of the elements. 
This function is particularly useful for datasets involving rates or ratios, such as speeds or densities.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.harmonicMean([1, 2, 4]).toFixed(4))    // 1.7143
console.log(ana.harmonicMean([10, 20, 30]).toFixed(4)) // 16.3636
```

## median()

The `median` function calculates the median of a given array of numbers. 
The median is the middle value when the numbers are sorted in ascending order. 
If the array has an even number of elements, the median is the average of the two middle values. 
This function is commonly used in statistical analysis to determine the central value of a dataset.

The input array should be sorted, otherwise it throws exception.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.median(ana.sort([1, 3, 2, 5, 4])))      // 3
console.log(ana.median(ana.sort([10, 20, 30, 40, 50]))) // 30
```

## medianInterp()

The `medianInterp` function is same as `median` except it returns the linear interpolated value.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.medianInterp(ana.sort([1, 3, 2, 5, 4])))      // 2.5
console.log(ana.medianInterp(ana.sort([10, 20, 30, 40, 50]))) // 25
```

## quantile()

The `quantile` function calculates the quantile of a given dataset for a specified probability. 
Quantiles divide the dataset into intervals with equal probabilities, such as quartiles (4 intervals) or percentiles (100 intervals). 
This function is useful for understanding the distribution of data.

**Syntax**

```js
quantile(p, x, weights)
```

- `p` `Number`
- `x` `Number[]` The `x` data must be sorted in increasing order.
- `weights` `Number[]` If weights is not specified then all of the weights are 1. If weights is specified, then length of `x` must equal length of `weights`.

**Usage example**

```js
const ana = require("@jsh/analysis")
data = ana.sort([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
console.log(ana.quantile(0.25, data)) // 3
console.log(ana.quantile(0.5, data))  // 5
console.log(ana.quantile(0.74, data)) // 8
```

## quantileInterp()

The `quantileInterp` function is same as `quantile` except it returns the linear interpolated value.

**Syntax**

```js
quantileInterp(p, x, weights)
```

- `p` `Number`
- `x` `Number[]` The `x` data must be sorted in increasing order.
- `weights` `Number[]` If weights is not specified then all of the weights are 1. If weights is specified, then length of `x` must equal length of `weights`.

**Usage example**

```js
const ana = require("@jsh/analysis")
data = ana.sort([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
console.log(ana.quantileInterp(0.25, data)) // 2.5
console.log(ana.quantileInterp(0.5, data))  // 5
console.log(ana.quantileInterp(0.74, data)) // 7.4
```

## meanStdDev()

The `meanStdDev` function calculates both the mean and the standard deviation of a given array of numbers. 
The mean represents the central tendency, while the standard deviation measures the spread or dispersion of the data. 
This function is useful for summarizing datasets in statistical analysis.

**Usage example**

```js
const ana = require("@jsh/analysis")
data = [1, 2, 3, 4, 5]
result = ana.meanStdDev(data)
console.log(result.mean.toFixed(2))   // 3.00
console.log(result.stdDev.toFixed(2)) // 1.58
```

## mode()

The `mode` function calculates the mode of a given array of numbers. 
The mode is the value that appears most frequently in the dataset. 
If there are multiple modes, the function may return all of them or handle it based on implementation.

It returns `{value: number, count: number}`.

**Syntax**

```js
mode(x, weights)
```

- `x` `Number[]` The `x` data must be sorted in increasing order.
- `weights` `Number[]` If weights is not specified then all of the weights are 1. If weights is specified, then length of `x` must equal length of `weights`.

**Usage example**

```js
const ana = require("@jsh/analysis")
data = ana.sort([1, 2, 2, 3, 4])
console.log(ana.mode(data)) // {value:2, count:2}
data = ana.sort([1, 1, 2, 3, 4])
console.log(ana.mode(data)) // {value:1, count:2}
```

## moment()

The `moment` function calculates the nth moment of a dataset about a specified point. 
Moments are used in statistics to describe the shape of a distribution, such as skewness (3rd moment) or kurtosis (4th moment).

**Usage example**

```js
const ana = require("@jsh/analysis")
data = [1, 2, 3, 4, 5]
console.log(ana.moment(2, data).toFixed(4)) // 2.5000
console.log(ana.moment(4, data).toFixed(4)) // 6.8000
```

## stdDev()

The `stdDev` function calculates the standard deviation of a given array of numbers. 
Standard deviation measures the amount of variation or dispersion in a dataset. 
A low standard deviation indicates that the data points are close to the mean, while a high standard deviation indicates greater spread.

**Usage example**

```js
const ana = require("@jsh/analysis")
console.log(ana.stdDev([1, 2, 3, 4, 5]).toFixed(4))      // 1.5811
console.log(ana.stdDev([10, 20, 30, 40, 50]).toFixed(4)) // 15.8114
```

## stdErr()

The `stdErr` function calculates the standard error of the mean for a given array of numbers. 
The standard error measures the accuracy with which a sample mean represents the population mean. 
It is computed as the standard deviation divided by the square root of the sample size.

**Usage example**

```js
const ana = require("@jsh/analysis")
let stddev = ana.stdDev([1, 2, 3, 4, 5])
let sampleSize = 5
console.log(ana.stdErr(stddev, sampleSize).toFixed(4)) // 0.7071
```

## linearRegression()

The `linearRegression` function performs a linear regression analysis on two datasets. 
It calculates the best-fit line that minimizes the sum of squared residuals between the observed and predicted values. 
This function is commonly used in predictive modeling and trend analysis.

It returns `{slope: alpha, intercept: beta}` where `y = alpha*x + beta`.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4, 5]
y = [2, 4, 6, 8, 10]
result = ana.linearRegression(x, y)
console.log(result.slope.toFixed(4))     // 2.0000
console.log(result.intercept.toFixed(4)) // 0.0000
```

## fft()

The `fft` function performs a Fast Fourier Transform (FFT) on a given dataset. 
FFT is used to analyze the frequency components of a signal, making it useful in signal processing and data analysis.

```js
fft(times, amplitudes)
```

The length of times and amplitudes should be equal.

## PiecewiseConstant

The `PiecewiseConstant` performs piecewise constant interpolation on a dataset. 
It approximates the value of a function by using the nearest data point in each interval. 
This method is useful for step-like data.

**Usage example**

```js
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.PiecewiseConstant();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 30
```

## PiecewiseLinear

The `PiecewiseLinear` performs piecewise linear interpolation on a dataset. 
It approximates the value of a function by connecting data points with straight lines. 
This method is useful for smooth transitions between data points.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.PiecewiseLinear();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## AkimaSpline

The `AkimaSpline` performs Akima spline interpolation on a dataset. 
This method creates a smooth curve that passes through the data points, avoiding oscillations in regions with sparse data.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.AkimaSpline();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## FritschButland

The `FritschButland` performs Fritsch-Butland interpolation on a dataset. 
This method ensures monotonicity in the interpolated values, making it suitable for datasets where preserving order is important.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.FritschButland();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## LinearRegression

The `LinearRegression` performs linear regression-based interpolation on a dataset. 
It predicts the value of a function at a given point using the best-fit line derived from the data.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.LinearRegression();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## ClampedCubic

The `ClampedCubic` performs linear clamped-cubic interpolation on a dataset. 
It predicts the value of a function at a given point using the best-fit line derived from the data.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.ClampedCubic();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## NaturalCubic

The `NaturalCubic` performs linear natural-cubic interpolation on a dataset. 
It predicts the value of a function at a given point using the best-fit line derived from the data.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.NaturalCubic();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```

## NotAKnotCubic

The `NotAKnotCubic` performs linear not-a-knot cubic spline interpolation on a dataset. 
It predicts the value of a function at a given point using the best-fit line derived from the data.

**Usage example**

```js
const ana = require("@jsh/analysis")
x = [1, 2, 3, 4]
y = [10, 20, 30, 40]
interp = new ana.NotAKnotCubic();
interp.fit(x,y);
console.log(interp.predict(2.5)); // 25
```