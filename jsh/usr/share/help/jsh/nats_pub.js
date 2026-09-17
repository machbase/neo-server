'use strict';
const { simple } = require('help/config');
module.exports = simple('nats_pub', 'Publish a message to a NATS subject', 'Usage: nats_pub [options]', {
    options: {
        debug: { type: 'boolean', short: 'd', description: 'Enable debug mode', default: false },
        topic: { type: 'string', short: 't', description: 'Subject to publish to', default: '' },
        subject: { type: 'string', short: 's', description: 'Subject to publish to', default: '' },
        broker: { type: 'string', short: 'b', description: 'NATS broker address', default: 'nats://localhost:4222' },
        message: { type: 'string', short: 'm', description: 'Message to publish', default: '' },
        file: { type: 'string', short: 'f', description: 'File containing message to publish', default: '' },
        reply: { type: 'string', short: 'r', description: 'Reply subject to wait for', default: '' },
        request: { type: 'boolean', description: 'Generate a temporary reply subject and wait for one response', default: false },
        timeout: { type: 'integer', description: 'Timeout in milliseconds for connect and reply wait', default: 10000 },
    },
});