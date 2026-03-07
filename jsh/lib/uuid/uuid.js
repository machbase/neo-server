'use strict';

const _uuid = require('@jsh/uuid');

class UUID {
    constructor(defaultVersion = 4) {
        this._gen = _uuid.generator();
        this._defaultVersion = defaultVersion;
    }
    newV1() {
        return this._gen.newV1();
    }
    newV4() {
        return this._gen.newV4();
    }
    newV6() {
        return this._gen.newV6();
    }
    newV7() {
        return this._gen.newV7();
    }
    /**
     * Generate a new UUID of the specified version. 
     * If no version is specified, the default version is used. 
     * Supported versions are 1, 4, 6, and 7.
     * @param {number} v - The version of UUID to generate (1, 4, 6, or 7). If not specified, the default version is used.
     * @returns {string} The generated UUID string.
    */
    new(v = this._defaultVersion) {
        switch (v) {
            case 1:
                return this._gen.newV1();
            case 4:
                return this._gen.newV4();
            case 6:
                return this._gen.newV6();
            case 7:
                return this._gen.newV7();
            default:
                throw new Error(`unsupported uuid version: ${v}`);
        }
    }
}

module.exports = {
    UUID,
};
