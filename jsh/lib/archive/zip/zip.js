'use strict';

const _zip = require('@jsh/archive/zip');
const fs = require('fs');
const path = require('path');

const createZip = _zip.createZip;
const createUnzip = _zip.createUnzip;
const zip = _zip.zip;
const unzip = _zip.unzip;
const zipSync = _zip.zipSync;
const unzipSync = _zip.unzipSync;

function ensureParentDir(targetPath) {
    const dir = path.dirname(targetPath);
    if (dir && dir !== '.' && !fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
}

function toWritableBytes(data) {
    if (data instanceof Uint8Array) {
        return Array.from(data);
    }
    if (data instanceof ArrayBuffer) {
        return Array.from(new Uint8Array(data));
    }
    if (Array.isArray(data)) {
        return data;
    }
    return data;
}

function normalizeEntryData(data) {
    if (data instanceof Uint8Array) {
        return data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
    }
    if (Array.isArray(data)) {
        return Uint8Array.from(data).buffer;
    }
    return data;
}

function normalizeEntry(entry) {
    if (!entry || typeof entry !== 'object') {
        throw new TypeError('entry must be an object');
    }
    if (!entry.name || typeof entry.name !== 'string') {
        throw new TypeError('entry.name must be a non-empty string');
    }
    if (entry.data === undefined || entry.data === null) {
        throw new TypeError('entry.data is required');
    }
    return {
        ...entry,
        name: entry.name,
        data: normalizeEntryData(entry.data),
    };
}

function shouldExtract(entry, filter) {
    if (!filter) {
        return true;
    }
    if (typeof filter === 'function') {
        return !!filter(entry);
    }
    if (filter instanceof RegExp) {
        return filter.test(entry.name);
    }
    if (Array.isArray(filter)) {
        return filter.includes(entry.name);
    }
    if (typeof filter === 'string') {
        return entry.name.includes(filter);
    }
    throw new TypeError('filter must be a function, RegExp, array, or string');
}

function normalizeExtractOptions(overwriteOrOptions, maybeOptions) {
    if (typeof overwriteOrOptions === 'object' && overwriteOrOptions !== null && !Array.isArray(overwriteOrOptions)) {
        return {
            overwrite: !!overwriteOrOptions.overwrite,
            filter: overwriteOrOptions.filter,
        };
    }
    const options = maybeOptions && typeof maybeOptions === 'object' ? maybeOptions : {};
    return {
        overwrite: !!overwriteOrOptions,
        filter: options.filter,
    };
}

class Zip {
    constructor(filePath) {
        this.filePath = filePath || null;
        this.entries = [];
        if (this.filePath) {
            this._reload();
        }
    }

    _reload() {
        if (!this.filePath) {
            return [];
        }
        const archive = fs.readFileSync(this.filePath, 'buffer');
        this.entries = unzipSync(archive);
        return this.entries;
    }

    addFile(filePath, entryName) {
        const data = fs.readFileSync(filePath, 'buffer');
        this.entries.push({
            name: entryName || path.basename(filePath),
            data,
        });
        return this;
    }

    addBuffer(data, entryName, options = {}) {
        if (!entryName || typeof entryName !== 'string') {
            throw new TypeError('addBuffer() requires entryName');
        }
        this.entries.push({
            ...options,
            name: entryName,
            data: normalizeEntryData(data),
        });
        return this;
    }

    addEntry(entry) {
        this.entries.push(normalizeEntry(entry));
        return this;
    }

    getEntries() {
        const entries = this.filePath ? this._reload() : this.entries;
        return entries.slice();
    }

    writeTo(filePath) {
        const targetPath = filePath || this.filePath;
        if (!targetPath) {
            throw new Error('writeTo() requires a target path');
        }
        const archive = zipSync(this.entries);
        ensureParentDir(targetPath);
        fs.writeFileSync(targetPath, toWritableBytes(archive), 'buffer');
        this.filePath = targetPath;
        return targetPath;
    }

    extractAllTo(outputDir, overwriteOrOptions, maybeOptions) {
        const options = normalizeExtractOptions(overwriteOrOptions, maybeOptions);
        const entries = this.filePath ? this._reload() : this.entries;
        fs.mkdirSync(outputDir, { recursive: true });
        for (const entry of entries) {
            if (!shouldExtract(entry, options.filter)) {
                continue;
            }
            const targetPath = path.join(outputDir, entry.name);
            if (entry.isDir) {
                fs.mkdirSync(targetPath, { recursive: true });
                continue;
            }
            ensureParentDir(targetPath);
            if (!options.overwrite && fs.existsSync(targetPath)) {
                throw new Error(`extractAllTo() target exists and overwrite is false: ${targetPath} (entry: ${entry.name})`);
            }
            fs.writeFileSync(targetPath, toWritableBytes(entry.data), 'buffer');
        }
        return outputDir;
    }
}

module.exports = {
    createZip,
    createUnzip,
    zip,
    unzip,
    zipSync,
    unzipSync,
    Zip,
};