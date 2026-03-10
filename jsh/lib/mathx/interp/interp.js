'use strict';

const _interp = require('@jsh/mathx/interp');

function isNumber(value) {
    return typeof value === 'number';
}

function validateSeries(value, name) {
    if (!Array.isArray(value)) {
        throw new Error(`fit: ${name} should be an array`);
    }
    for (const item of value) {
        if (!isNumber(item)) {
            throw new Error(`fit: ${name} should contain only numbers`);
        }
    }
}

function validateFitArgs(args) {
    if (args.length !== 2) {
        throw new Error('fit: x and y are required');
    }
    const [xs, ys] = args;
    validateSeries(xs, 'x');
    validateSeries(ys, 'y');
    if (xs.length !== ys.length) {
        throw new Error('fit: x and y should be the same length');
    }
}

function validatePredictArgs(name, args) {
    if (args.length !== 1) {
        throw new Error(`${name}: x is required`);
    }
    if (!isNumber(args[0])) {
        throw new Error(`${name}: invalid argument`);
    }
}

function createInterpolatorWrapper(Impl) {
    return class InterpolatorWrapper {
        constructor() {
            this._impl = Impl();
            if (typeof this._impl.predictDerivative !== 'function') {
                this.predictDerivative = undefined;
            }
        }

        fit(...args) {
            validateFitArgs(args);
            return this._impl.fit(...args);
        }

        predict(...args) {
            validatePredictArgs('predict', args);
            return this._impl.predict(...args);
        }

        predictDerivative(...args) {
            validatePredictArgs('predictDerivative', args);
            if (typeof this._impl.predictDerivative !== 'function') {
                throw new Error('predictDerivative: not supported');
            }
            return this._impl.predictDerivative(...args);
        }
    };
}

const PiecewiseConstant = createInterpolatorWrapper(_interp.PiecewiseConstant);
const PiecewiseLinear = createInterpolatorWrapper(_interp.PiecewiseLinear);
const AkimaSpline = createInterpolatorWrapper(_interp.AkimaSpline);
const FritschButland = createInterpolatorWrapper(_interp.FritschButland);
const LinearRegression = createInterpolatorWrapper(_interp.LinearRegression);
const ClampedCubic = createInterpolatorWrapper(_interp.ClampedCubic);
const NaturalCubic = createInterpolatorWrapper(_interp.NaturalCubic);
const NotAKnotCubic = createInterpolatorWrapper(_interp.NotAKnotCubic);

module.exports = {
    AkimaSpline,
    ClampedCubic,
    FritschButland,
    LinearRegression,
    NaturalCubic,
    NotAKnotCubic,
    PiecewiseConstant,
    PiecewiseLinear,
};