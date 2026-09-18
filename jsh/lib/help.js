'use strict';

const fs = require('fs');
const parseArgs = require('util/parseArgs');

const helpRoot = '/usr/share/help';

function parseTarget(namespaces, commandPath) {
    const path = Array.isArray(commandPath) ? [...commandPath] : [];
    let searchNamespaces = [...namespaces];
    if (path.length > 0) {
        const separator = path[0].indexOf(':');
        if (separator > 0) {
            searchNamespaces = [path[0].substring(0, separator)];
            path[0] = path[0].substring(separator + 1);
        }
    }
    return { namespaces: searchNamespaces, path };
}

function load(namespace, command) {
    try {
        return require(`${helpRoot}/${namespace}/${command}`);
    } catch (_) {
        return null;
    }
}

function find(namespaces, commandPath) {
    const target = parseTarget(namespaces, commandPath);
    if (target.path.length === 0 || !target.path[0]) {
        return null;
    }
    for (const namespace of target.namespaces) {
        let metadata = load(namespace, target.path[0].toLowerCase());
        if (!metadata) {
            continue;
        }
        for (const name of target.path.slice(1)) {
            metadata = metadata.commands && metadata.commands[name.toLowerCase()];
            if (!metadata) {
                break;
            }
        }
        if (metadata) {
            return { namespace, metadata };
        }
    }
    return null;
}

function list(namespaces) {
    const result = [];
    const seen = {};
    for (const namespace of namespaces) {
        let names;
        try {
            names = fs.readdirSync(`${helpRoot}/${namespace}`);
        } catch (_) {
            continue;
        }
        for (const filename of names) {
            if (!filename.endsWith('.js')) {
                continue;
            }
            const name = filename.substring(0, filename.length - 3);
            if (seen[name]) {
                continue;
            }
            const metadata = load(namespace, name);
            if (metadata) {
                seen[name] = true;
                result.push({ namespace, name, kind: metadata.kind || 'command', description: metadata.description || '' });
            }
        }
    }
    return result.sort((left, right) => left.name.localeCompare(right.name));
}

function distance(left, right) {
    const rows = [];
    for (let column = 0; column <= right.length; column++) {
        rows[0] = rows[0] || [];
        rows[0][column] = column;
    }
    for (let row = 1; row <= left.length; row++) {
        rows[row] = [row];
        for (let column = 1; column <= right.length; column++) {
            rows[row][column] = Math.min(
                rows[row - 1][column] + 1,
                rows[row][column - 1] + 1,
                rows[row - 1][column - 1] + (left[row - 1] === right[column - 1] ? 0 : 1),
            );
        }
    }
    return rows[left.length][right.length];
}

function suggest(namespaces, commandPath) {
    const target = parseTarget(namespaces, commandPath);
    const path = target.path.map((name) => name.toLowerCase());
    const name = path[path.length - 1] || '';
    let candidates;
    if (path.length > 1) {
        const parent = find(target.namespaces, path.slice(0, -1));
        candidates = parent
            ? Object.keys(parent.metadata.commands || {}).map((child) => ({
                namespace: parent.namespace,
                name: [...path.slice(0, -1), child].join(' '),
                matchName: child,
            }))
            : [];
    } else {
        candidates = list(target.namespaces).map((entry) => ({ ...entry, matchName: entry.name }));
    }
    return candidates
        .map((entry) => ({ ...entry, distance: distance(name, entry.matchName) }))
        .filter((entry) => entry.distance <= Math.max(2, Math.floor(name.length / 3)))
        .sort((left, right) => left.distance - right.distance || left.name.localeCompare(right.name))
        .slice(0, 3);
}

function format(metadata) {
    if (typeof metadata.content === 'string') {
        return metadata.content.trimEnd();
    }
    const config = {
        usage: metadata.usage,
        options: metadata.options || {},
        positionals: metadata.positionals || [],
    };
    let output = parseArgs.formatHelp(config);
    if (metadata.longDescription) {
        output += `\n${metadata.longDescription}`;
    }
    const commands = metadata.commands || {};
    const names = Object.keys(commands).sort();
    if (names.length > 0) {
        output += '\n\nAvailable commands:';
        const width = names.reduce((max, name) => Math.max(max, name.length), 0);
        for (const name of names) {
            output += `\n  ${name.padEnd(width)}  ${commands[name].description || ''}`;
        }
    }
    return output;
}

function printMetadata(metadata) {
    console.println(format(metadata));
}

function print(namespaces, commandPath) {
    const found = find(namespaces, commandPath);
    if (found) {
        printMetadata(found.metadata);
        return true;
    }
    const name = commandPath.join(' ');
    console.println(`No help document found for '${name}'.`);
    const candidates = suggest(namespaces, commandPath);
    if (candidates.length > 0) {
        console.println('\nDid you mean?');
        for (const candidate of candidates) {
            console.println(`  ${candidate.namespace}:${candidate.name}`);
        }
    }
    return false;
}

function printList(namespaces) {
    const entries = list(namespaces);
    const groups = [
        { kind: 'topic', title: 'Available topics:' },
        { kind: 'command', title: 'Available commands:' },
    ];
    let printed = false;
    for (const group of groups) {
        const groupEntries = entries.filter((entry) => entry.kind === group.kind);
        if (groupEntries.length === 0) {
            continue;
        }
        if (printed) {
            console.println();
        }
        console.println(group.title);
        const width = groupEntries.reduce((max, entry) => Math.max(max, entry.name.length), 0);
        for (const entry of groupEntries) {
            console.println(`  ${entry.name.padEnd(width)}  ${entry.description}`);
        }
        printed = true;
    }
}

function tryHandle(fields, namespaces) {
    if (!fields || fields.length === 0 || fields[0].toLowerCase() !== 'help') {
        return null;
    }
    if (fields.length === 1) {
        printList(namespaces);
        return 0;
    } else if (fields[1] === '-h' || fields[1] === '--help') {
        return print(namespaces, ['jsh:help']) ? 0 : 1;
    } else {
        return print(namespaces, fields.slice(1)) ? 0 : 1;
    }
}

module.exports = { find, format, list, print, printList, suggest, tryHandle };