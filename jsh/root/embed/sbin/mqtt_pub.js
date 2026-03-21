'use strict';

const process = require('process');
const fs = require('fs');
const mqtt = require('mqtt');
const parseArgs = require('util/parseArgs');

const options = {
    help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    debug: { type: 'boolean', short: 'd', description: "Enable debug mode", default: false },
    topic: { type: 'string', short: 't', description: "Topic to publish to", default: '' },
    broker: { type: 'string', short: 'b', description: "MQTT broker address", default: 'tcp://localhost:5653' },
    message: { type: 'string', short: 'm', description: "Message to publish", default: '' },
    file: { type: 'string', short: 'f', description: "File containing message to publish", default: '' },
    qos: { type: 'integer', short: 'q', description: "QoS level for MQTT message (0, 1, or 2)", default: 0 },
}

let showHelp = true;
let config = {};
try {
    const parsed = parseArgs(process.argv.slice(2), {
        options,
    });
    config = parsed.values;
    showHelp = config.help
}
catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(parseArgs.formatHelp({
        usage: 'Usage: mqtt_pub [options]',
        options,
    }));
    process.exit(showHelp ? 0 : 1);
}

function fail(message) {
    console.println(message);
    process.exit(1);
}

function debug(...args) {
    if (config.debug) {
        console.println(...args);
    }
}

function resolvePath(path) {
    if (path.startsWith('/')) {
        return path;
    }
    const cwd = process.env.get('PWD') || '/';
    return `${cwd}/${path}`;
}

function buildServerUrl(broker) {
    if (broker.includes('://')) {
        return broker;
    }
    return `tcp://${broker}`;
}

if (!config.topic || config.topic.length === 0) {
    fail('Topic is required. Use -t <topic>.');
}

if (config.message.length > 0 && config.file.length > 0) {
    fail('Options -m and -f are mutually exclusive.');
}

if (![0, 1, 2].includes(config.qos)) {
    fail(`Invalid QoS level '${config.qos}'. Use 0, 1, or 2.`);
}

let payload = config.message;
if (config.file.length > 0) {
    const filename = resolvePath(config.file);
    try {
        payload = fs.readFile(filename);
        debug(`Loaded payload from ${filename}`);
    } catch (err) {
        fail(`Error reading file '${filename}': ${err.message}`);
    }
}

const serverUrl = buildServerUrl(config.broker);
debug(`Connecting to ${serverUrl}`);

const client = new mqtt.Client({
    servers: [serverUrl],
    keepAlive: 30,
    connectRetryDelay: 0,
    cleanStartOnInitialConnection: true,
    connectTimeout: 10 * 1000,
});

let exitCode = 1;
let finished = false;

function finish(code) {
    if (finished) {
        return;
    }
    finished = true;
    exitCode = code;
    if (exitCode !== 0) {
        process.exit(exitCode);
    }
}

client.on('open', () => {
    debug('Connected');
    client.publish(config.topic, payload, { qos: config.qos });
});

client.on('published', (topic, reason) => {
    debug(`Published to ${topic} reason=${reason}`);
    if (reason !== 0) {
        fail(`Publish failed with reason code ${reason}.`);
        return;
    }
    client.close();
});

client.on('close', () => {
    debug('Disconnected');
    finish(0);
});

client.on('error', (err) => {
    fail(`Error: ${err.message}`);
});

