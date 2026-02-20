'use strict';

const EventEmitter = require('events');
const _process = require('@jsh/process');

class Process extends EventEmitter {
    constructor() {
        super();
        for (const key of Object.keys(_process)) {
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
}

const p = new Process();
module.exports = p;