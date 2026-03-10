'use strict';

const _mat = require('@jsh/mathx/mat');

function isNumber(value) {
    return typeof value === 'number';
}

function isInteger(value) {
    return Number.isInteger(value);
}

function validateDimension(name, value, label) {
    if (!isInteger(value)) {
        throw new Error(`${name}: ${label} should be an integer`);
    }
    if (value < 0) {
        throw new Error(`${name}: ${label} should be non-negative`);
    }
}

function validateNumericArray(name, data, expectedLength) {
    if (data === undefined || data === null) {
        return;
    }
    if (!Array.isArray(data)) {
        throw new Error(`${name}: data should be an array`);
    }
    for (const value of data) {
        if (!isNumber(value)) {
            throw new Error(`${name}: data should contain only numbers`);
        }
    }
    if (expectedLength !== undefined && data.length !== expectedLength) {
        throw new Error(`${name}: data length should be ${expectedLength}`);
    }
}

function Dense(rows, cols, data) {
    if (arguments.length > 0) {
        validateDimension('Dense', rows, 'rows');
    }
    if (arguments.length > 1) {
        validateDimension('Dense', cols, 'cols');
    }
    if (arguments.length > 2) {
        validateNumericArray('Dense', data, rows * cols);
    }
    if (arguments.length === 2) {
        return _mat.Dense(rows, cols, null);
    } else if (arguments.length === 3) {
        return _mat.Dense(rows, cols, data);
    } else {
        return _mat.Dense(0, 0, null);
    }
}

function SymDense(n, data) {
    if (arguments.length > 0) {
        validateDimension('SymDense', n, 'size');
    }
    if (arguments.length > 1) {
        validateNumericArray('SymDense', data, n * n);
    }
    return _mat.SymDense(n, data);
}

function VecDense(n, data) {
    if (arguments.length > 0) {
        validateDimension('VecDense', n, 'size');
    }
    if (arguments.length > 1) {
        validateNumericArray('VecDense', data, n);
    }
    return _mat.VecDense(n, data);
}

function QR() {
    return _mat.QR(...arguments);
}

function format(matrix, opts) {
    if (arguments.length === 0) {
        return undefined;
    }
    // if (!matrix || typeof matrix !== 'object' || !('$' in matrix)) {
    //     return undefined;
    // }

    if (arguments.length > 1 && opts !== undefined && opts !== null) {
        if (typeof opts !== 'object' || Array.isArray(opts)) {
            throw new Error('format: invalid options');
        }
        if (opts.format !== undefined && typeof opts.format !== 'string') {
            throw new Error('format: invalid options');
        }
        if (opts.prefix !== undefined && typeof opts.prefix !== 'string') {
            throw new Error('format: invalid options');
        }
        if (opts.excerpt !== undefined && !isInteger(opts.excerpt)) {
            throw new Error('format: invalid options');
        }
        if (opts.squeeze !== undefined && typeof opts.squeeze !== 'boolean') {
            throw new Error('format: invalid options');
        }
    }
    return _mat.format(matrix, opts);
}

module.exports = {
    Dense,
    format,
    QR,
    SymDense,
    VecDense,
};