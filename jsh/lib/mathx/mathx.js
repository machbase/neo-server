'use strict';

const _mathx = require('@jsh/mathx');

function hasValue(value) {
    return value !== undefined && value !== null;
}

function validateArrayLength(name, x, weight) {
    if (hasValue(weight) && x.length !== weight.length) {
        throw new Error(`${name}: x and weight should be the same length`);
    }
}

function validatePairArrayLength(name, x, y, weight) {
    if (x.length !== y.length) {
        throw new Error(`${name}: x and y should be the same length`);
    }
    if (hasValue(weight) && x.length !== weight.length) {
        throw new Error(`${name}: x, y and weight should be the same length`);
    }
}

function arrange(start, stop, step) {
    if (step === 0) {
        throw new Error('arrange: step must not be 0');
    }
    if (start === stop) {
        throw new Error('arrange: start and stop must not be equal');
    }
    if (start < stop && step < 0) {
        throw new Error('arrange: step must be positive');
    }
    if (start > stop && step > 0) {
        throw new Error('arrange: step must be negative');
    }
    const length = Math.trunc(Math.abs((stop - start) / step)) + 1;
    const arr = new Array(length);
    for (let i = 0; i < length; i++) {
        arr[i] = start + i * step;
    }
    return arr;
}

function linspace(start, stop, count) {
    if (count <= 0) {
        return [];
    }
    if (count === 1) {
        return [start];
    }
    const step = (stop - start) / (count - 1);
    const arr = new Array(count);
    for (let i = 0; i < count; i++) {
        arr[i] = start + i * step;
    }
    return arr;
}

function meshgrid(arr1, arr2) {
    const lenX = arr1.length;
    const lenY = arr2.length;
    const arr = new Array(lenX * lenY);
    for (let x = 0; x < lenX; x++) {
        for (let y = 0; y < lenY; y++) {
            arr[x * lenY + y] = [arr1[x], arr2[y]];
        }
    }
    return arr;
}

function cdf(q, x, weight) {
    validateArrayLength('cdf', x, weight);
    return _mathx.cdf(q, x, weight);
}

function circularMean(x, weight) {
    validateArrayLength('circularMean', x, weight);
    return _mathx.circularMean(x, weight);
}

function correlation(x, y, weight) {
    validatePairArrayLength('correlation', x, y, weight);
    return _mathx.correlation(x, y, weight);
}

function covariance(x, y, weight) {
    validatePairArrayLength('covariance', x, y, weight);
    return _mathx.covariance(x, y, weight);
}

function geometricMean(x, weight) {
    validateArrayLength('geometricMean', x, weight);
    return _mathx.geometricMean(x, weight);
}

function mean(x, weight) {
    validateArrayLength('mean', x, weight);
    return _mathx.mean(x, weight);
}

function harmonicMean(x, weight) {
    validateArrayLength('harmonicMean', x, weight);
    return _mathx.harmonicMean(x, weight);
}

function median(x, weight) {
    validateArrayLength('median', x, weight);
    return _mathx.median(x, weight);
}

function medianInterp(x, weight) {
    validateArrayLength('median', x, weight);
    return _mathx.medianInterp(x, weight);
}

function variance(x, weight) {
    validateArrayLength('variance', x, weight);
    return _mathx.variance(x, weight);
}

function meanVariance(x, weight) {
    validateArrayLength('meanVariance', x, weight);
    return _mathx.meanVariance(x, weight);
}

function stdDev(x, weight) {
    validateArrayLength('stdDev', x, weight);
    return _mathx.stdDev(x, weight);
}

function meanStdDev(x, weight) {
    validateArrayLength('meanStdDev', x, weight);
    return _mathx.meanStdDev(x, weight);
}

function sort(arr) {
    return _mathx.sort(arr);
}

function sum(arr) {
    return _mathx.sum(arr);
}

function entropy(p) {
    return _mathx.entropy(p);
}

function stdErr(std, sampleSize) {
    return _mathx.stdErr(std, sampleSize);
}

function mode(arr) {
    return _mathx.mode(arr);
}

function moment(momentValue, arr) {
    return _mathx.moment(momentValue, arr);
}

function quantile(p, arr) {
    return _mathx.quantile(p, arr);
}

function quantileInterp(p, arr) {
    return _mathx.quantileInterp(p, arr);
}

function linearRegression(x, y) {
    return _mathx.linearRegression(x, y);
}

function fft(times, values) {
    return _mathx.fft(times, values);
}

module.exports = {
    arrange,
    cdf,
    circularMean,
    correlation,
    covariance,
    entropy,
    fft,
    geometricMean,
    linspace,
    harmonicMean,
    linearRegression,
    mean,
    meanStdDev,
    meanVariance,
    median,
    medianInterp,
    meshgrid,
    mode,
    moment,
    quantile,
    quantileInterp,
    sort,
    stdDev,
    stdErr,
    sum,
    variance,
};