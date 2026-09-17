'use strict';
const { simple } = require('help/config');
module.exports = simple('sleep', 'Pause execution', 'Usage: sleep [options] <sec...>', {
    positionals: [{ name: 'sec', type: 'integer', variadic: true, description: 'Number of seconds to sleep' }],
});