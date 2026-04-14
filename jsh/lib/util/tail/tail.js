'use strict';

const _tail = require('@jsh/util/tail');
const sse = require('./tail/sse.js');

function create(path, options) {
    if (!path || typeof path !== 'string') {
        throw new Error('tail.create(path, options): path must be a non-empty string');
    }
    const nativeTail = _tail.create(path, options || {});
    return {
        path: nativeTail.path,
        poll: function (callback) {
            if (callback !== undefined && callback !== null && typeof callback !== 'function') {
                throw new Error('tail.poll(callback): callback must be a function');
            }
            return nativeTail.poll(callback);
        },
        close: function () {
            nativeTail.close();
        },
    };
}

module.exports = {
    create,
    sse,
};
