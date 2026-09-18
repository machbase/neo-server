'use strict';
const { simple } = require('help/config');
module.exports = simple('ps', 'List JSH process entries under /proc/process', 'Usage: ps [options]', {
    options: { json: { type: 'boolean', short: 'j', description: 'Print process entries as JSON', default: false } },
});