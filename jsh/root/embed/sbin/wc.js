(() => {
    const fs = require('fs');
    const process = require('process');
    const { parseArgs } = require('util');
    const pwd = process.env.get('PWD');

    const { values, positionals } = parseArgs(process.argv.slice(2), {
        options: {
            lines: { type: 'boolean', short: 'l', default: false },
            words: { type: 'boolean', short: 'w', default: false },
            bytes: { type: 'boolean', short: 'c', default: false },
            chars: { type: 'boolean', short: 'm', default: false },
            help: { type: 'boolean', short: 'h', default: false },
        },
        strict: false,
        allowPositionals: true,
    });

    if (values.help) {
        console.println('Usage: wc [OPTION]... [FILE]...');
        console.println('Count lines, words, bytes, and characters for each FILE.');
        console.println('Read standard input when no FILE is given or when FILE is -.');
        console.println('');
        console.println('Options:');
        console.println('  -l, --lines           print the line counts');
        console.println('  -w, --words           print the word counts');
        console.println('  -c, --bytes           print the byte counts');
        console.println('  -m, --chars           print the character counts');
        console.println('  -h, --help            display this help and exit');
        process.exit(0);
    }

    function selectedColumns() {
        const columns = [];
        if (values.lines) {
            columns.push('lines');
        }
        if (values.words) {
            columns.push('words');
        }
        if (values.bytes) {
            columns.push('bytes');
        }
        if (values.chars) {
            columns.push('chars');
        }
        if (columns.length === 0) {
            columns.push('lines', 'words', 'bytes');
        }
        return columns;
    }

    function sourceSpec(filepath, implicit) {
        if (filepath === '-') {
            return { path: '-', label: implicit ? '' : '-', implicit };
        }
        return {
            path: filepath.startsWith('/') ? filepath : pwd + '/' + filepath,
            label: filepath,
            implicit,
        };
    }

    function createCounts() {
        return {
            lines: 0,
            words: 0,
            bytes: 0,
            chars: 0,
            inWord: false,
        };
    }

    function isWhitespace(ch) {
        return /\s/.test(ch);
    }

    function updateCounts(counts, chunk) {
        counts.bytes += chunk.length;
        counts.chars += chunk.length;

        for (let i = 0; i < chunk.length; i++) {
            const ch = chunk[i];
            if (ch === '\n') {
                counts.lines += 1;
            }
            if (isWhitespace(ch)) {
                counts.inWord = false;
                continue;
            }
            if (!counts.inWord) {
                counts.words += 1;
                counts.inWord = true;
            }
        }
    }

    function formatCounts(counts, label, columns) {
        const valuesByColumn = {
            lines: counts.lines,
            words: counts.words,
            bytes: counts.bytes,
            chars: counts.chars,
        };
        const parts = columns.map((column) => String(valuesByColumn[column]));
        if (label) {
            parts.push(label);
        }
        return parts.join(' ');
    }

    function writeStdoutLine(text) {
        process.stdout.write(String(text) + '\n');
    }

    function writeStderrLine(text) {
        process.stderr.write(String(text) + '\n');
    }

    function reportError(filepath, err) {
        const message = err && err.message ? err.message : String(err);
        writeStderrLine(`wc: ${filepath}: ${message}`);
    }

    function countSource(spec, columns, callback) {
        if (spec.path === '-') {
            const chunk = process.stdin.read();
            if (chunk instanceof Error) {
                callback(chunk);
                return;
            }
            const counts = createCounts();
            updateCounts(counts, chunk || '');
            callback(null, counts);
            return;
        }

        let stream;
        try {
            stream = fs.createReadStream(spec.path, { encoding: 'utf8' });
        } catch (err) {
            callback(err);
            return;
        }

        const counts = createCounts();
        stream.on('data', (chunk) => {
            updateCounts(counts, chunk);
        });
        stream.on('error', (err) => {
            callback(err);
        });
        stream.on('end', () => {
            callback(null, counts);
        });
    }

    function addCounts(target, source) {
        target.lines += source.lines;
        target.words += source.words;
        target.bytes += source.bytes;
        target.chars += source.chars;
    }

    function processSources(sources, index, columns, total, exitState) {
        if (index >= sources.length) {
            if (sources.length > 1) {
                writeStdoutLine(formatCounts(total, 'total', columns));
            }
            process.exit(exitState.code);
            return;
        }

        const spec = sources[index];
        countSource(spec, columns, (err, counts) => {
            if (err) {
                reportError(spec.label || '-', err);
                exitState.code = 1;
                processSources(sources, index + 1, columns, total, exitState);
                return;
            }

            addCounts(total, counts);
            const label = spec.implicit && sources.length === 1 ? '' : spec.label;
            writeStdoutLine(formatCounts(counts, label, columns));
            processSources(sources, index + 1, columns, total, exitState);
        });
    }

    const columns = selectedColumns();
    const sources = positionals.length === 0
        ? [sourceSpec('-', true)]
        : positionals.map((filepath) => sourceSpec(filepath, false));
    processSources(sources, 0, columns, createCounts(), { code: 0 });
})()