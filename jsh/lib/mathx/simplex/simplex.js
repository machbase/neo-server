'use strict';

const _simplex = require('@jsh/mathx/simplex');

class Simplex {
    constructor(seed = 0) {
        this._gen = _simplex.seed(seed);
    }

    noise(...args) {
        return this._gen.eval(...args);
    }
}

module.exports = {
    Simplex,
};
