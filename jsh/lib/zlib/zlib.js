'use strict';

const _zlib = require('@jsh/zlib');

// Export all native functions directly
// Stream creation functions
const createGzip = _zlib.createGzip;
const createGunzip = _zlib.createGunzip;
const createDeflate = _zlib.createDeflate;
const createInflate = _zlib.createInflate;
const createDeflateRaw = _zlib.createDeflateRaw;
const createInflateRaw = _zlib.createInflateRaw;
const createUnzip = _zlib.createUnzip;

// Convenience methods (async with callback)
const gzip = _zlib.gzip;
const gunzip = _zlib.gunzip;
const deflate = _zlib.deflate;
const inflate = _zlib.inflate;
const deflateRaw = _zlib.deflateRaw;
const inflateRaw = _zlib.inflateRaw;
const unzip = _zlib.unzip;

// Sync convenience methods
const gzipSync = _zlib.gzipSync;
const gunzipSync = _zlib.gunzipSync;
const deflateSync = _zlib.deflateSync;
const inflateSync = _zlib.inflateSync;
const deflateRawSync = _zlib.deflateRawSync;
const inflateRawSync = _zlib.inflateRawSync;
const unzipSync = _zlib.unzipSync;

// Constants
const constants = _zlib.constants;

// Export all functions and constants
module.exports = {
    // Stream creation functions
    createGzip,
    createGunzip,
    createDeflate,
    createInflate,
    createDeflateRaw,
    createInflateRaw,
    createUnzip,

    // Async convenience methods
    gzip,
    gunzip,
    deflate,
    inflate,
    deflateRaw,
    inflateRaw,
    unzip,

    // Sync convenience methods
    gzipSync,
    gunzipSync,
    deflateSync,
    inflateSync,
    deflateRawSync,
    inflateRawSync,
    unzipSync,

    // Constants
    constants
};
