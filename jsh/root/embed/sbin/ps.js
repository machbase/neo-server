(() => {
    'use strict';

    const fs = require('fs');
    const path = require('path');
    const process = require('process');
    const parseArgs = require('util/parseArgs');

    const options = {
        help: { type: 'boolean', short: 'h', description: 'Show this help message', default: false },
        json: { type: 'boolean', short: 'j', description: 'Print process entries as JSON', default: false },
    };

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), {
            options: options,
            allowPositionals: false,
        });
    } catch (err) {
        console.println(err.message);
        console.println(parseArgs.formatHelp({
            usage: 'Usage: ps [options]',
            description: 'List JSH process entries under /proc/process.',
            options: options,
        }));
        process.exit(1);
    }

    if (parsed.values.help) {
        console.println(parseArgs.formatHelp({
            usage: 'Usage: ps [options]',
            description: 'List JSH process entries under /proc/process.',
            options: options,
        }));
        process.exit(0);
    }

    const entries = listProcEntries('/proc/process');
    if (parsed.values.json) {
        console.println(JSON.stringify(entries, null, 2));
        return;
    }

    console.printf('%-8s %-8s %-10s %-30s %s\n', 'PID', 'PPID', 'STATE', 'STARTED', 'COMMAND');
    entries.forEach((entry) => {
        console.printf(
            '%-8s %-8s %-10s %-30s %s\n',
            String(entry.pid),
            String(entry.ppid),
            String(entry.state || ''),
            formatDisplayTimestamp(entry.started_at || ''),
            entry.brief_command_line
        );
    });

    function listProcEntries(rootDir) {
        if (!fs.existsSync(rootDir)) {
            return [];
        }
        const names = fs.readdirSync(rootDir);
        const entries = [];
        names.forEach((name) => {
            const baseDir = path.join(rootDir, name);
            let stats;
            try {
                stats = fs.statSync(baseDir);
            } catch (err) {
                return;
            }
            if (!stats.isDirectory()) {
                return;
            }

            const meta = readJSON(path.join(baseDir, 'meta.json'));
            const status = readJSON(path.join(baseDir, 'status.json'));
            if (!meta || !status || typeof meta.pid !== 'number' || meta.pid <= 0) {
                return;
            }

            entries.push({
                pid: meta.pid,
                ppid: meta.ppid || 0,
                pgid: meta.pgid || 0,
                command: meta.command || '',
                args: Array.isArray(meta.args) ? meta.args.slice() : [],
                cwd: meta.cwd || '',
                started_at: status.started_at || meta.started_at || '',
                updated_at: status.updated_at || '',
                state: status.state || '',
                service_controller_client_id: meta.service_controller_client_id || '',
                exec_path: meta.exec_path || '',
                command_line: renderCommandLine(meta.command || '', meta.args),
                brief_command_line: renderBriefCommandLine(meta.command || '', meta.args),
            });
        });

        entries.sort((left, right) => left.pid - right.pid);
        return entries;
    }

    function readJSON(filename) {
        try {
            return JSON.parse(fs.readFileSync(filename, 'utf8'));
        } catch (err) {
            return null;
        }
    }

    function renderCommandLine(command, args) {
        const fields = [];
        if (command) {
            fields.push(command);
        }
        if (Array.isArray(args)) {
            args.forEach((arg) => fields.push(String(arg)));
        }
        return fields.join(' ').trim();
    }

    function renderBriefCommandLine(command, args) {
        const fields = [];
        if (command) {
            fields.push(command);
        }
        if (Array.isArray(args)) {
            args.forEach((arg) => fields.push(String(arg)));
        }

        const scriptIndex = fields.findIndex((field) => /\.js$/.test(field));
        if (scriptIndex >= 0) {
            return fields.slice(scriptIndex).join(' ').trim();
        }
        return fields.join(' ').trim();
    }

    function formatDisplayTimestamp(value) {
        const text = String(value || '').trim();
        const match = text.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/);
        if (match) {
            return match[1] + ' ' + match[2];
        }
        return text;
    }
})();
