'use strict';

/*
CGI-bin SSE caller example (jsh)

File: public/myapp/cgi-bin/log-stream.js

const fs = require('fs');
const tailSSE = require('util/tail/sse');

const target = process.env.QUERY_FILE || '/tmp/app.log';
const intervalMs = Number(process.env.QUERY_INTERVAL_MS || 500);

const adapter = tailSSE.create(target, {
    fromStart: false,
    event: 'log',
    retryMs: 1500,
});

adapter.writeHeaders();

const timer = setInterval(function () {
    try {
        adapter.poll();
    } catch (err) {
        adapter.send(String(err), 'error');
        clearInterval(timer);
        adapter.close();
        process.exit(0);
    }
}, intervalMs);

process.on('SIGINT', function () {
    clearInterval(timer);
    adapter.close();
    process.exit(0);
});

process.on('SIGTERM', function () {
    clearInterval(timer);
    adapter.close();
    process.exit(0);
});
*/

const _tail = require('@jsh/util/tail');

function defaultWriter(chunk) {
    process.stdout.write(String(chunk));
}

function normalizeWriter(writeFn) {
    if (typeof writeFn === 'function') {
        return function (chunk) {
            writeFn(String(chunk));
        };
    }
    return defaultWriter;
}

function normalizePath(path) {
    if (!path || typeof path !== 'string') {
        throw new Error('tail/sse.create(path, options): path must be a non-empty string');
    }
    return path;
}

function normalizeTail(path, options) {
    return _tail.create(path, {
        fromStart: !!(options && options.fromStart),
    });
}

function writeSSE(writer, eventName, data) {
    if (eventName) {
        writer('event: ' + eventName + '\n');
    }
    const text = String(data);
    const lines = text.split(/\r?\n/);
    for (let i = 0; i < lines.length; i++) {
        writer('data: ' + lines[i] + '\n');
    }
    writer('\n');
}

function create(path, options) {
    const targetPath = normalizePath(path);
    const opts = options || {};
    const writer = normalizeWriter(opts.write);
    const tailer = normalizeTail(targetPath, opts);
    const eventName = typeof opts.event === 'string' ? opts.event : '';

    return {
        path: targetPath,
        writeHeaders: function () {
            writer('Content-Type: text/event-stream\n');
            writer('Cache-Control: no-cache\n');
            writer('Connection: keep-alive\n');
            if (opts.retryMs !== undefined && opts.retryMs !== null) {
                writer('retry: ' + String(opts.retryMs) + '\n');
            }
            writer('\n');
        },
        poll: function () {
            const lines = tailer.poll();
            for (let i = 0; i < lines.length; i++) {
                writeSSE(writer, eventName, lines[i]);
            }
            return lines;
        },
        send: function (data, event) {
            const evt = typeof event === 'string' ? event : eventName;
            writeSSE(writer, evt, data);
        },
        comment: function (text) {
            writer(': ' + String(text) + '\n\n');
        },
        close: function () {
            tailer.close();
        },
    };
}

module.exports = {
    create,
};
