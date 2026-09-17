'use strict';

const process = require('process');
const fs = require('fs');
const nats = require('nats');
const { UUID } = require('uuid');
const parseArgs = require('util/parseArgs');
const help = require('help');
const metadata = require('/usr/share/help/jsh/nats_pub');

let showHelp = true;
let config = {};
try {
    const parsed = parseArgs(process.argv.slice(2), metadata);
    config = parsed.values;
    showHelp = config.help;
} catch (err) {
    console.println(err.message);
}

if (showHelp) {
    console.println(help.format(metadata));
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
    return `nats://${broker}`;
}

function buildInboxSubject() {
    const uuid = new UUID();
    const token = String(uuid.newV4());
    return `_INBOX.${token.split('-').join('')}`;
}

const subject = config.topic || config.subject;
if (!subject || subject.length === 0) {
    fail('Subject is required. Use -t <subject> or -s <subject>.');
}

if (config.message.length > 0 && config.file.length > 0) {
    fail('Options -m and -f are mutually exclusive.');
}

if (config.timeout <= 0) {
    fail(`Invalid timeout '${config.timeout}'. Use a positive number of milliseconds.`);
}

const replySubject = config.reply.length > 0 ? config.reply : (config.request ? buildInboxSubject() : '');

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

const client = new nats.Client({
    servers: [serverUrl],
    timeout: config.timeout,
});

let finished = false;
let published = false;
let replyTimer = null;

function clearReplyTimer() {
    if (replyTimer !== null) {
        clearTimeout(replyTimer);
        replyTimer = null;
    }
}

function finish(code) {
    if (finished) {
        return;
    }
    finished = true;
    clearReplyTimer();
    if (code !== 0) {
        process.exit(code);
    }
}

function publishMessage() {
    if (published) {
        return;
    }
    published = true;
    if (replySubject.length > 0) {
        replyTimer = setTimeout(() => {
            fail(`Timed out waiting for response on '${replySubject}'.`);
        }, config.timeout);
        client.publish(subject, payload, { reply: replySubject });
        return;
    }
    client.publish(subject, payload);
}

client.on('open', () => {
    debug('Connected');
    if (replySubject.length > 0) {
        debug(`Waiting for reply on ${replySubject}`);
        client.subscribe(replySubject);
        return;
    }
    publishMessage();
});

client.on('subscribed', (subscribedSubject, reason) => {
    debug(`Subscribed to ${subscribedSubject} reason=${reason}`);
    if (reason !== 1) {
        fail(`Subscribe failed with reason code ${reason}.`);
        return;
    }
    if (replySubject.length > 0 && subscribedSubject === replySubject) {
        publishMessage();
    }
});

client.on('published', (publishedSubject, reason) => {
    debug(`Published to ${publishedSubject} reason=${reason}`);
    if (reason !== 0) {
        fail(`Publish failed with reason code ${reason}.`);
        return;
    }
    if (replySubject.length > 0) {
        return;
    }
    client.close();
});

client.on('message', (msg) => {
    debug(`Received message on ${msg.subject}`);
    if (replySubject.length > 0 && msg.subject === replySubject) {
        clearReplyTimer();
        console.println(msg.payload);
        client.close();
    }
});

client.on('close', () => {
    debug('Disconnected');
    finish(0);
});

client.on('error', (err) => {
    fail(`Error: ${err.message}`);
});