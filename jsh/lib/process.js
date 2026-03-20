'use strict';

const EventEmitter = require('events');
const _process = require('@jsh/process');

const SUPPORTED_SIGNALS = {
    HUP: 'SIGHUP',
    INT: 'SIGINT',
    QUIT: 'SIGQUIT',
    ABRT: 'SIGABRT',
    KILL: 'SIGKILL',
    USR1: 'SIGUSR1',
    SEGV: 'SIGSEGV',
    USR2: 'SIGUSR2',
    PIPE: 'SIGPIPE',
    ALRM: 'SIGALRM',
    TERM: 'SIGTERM',
};

function normalizeSignalEvent(eventName) {
    if (typeof eventName !== 'string') {
        return eventName;
    }
    const normalized = eventName.trim().toUpperCase();
    if (!normalized) {
        return eventName;
    }
    const rawName = normalized.startsWith('SIG') ? normalized.slice(3) : normalized;
    return SUPPORTED_SIGNALS[rawName] || eventName;
}

class Process extends EventEmitter {
    constructor() {
        super();
        this._watchedSignals = Object.create(null);
        for (const key of Object.keys(_process)) {
            if (key === 'watchSignal') {
                continue;
            }
            if (typeof _process[key] === 'function') {
                // +-- if bind, it causes issues of race conditions
                // |
                // v
                // this[key] = _process[key].bind(_process); 
                this[key] = _process[key];
            } else {
                this[key] = _process[key];
            }
        }
    }

    _ensureSignalWatch(eventName) {
        if (typeof eventName !== 'string') {
            return;
        }
        if (!SUPPORTED_SIGNALS[eventName.slice(3)] || this._watchedSignals[eventName]) {
            return;
        }
        const result = _process.watchSignal(eventName, this);
        if (result instanceof Error) {
            throw result;
        }
        this._watchedSignals[eventName] = true;
    }

    on(eventName, listener) {
        const normalizedEventName = normalizeSignalEvent(eventName);
        this._ensureSignalWatch(normalizedEventName);
        return super.on(normalizedEventName, listener);
    }

    addListener(eventName, listener) {
        const normalizedEventName = normalizeSignalEvent(eventName);
        this._ensureSignalWatch(normalizedEventName);
        return super.addListener(normalizedEventName, listener);
    }

    once(eventName, listener) {
        const normalizedEventName = normalizeSignalEvent(eventName);
        this._ensureSignalWatch(normalizedEventName);
        return super.once(normalizedEventName, listener);
    }

    prependListener(eventName, listener) {
        const normalizedEventName = normalizeSignalEvent(eventName);
        this._ensureSignalWatch(normalizedEventName);
        if (typeof super.prependListener === 'function') {
            return super.prependListener(normalizedEventName, listener);
        }
        return super.on(normalizedEventName, listener);
    }

    prependOnceListener(eventName, listener) {
        const normalizedEventName = normalizeSignalEvent(eventName);
        this._ensureSignalWatch(normalizedEventName);
        if (typeof super.prependOnceListener === 'function') {
            return super.prependOnceListener(normalizedEventName, listener);
        }
        return super.once(normalizedEventName, listener);
    }

    removeListener(eventName, listener) {
        return super.removeListener(normalizeSignalEvent(eventName), listener);
    }

    off(eventName, listener) {
        if (typeof super.off === 'function') {
            return super.off(normalizeSignalEvent(eventName), listener);
        }
        return super.removeListener(normalizeSignalEvent(eventName), listener);
    }

    emit(eventName, ...args) {
        return super.emit(normalizeSignalEvent(eventName), ...args);
    }
}

const p = new Process();
module.exports = p;