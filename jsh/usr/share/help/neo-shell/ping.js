'use strict';

const { simple } = require('help/config');

module.exports = simple('ping', 'Ping a database server', 'Usage: ping [options]', {
    allowNegative: true,
    options: { repeat: { type: 'integer', short: 'n', description: 'Number of times to repeat the ping', default: '1' } },
});