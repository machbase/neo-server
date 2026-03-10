'use strict';

const _filter = require('@jsh/mathx/filter');

function isNumber(value) {
    return typeof value === 'number';
}

function isOptionsObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function isValidDate(value) {
    return value instanceof Date && !Number.isNaN(value.getTime());
}

function validateEvalNumber(name, args) {
    if (args.length === 0) {
        throw new Error(`${name}: no argument`);
    }
    const value = args[0];
    if (!isNumber(value)) {
        throw new Error(`${name}: invalid argument`);
    }
}

function validateKalmanCtorArgs(args) {
    if (args.length === 1) {
        if (!isOptionsObject(args[0])) {
            throw new Error('kalman: invalid argument');
        }
        const { initialVariance, processVariance, observationVariance } = args[0];
        if ((initialVariance !== undefined && !isNumber(initialVariance)) ||
            (processVariance !== undefined && !isNumber(processVariance)) ||
            (observationVariance !== undefined && !isNumber(observationVariance))) {
            throw new Error('kalman: invalid argument');
        }
        return { initialVariance, processVariance, observationVariance };
    }
    if (args.length === 3) {
        if (!isNumber(args[0]) || !isNumber(args[1]) || !isNumber(args[2])) {
            throw new Error('kalman: invalid argument');
        }
        return { initialVariance: args[0], processVariance: args[1], observationVariance: args[2] };
    }
    throw new Error('kalman: invalid arguments');
}

function validateKalmanEvalArgs(name, args) {
    if (args.length < 2) {
        throw new Error(`${name}: invalid arguments`);
    }
    if (!isValidDate(args[0])) {
        throw new Error(`${name}: invalid argument`);
    }
    for (let i = 1; i < args.length; i++) {
        if (!isNumber(args[i])) {
            throw new Error(`${name}: invalid argument`);
        }
    }
}

class Avg {
    constructor() {
        this._impl = _filter.Avg();
    }

    eval(...args) {
        validateEvalNumber('avg', args);
        return this._impl.eval(...args);
    }
}

class MovAvg {
    constructor(windowSize) {
        if (arguments.length === 0) {
            throw new Error('movavg: no argument');
        }
        if (!isNumber(windowSize)) {
            throw new Error('movavg: invalid argument');
        }
        if (windowSize <= 1) {
            throw new Error('movavg: windowSize should be larger than 1');
        }
        this._impl = _filter.MovAvg(windowSize);
    }

    eval(...args) {
        validateEvalNumber('movavg', args);
        return this._impl.eval(...args);
    }
}

class Lowpass {
    constructor(alpha) {
        if (arguments.length === 0) {
            throw new Error('lowpass: no argument');
        }
        if (!isNumber(alpha)) {
            throw new Error('lowpass: invalid argument');
        }
        if (alpha <= 0 || alpha >= 1) {
            throw new Error('lowpass: alpha should be 0 < alpha < 1');
        }
        this._impl = _filter.Lowpass(alpha);
    }

    eval(...args) {
        validateEvalNumber('lowpass', args);
        return this._impl.eval(...args);
    }
}

class Kalman {
    constructor(...args) {
        let variances = validateKalmanCtorArgs(args);
        this._impl = _filter.Kalman(variances.initialVariance, variances.processVariance, variances.observationVariance);
    }

    eval(...args) {
        validateKalmanEvalArgs('kalman', args);
        let result = this._impl.eval(...args);
        if (result && result.length == 1) {
            return result[0];
        }
        return result;
    }
}

class KalmanSmoother {
    constructor(...args) {
        let variances = validateKalmanCtorArgs(args);
        this._impl = _filter.KalmanSmoother(variances.initialVariance, variances.processVariance, variances.observationVariance);
    }

    eval(...args) {
        validateKalmanEvalArgs('kalman', args);
        let result = this._impl.eval(...args);
        if (result && result.length == 1) {
            return result[0];
        }
        return result;
    }
}

module.exports = {
    Avg,
    Kalman,
    KalmanSmoother,
    Lowpass,
    MovAvg,
};