'use strict';
const { simple } = require('help/config');
module.exports = simple('mqtt_pub', 'Publish a message to an MQTT topic', 'Usage: mqtt_pub [options]', {
    options: {
        debug: { type: 'boolean', short: 'd', description: 'Enable debug mode', default: false },
        topic: { type: 'string', short: 't', description: 'Topic to publish to', default: '' },
        broker: { type: 'string', short: 'b', description: 'MQTT broker address', default: 'tcp://localhost:5653' },
        message: { type: 'string', short: 'm', description: 'Message to publish', default: '' },
        file: { type: 'string', short: 'f', description: 'File containing message to publish', default: '' },
        qos: { type: 'integer', short: 'q', description: 'QoS level for MQTT message (0, 1, or 2)', default: 0 },
    },
});