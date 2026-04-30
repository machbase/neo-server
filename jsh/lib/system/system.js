'use strict';

const _system = require('@jsh/system');

function statz(series, ...name) {
    return _system.statz(series, ...name);
}

function now() {
    return _system.now();
}

function timeLocation(loc) {
    if (loc === undefined || loc === null) {
        loc = "Local";
    }
    return _system.timeLocation(loc);
}

function free_os_memory() {
    return _system.free_os_memory();
}

function gc() {
    return _system.gc();
}

module.exports = {
    free_os_memory,
    gc,
    statz,
    now,
    timeLocation,
};