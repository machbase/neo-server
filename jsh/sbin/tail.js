(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const parseArgs = require('util/parseArgs');
    const tail = require('util/tail');

    const pwd = process.env.get('PWD');

    const options = {
        follow: { type: 'boolean', short: 'f', description: 'Follow the file as it grows', default: false },
        lines: { type: 'string', short: 'n', description: 'Number of lines to print (default: 10)', default: '10' },
        help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
    };
    const positionals_spec = [
        { name: 'file', type: 'string', description: 'File to tail' },
    ];

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), {
            options,
            allowPositionals: true,
        });
    } catch (err) {
        process.stderr.write(err.message + '\n');
        process.exit(1);
    }

    if (parsed.values.help || parsed.positionals.length === 0) {
        console.println(parseArgs.formatHelp({
            usage: 'Usage: tail [options] <file>',
            description: 'Output the last part of a file.',
            options,
            positionals: positionals_spec,
        }));
        process.exit(parsed.values.help ? 0 : 1);
    }

    const numLines = parseInt(parsed.values.lines, 10);
    const lineCount = (isNaN(numLines) || numLines < 0) ? 10 : numLines;
    const follow = parsed.values.follow;

    const rawPath = parsed.positionals[0];
    const filePath = rawPath.startsWith('/') ? rawPath : pwd + '/' + rawPath;

    // Read file and collect last n lines into lineBuffer callback(err, lines[])
    function readLastLines(path, n, callback) {
        let stream;
        try {
            stream = fs.createReadStream(path, { encoding: 'utf8' });
        } catch (err) {
            callback(err);
            return;
        }

        const buf = [];
        let remainder = '';

        stream.on('data', function (chunk) {
            const text = remainder + String(chunk);
            const parts = text.split('\n');
            remainder = parts.pop();
            for (let i = 0; i < parts.length; i++) {
                buf.push(parts[i]);
                if (buf.length > n) {
                    buf.shift();
                }
            }
        });
        stream.on('error', function (err) {
            callback(err);
        });
        stream.on('end', function () {
            if (remainder !== '') {
                buf.push(remainder);
                if (buf.length > n) {
                    buf.shift();
                }
            }
            callback(null, buf);
        });
    }

    function printLines(lines) {
        for (let i = 0; i < lines.length; i++) {
            process.stdout.write(lines[i] + '\n');
        }
    }

    if (!follow) {
        // Print last N lines then exit
        readLastLines(filePath, lineCount, function (err, lines) {
            if (err) {
                process.stderr.write('tail: ' + filePath + ': ' + (err.message || String(err)) + '\n');
                process.exit(1);
            }
            printLines(lines);
            process.exit(0);
        });
        return;
    }

    // -f (follow) mode:
    // 1. Create tailer positioned at end of file first (so we don't re-emit historical lines via poll)
    // 2. Print last N historical lines from full read
    // 3. Poll for new content on interval
    const tailer = tail.create(filePath, { fromStart: false });

    // First poll sets the file position at the current end (returns empty)
    try {
        tailer.poll();
    } catch (err) {
        process.stderr.write('tail: ' + filePath + ': ' + (err.message || String(err)) + '\n');
        process.exit(1);
    }

    readLastLines(filePath, lineCount, function (err, lines) {
        if (err) {
            process.stderr.write('tail: ' + filePath + ': ' + (err.message || String(err)) + '\n');
            tailer.close();
            process.exit(1);
        }

        printLines(lines);

        let timer = null;
        let closed = false;
        let onSigint = null;
        let onSigterm = null;

        function cleanupFollow() {
            if (closed) {
                return;
            }
            closed = true;

            if (timer !== null) {
                clearInterval(timer);
                timer = null;
            }
            if (onSigint) {
                process.off('SIGINT', onSigint);
                onSigint = null;
            }
            if (onSigterm) {
                process.off('SIGTERM', onSigterm);
                onSigterm = null;
            }
            tailer.close();
        }

        timer = setInterval(function () {
            let newLines;
            try {
                newLines = tailer.poll();
            } catch (e) {
                process.stderr.write('tail: ' + (e.message || String(e)) + '\n');
                cleanupFollow();
                return;
            }
            if (newLines && newLines.length > 0) {
                printLines(newLines);
            }
        }, 250);

        // Do not call process.exit() on SIGINT/SIGTERM in follow mode.
        // Exiting here can terminate the interactive JSH shell process itself.
        onSigint = function () {
            cleanupFollow();
        };
        onSigterm = function () {
            cleanupFollow();
        };

        process.on('SIGINT', onSigint);
        process.on('SIGTERM', onSigterm);
    });
})();
