'use strict';

const _fs = require('@jsh/fs');
const EventEmitter = require('events');

/*
## Features

- **Node.js Compatible API**: Familiar function names and behavior similar to Node.js fs module
- **Synchronous Operations**: All operations are synchronous (Sync suffix on function names)
- **Path Resolution**: Automatically resolves relative paths to absolute paths
- **Error Handling**: Proper error codes (ENOENT, EACCES, etc.) for better error handling
- **File Type Detection**: Check if path is file, directory, symlink, etc.

## Compatibility Notes

- All functions are synchronous (no async/callback versions yet)
- Some advanced features may not be fully implemented depending on jsh's native filesystem capabilities
- Error codes and messages follow Node.js conventions where possible
- Path resolution assumes Unix-style paths
*/

// Polyfill for Buffer.isBuffer if not available
if (!Buffer.isBuffer) {
    Buffer.isBuffer = function (obj) {
        // goja_nodejs Buffer constructor names are Go function names, so use the prototype chain.
        return obj instanceof Buffer;
    };
}

function isByteLike(data) {
    if (data === null || data === undefined) {
        return false;
    }
    if (Array.isArray(data) || data instanceof Uint8Array || data instanceof ArrayBuffer || Buffer.isBuffer(data)) {
        return true;
    }
    if (typeof data === 'object' && typeof data.byteLength === 'number') {
        return true;
    }
    return false;
}

function normalizeByteSource(data) {
    if (data === null || data === undefined) {
        return new Uint8Array(0);
    }
    if (data instanceof ArrayBuffer) {
        return new Uint8Array(data);
    }
    if (data instanceof Uint8Array) {
        return data;
    }
    if (Buffer.isBuffer(data)) {
        return new Uint8Array(data);
    }
    if (Array.isArray(data)) {
        return Uint8Array.from(data);
    }
    if (typeof data === 'object' && typeof data.byteLength === 'number') {
        if (data.buffer && typeof data.byteOffset === 'number') {
            return new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
        }
        if (typeof data.slice === 'function') {
            return new Uint8Array(data.slice(0, data.byteLength));
        }
    }
    return Uint8Array.from(Buffer.from(String(data), 'utf8'));
}

// Convert byte array to string
function bytesToString(bytes) {
    return Buffer.from(normalizeByteSource(bytes)).toString('utf8');
}

// Convert string to byte array
function stringToBytes(str) {
    return Uint8Array.from(Buffer.from(String(str), 'utf8'));
}

function getFileType(mode) {
    const modeString = mode.string();
    const typeFlags = modeString.slice(0, -9);

    if (typeFlags.includes('d')) return 'directory';
    if (typeFlags.includes('L')) return 'symbolicLink';
    if (typeFlags.includes('D')) {
        return typeFlags.includes('c') ? 'characterDevice' : 'blockDevice';
    }
    if (typeFlags.includes('p')) return 'fifo';
    if (typeFlags.includes('S')) return 'socket';
    if (typeFlags.includes('?')) return 'irregular';
    return 'file';
}

function getNativeErrorCode(error, fallbackCode) {
    if (error && typeof error.code === 'string') {
        return error.code;
    }
    if (error && error.value !== undefined && error.value !== null) {
        const nativeCode = _fs.errorCode(error.value);
        if (nativeCode) {
            return nativeCode;
        }
    }

    // Errors relayed as text (e.g. from the controller RPC) carry no error identity.
    const message = error && typeof error.message === 'string' ? error.message : String(error);
    const separator = message.lastIndexOf(': ');
    const reason = (separator >= 0 ? message.slice(separator + 2) : message).toLowerCase();
    const errorMessages = {
        ENOENT: ['no such file or directory', 'file does not exist'],
        ENOTDIR: ['not a directory'],
        EEXIST: ['file exists'],
        ENOTEMPTY: ['directory not empty'],
        EACCES: ['permission denied'],
        EPERM: ['operation not permitted'],
        EINVAL: ['invalid argument'],
        EISDIR: ['is a directory'],
        ELOOP: ['too many levels of symbolic links'],
        ENOTSUP: ['operation not supported', 'unsupported operation'],
        EIO: ['input/output error'],
        ENOSPC: ['no space left on device'],
        EROFS: ['read-only file system'],
        ENAMETOOLONG: ['file name too long'],
        EFBIG: ['file too large'],
        EBUSY: ['device or resource busy'],
        EXDEV: ['cross-device link', 'invalid cross-device link']
    };

    for (const code of Object.keys(errorMessages)) {
        if (errorMessages[code].includes(reason)) {
            return code;
        }
    }
    return fallbackCode;
}

function createFilesystemError(nativeError, syscall, path, fallbackCode) {
    const errorCode = getNativeErrorCode(nativeError, fallbackCode);
    const errorPath = nativeError && nativeError.path ? nativeError.path : path;
    const errorSyscall = nativeError && nativeError.syscall ? nativeError.syscall : syscall;
    return makeFilesystemError(errorCode, errorSyscall, errorPath);
}

// File descriptor errors carry no path. Native layers report an unknown
// descriptor as EINVAL, and a read-only descriptor rejects writes with a
// permission error; Node.js reports both as EBADF.
function createDescriptorError(nativeError, syscall) {
    if (isSharedWriteConflictError(nativeError)) {
        return makeFilesystemError('ECONFLICT', syscall);
    }
    let errorCode = getNativeErrorCode(nativeError, 'EBADF');
    if (errorCode === 'EINVAL') {
        errorCode = 'EBADF';
    } else if ((errorCode === 'EPERM' || errorCode === 'EACCES') && (syscall === 'read' || syscall === 'write')) {
        errorCode = 'EBADF';
    }
    return makeFilesystemError(errorCode, syscall);
}

function makeFilesystemError(errorCode, syscall, path) {
    const errorNumbers = {
        ENOENT: -2,
        EACCES: -13,
        EPERM: -1,
        EEXIST: -17,
        ENOTDIR: -20,
        EISDIR: -21,
        EINVAL: -22,
        ENOTEMPTY: -39,
        ELOOP: -40,
        ENOTSUP: -95,
        EIO: -5,
        ENOSPC: -28,
        EROFS: -30,
        ENAMETOOLONG: -36,
        EFBIG: -27,
        EBUSY: -16,
        EBADF: -9,
        EXDEV: -18,
        ECONFLICT: -32009
    };
    const errorMessages = {
        ENOENT: 'no such file or directory',
        EACCES: 'permission denied',
        EPERM: 'operation not permitted',
        EEXIST: 'file already exists',
        ENOTDIR: 'not a directory',
        EISDIR: 'illegal operation on a directory',
        EINVAL: 'invalid argument',
        ENOTEMPTY: 'directory not empty',
        ELOOP: 'too many symbolic links encountered',
        ENOTSUP: 'operation not supported',
        EIO: 'input/output error',
        ENOSPC: 'no space left on device',
        EROFS: 'read-only file system',
        ENAMETOOLONG: 'file name too long',
        EFBIG: 'file too large',
        EBUSY: 'resource busy or locked',
        EBADF: 'bad file descriptor',
        EXDEV: 'cross-device link not permitted',
        ECONFLICT: 'concurrent shared file update'
    };
    const reason = errorMessages[errorCode] || 'filesystem operation failed';
    const hasPath = path !== undefined && path !== null;
    const error = new Error(hasPath ? `${errorCode}: ${reason}, ${syscall} '${path}'` : `${errorCode}: ${reason}, ${syscall}`);
    error.code = errorCode;
    error.errno = errorNumbers[errorCode];
    if (hasPath) {
        error.path = path;
    }
    error.syscall = syscall;
    return error;
}

function concatByteArrays(first, second) {
    const left = normalizeByteSource(first);
    const right = normalizeByteSource(second);
    if (left.length === 0) {
        return right;
    }
    if (right.length === 0) {
        return left;
    }
    const merged = new Uint8Array(left.length + right.length);
    merged.set(left, 0);
    merged.set(right, left.length);
    return merged;
}

function splitIncompleteUtf8Tail(bytes) {
    const input = normalizeByteSource(bytes);
    if (input.length === 0) {
        return { complete: input, carry: new Uint8Array(0) };
    }

    let continuationCount = 0;
    let index = input.length - 1;
    while (index >= 0 && (input[index] & 0xC0) === 0x80) {
        continuationCount++;
        index--;
    }
    if (index < 0) {
        return { complete: input, carry: new Uint8Array(0) };
    }

    const lead = input[index];
    let expectedLength = 0;
    if ((lead & 0x80) === 0) {
        expectedLength = 1;
    } else if ((lead & 0xE0) === 0xC0) {
        expectedLength = 2;
    } else if ((lead & 0xF0) === 0xE0) {
        expectedLength = 3;
    } else if ((lead & 0xF8) === 0xF0) {
        expectedLength = 4;
    } else {
        return { complete: input, carry: new Uint8Array(0) };
    }

    const actualLength = input.length - index;
    if (expectedLength > 1 && actualLength < expectedLength) {
        return {
            complete: input.slice(0, index),
            carry: input.slice(index),
        };
    }

    return { complete: input, carry: new Uint8Array(0) };
}

function decodeUtf8Chunk(chunk, state, flush) {
    const input = normalizeByteSource(chunk);
    const carry = state && state.carry ? state.carry : new Uint8Array(0);
    const merged = concatByteArrays(carry, input);
    const parts = flush ? { complete: merged, carry: new Uint8Array(0) } : splitIncompleteUtf8Tail(merged);
    if (state) {
        state.carry = parts.carry;
    }
    return bytesToString(parts.complete);
}

function chunkLength(chunk) {
    if (chunk === null || chunk === undefined) {
        return 0;
    }
    if (typeof chunk.byteLength === 'number') {
        return chunk.byteLength;
    }
    return typeof chunk.length === 'number' ? chunk.length : 0;
}

function toBufferChunk(chunk) {
    return normalizeByteSource(chunk);
}

function toStringChunk(chunk) {
    if (typeof chunk === 'string') {
        return chunk;
    }
    if (chunk instanceof Uint8Array || Buffer.isBuffer(chunk) || Array.isArray(chunk)) {
        return bytesToString(chunk);
    }
    return String(chunk);
}

function getProcessModule() {
    return require('./process');
}

function resolveFileEncoding(options) {
    if (options === null || options === undefined) {
        return 'utf8';
    }
    if (typeof options === 'string') {
        return options;
    }
    if (typeof options === 'object') {
        if (Object.prototype.hasOwnProperty.call(options, 'encoding')) {
            return options.encoding === undefined ? 'utf8' : options.encoding;
        }
    }
    return 'utf8';
}

function makeUnknownEncodingError(encoding) {
    const error = new TypeError(`Unknown encoding: ${encoding}`);
    error.code = 'ERR_UNKNOWN_ENCODING';
    return error;
}

function singleByteStringToBytes(str) {
    const bytes = new Uint8Array(str.length);
    for (let i = 0; i < str.length; i++) {
        bytes[i] = str.charCodeAt(i) & 0xff;
    }
    return bytes;
}

function utf16leStringToBytes(str) {
    const bytes = new Uint8Array(str.length * 2);
    for (let i = 0; i < str.length; i++) {
        const code = str.charCodeAt(i);
        bytes[i * 2] = code & 0xff;
        bytes[i * 2 + 1] = code >> 8;
    }
    return bytes;
}

// Normalize a Node.js encoding name; returns undefined for unknown names.
function normalizeEncodingName(encoding) {
    switch (String(encoding).toLowerCase()) {
        case 'utf8':
        case 'utf-8':
            return 'utf8';
        case 'hex':
            return 'hex';
        case 'base64':
            return 'base64';
        case 'base64url':
            return 'base64url';
        case 'latin1':
        case 'binary':
            return 'latin1';
        case 'ascii':
            return 'ascii';
        case 'ucs2':
        case 'ucs-2':
        case 'utf16le':
        case 'utf-16le':
            return 'utf16le';
        default:
            return undefined;
    }
}

// Encode a string with a normalized Node.js encoding name.
function encodeString(str, encoding) {
    switch (encoding) {
        case 'hex':
            return viewToBytes(Buffer.from(str, 'hex'));
        case 'base64':
            return viewToBytes(Buffer.from(str, 'base64'));
        case 'base64url':
            // goja_nodejs registers this codec as 'base64Url'.
            return viewToBytes(Buffer.from(str, 'base64Url'));
        case 'latin1':
        case 'ascii':
            return singleByteStringToBytes(str);
        case 'utf16le':
            return utf16leStringToBytes(str);
        default:
            return stringToBytes(str);
    }
}

// Convert writeFile/appendFile data to bytes: Node.js types plus JSH extensions (ArrayBuffer, byte Array).
// The 'buffer' and null encodings are JSH conventions that mean UTF-8 for strings.
function fileDataToBytes(data, encoding) {
    const binary = binaryToBytes(data);
    if (binary) {
        return binary;
    }
    // JSH extension beyond Node.js: plain byte arrays, e.g. http readBodyBuffer().
    if (Array.isArray(data)) {
        return Uint8Array.from(data);
    }
    if (typeof data !== 'string') {
        throw makeInvalidArgTypeError('data', 'of type string, Array, or an instance of Buffer, TypedArray, DataView, or ArrayBuffer', data);
    }
    if (encoding === null || encoding === undefined || encoding === 'buffer') {
        return stringToBytes(data);
    }
    const name = normalizeEncodingName(encoding);
    if (name === undefined) {
        throw makeUnknownEncodingError(encoding);
    }
    return encodeString(data, name);
}

/**
 * Read file contents synchronously
 * @param {string} path - File path
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 * @returns {string|Buffer} File contents as string or Buffer
 */
function readFileSync(path, options) {
    const fullPath = _fs.resolveAbsPath(path);
    try {
        const raw = _fs.readFile(fullPath);
        const encoding = resolveFileEncoding(options);
        if (encoding === null || encoding === 'buffer') {
            return Buffer.from(raw);
        }
        return bytesToString(raw);
    } catch (e) {
        throw createFilesystemError(e, 'open', path, 'EIO');
    }
}

/**
 * Write file contents synchronously
 * @param {string} path - File path
 * @param {string|Buffer|TypedArray|DataView} data - Data to write
 * @param {object|string} options - Options or encoding for string data (default: 'utf8')
 */
function writeFileSync(path, data, options) {
    const bytes = fileDataToBytes(data, resolveFileEncoding(options));
    const fullPath = _fs.resolvePath(path);
    try {
        _fs.writeFile(fullPath, bytes);
    } catch (e) {
        throw createFilesystemError(e, 'open', path, 'EIO');
    }
}

class ReadStream extends EventEmitter {
    constructor(path, options) {
        super();
        this.path = path;
        this.isStdin = path === '-';
        this.fullPath = this.isStdin ? path : _fs.resolvePath(path);
        this.encoding = resolveFileEncoding(options);
        this.flags = constants.O_RDONLY;
        // Use highWaterMark for Node.js compatibility, fallback to bufferSize for backward compatibility
        this.bufferSize = options?.highWaterMark || options?.bufferSize || 64 * 1024; // 64KB (Node.js default)
        this.eof = false;
        this._decoderState = { carry: new Uint8Array(0) };
        if (this.isStdin) {
            this.reader = getProcessModule().stdin;
            return;
        }
        try {
            this.fd = _fs.open(this.fullPath, this.flags);
            this.reader = _fs.hostReader(this.fd);
        } catch (e) {
            throw createFilesystemError(e, 'open', path, 'EIO');
        }
    }
    _emitChunk(chunk) {
        if (this.encoding === null || this.encoding === 'buffer') {
            const data = toBufferChunk(chunk);
            this.emit('data', data);
            return data;
        }

        const data = isByteLike(chunk) ? decodeUtf8Chunk(chunk, this._decoderState, false) : toStringChunk(chunk);
        if (data.length > 0) {
            this.emit('data', data);
        }
        return data;
    }
    _flushStringData() {
        if (this.encoding === null || this.encoding === 'buffer') {
            return '';
        }
        const carry = this._decoderState && this._decoderState.carry ? this._decoderState.carry : new Uint8Array(0);
        if (carry.length === 0) {
            return '';
        }
        const data = decodeUtf8Chunk(new Uint8Array(0), this._decoderState, true);
        if (data.length > 0) {
            this.emit('data', data);
        }
        return data;
    }
    _readFromStdin() {
        const chunk = (this.encoding === null || this.encoding === 'buffer') && typeof this.reader.readBuffer === 'function'
            ? this.reader.readBuffer(this.bufferSize)
            : this.reader.readBytes(this.bufferSize);
        const bytesRead = chunkLength(chunk);
        if (bytesRead <= 0) {
            this.eof = true;
            this._flushStringData();
            this.emit('end');
            return bytesRead;
        }

        this._emitChunk(chunk);
        return bytesRead;
    }
    _read() {
        let bytesRead = 0;
        try {
            if (this.isStdin) {
                return this._readFromStdin();
            }

            // Allocate buffer once and reuse it
            if (!this._buffer) {
                this._buffer = new Uint8Array(this.bufferSize);
            }
            bytesRead = _fs.read(this.fd, this._buffer, 0, this.bufferSize);
            if (bytesRead <= 0) {
                this.eof = true;
                _fs.close(this.fd);
                this._flushStringData();
                this.emit('end');
                return bytesRead;
            }

            if (this.encoding === null || this.encoding === 'buffer') {
                // Must copy since buffer will be reused in next read
                const uint8Data = this._buffer.slice(0, bytesRead);
                this.emit('data', uint8Data);
                return uint8Data;
            }

            const data = this._buffer.slice(0, bytesRead);
            return this._emitChunk(data);
        } catch (e) {
            if (!this.isStdin && this.fd !== undefined) {
                _fs.close(this.fd);
            }
            if (e.message === 'EOF') {
                this.eof = true;
                this._flushStringData();
                this.emit('end');
            } else {
                this.emit('error', createFilesystemError(e, 'read', this.path, 'EIO'));
            }
            return bytesRead;
        }
    }
    _readLoop() {
        this._read();
        if (!this.eof) {
            setImmediate(() => {
                this._readLoop();
            });
        }
    }
    pipe(destination, options) {
        if (!destination || typeof destination.write !== 'function') {
            throw new TypeError('Destination must be a writable stream');
        }

        const end = options && options.end !== undefined ? options.end : true;

        this.on('data', (chunk) => {
            destination.write(chunk);
        });

        this.on('end', () => {
            if (end) {
                destination.end();
            }
        });

        this.on('error', (err) => {
            if (destination.emit) {
                destination.emit('error', err);
            }
        });

        return destination;
    }
}
/**
 * Create a read stream to a file
 * @param {string} path
 * @param {object} options
 * @returns
 */
function createReadStream(path, options) {
    const rs = new ReadStream(path, options);
    setImmediate(() => rs._readLoop());
    return rs;
}

class WriteStream extends EventEmitter {
    constructor(path, options) {
        super();
        this.path = path;
        this.fullPath = _fs.resolvePath(path);
        this.encoding = resolveFileEncoding(options);
        this.flags = constants.O_WRONLY | constants.O_CREAT | constants.O_TRUNC;
        try {
            this.fd = _fs.open(this.fullPath, this.flags, options?.mode || 0o666);
            this.writer = _fs.hostWriter(this.fd);
        } catch (e) {
            throw createFilesystemError(e, 'open', path, 'EIO');
        }
    }
    write(data) {
        try {
            const bytes = (this.encoding === null || this.encoding === 'buffer' || isByteLike(data))
                ? data
                : stringToBytes(data);
            let n = _fs.write(this.fd, bytes);
            this.emit('drain');
            return true;
        } catch (e) {
            this.emit('error', createFilesystemError(e, 'write', this.path, 'EIO'));
            return false;
        }
    }
    end(data) {
        if (data !== undefined) {
            this.write(data);
        }
        const ret = _fs.close(this.fd);
        this.emit('finish');
        return ret
    }
}

/**
 * Create a write stream to a file
 * @param {string} path 
 * @param {object} options 
 * @returns 
 */
function createWriteStream(path, options) {
    return new WriteStream(path, options);
}

/**
 * Append data to file synchronously
 * @param {string} path - File path
 * @param {string|Buffer|TypedArray|DataView} data - Data to append
 * @param {object|string} options - Options or encoding for string data (default: 'utf8')
 */
function appendFileSync(path, data, options) {
    const newBytes = fileDataToBytes(data, resolveFileEncoding(options));
    const fullPath = _fs.resolvePath(path);
    try {
        _fs.appendFile(fullPath, newBytes);
    } catch (e) {
        throw createFilesystemError(e, 'open', path, 'EIO');
    }
}

/**
 * Check if file or directory exists
 * @param {string} path - File or directory path
 * @returns {boolean} True if exists
 */
function existsSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        _fs.stat(fullPath);
        return true;
    } catch (e) {
        return false;
    }
}

/**
 * Get file or directory stats
 * @param {string} path - File or directory path
 * @returns {object} Stats object with file information
 */
function statSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        const info = _fs.stat(fullPath);
        const mode = info.mode();
        const fileType = getFileType(mode);

        return {
            isFile: () => fileType === 'file',
            isDirectory: () => fileType === 'directory',
            isSymbolicLink: () => fileType === 'symbolicLink',
            isBlockDevice: () => fileType === 'blockDevice',
            isCharacterDevice: () => fileType === 'characterDevice',
            isFIFO: () => fileType === 'fifo',
            isSocket: () => fileType === 'socket',
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            atime: info.modTime(), // jsh may not have separate atime
            ctime: info.modTime(), // jsh may not have separate ctime
            birthtime: info.modTime(),
            name: info.name()
        };
    } catch (e) {
        throw createFilesystemError(e, 'stat', path, 'ENOENT');
    }
}

function countLinesSync(path) {
    const fullPath = _fs.resolvePath(path);
    try {
        return _fs.countLines(fullPath);
    } catch (e) {
        throw createFilesystemError(e, 'open', path, 'EIO');
    }
}

/**
 * Get file or directory stats without following the final symbolic link
 * @param {string} path - File or directory path
 * @returns {object} Stats object
 */
function lstatSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        const info = _fs.lstat(fullPath);
        const mode = info.mode();
        const fileType = getFileType(mode);

        return {
            isFile: () => fileType === 'file',
            isDirectory: () => fileType === 'directory',
            isSymbolicLink: () => fileType === 'symbolicLink',
            isBlockDevice: () => fileType === 'blockDevice',
            isCharacterDevice: () => fileType === 'characterDevice',
            isFIFO: () => fileType === 'fifo',
            isSocket: () => fileType === 'socket',
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            atime: info.modTime(),
            ctime: info.modTime(),
            birthtime: info.modTime(),
            name: info.name()
        };
    } catch (e) {
        throw createFilesystemError(e, 'lstat', path, 'ENOENT');
    }
}

/**
 * Read directory contents synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (withFileTypes: boolean, recursive: boolean)
 * @returns {Array} Array of filenames or Dirent objects
 */
function readdirSync(path, options) {
    const fullPath = _fs.resolvePath(path);

    try {
        const entries = _fs.readDir(fullPath);

        if (!options?.recursive) {
            // Non-recursive mode
            if (options?.withFileTypes) {
                return entries.map((entry) => {
                    const info = entry.info();
                    const mode = info.mode();
                    const fileType = getFileType(mode);

                    return {
                        name: info.name(),
                        isFile: () => fileType === 'file',
                        isDirectory: () => fileType === 'directory',
                        isSymbolicLink: () => fileType === 'symbolicLink',
                        isBlockDevice: () => fileType === 'blockDevice',
                        isCharacterDevice: () => fileType === 'characterDevice',
                        isFIFO: () => fileType === 'fifo',
                        isSocket: () => fileType === 'socket'
                    };
                });
            } else {
                return entries.map((entry) => entry.info().name());
            }
        }

        // Recursive mode
        const results = [];
        const visited = new Set();

        const processDir = (dirPath, fullDirPath) => {
            if (visited.has(fullDirPath)) {
                return;
            }
            visited.add(fullDirPath);

            let dirEntries;
            try {
                dirEntries = _fs.readDir(fullDirPath);
            } catch (e) {
                if (e && typeof e === 'object') {
                    if (!e.path) {
                        e.path = fullDirPath;
                    }
                    if (!e.syscall) {
                        e.syscall = 'readdir';
                    }
                }
                throw e;
            }

            for (const entry of dirEntries) {
                const info = entry.info();
                const name = info.name();

                // Skip special entries
                if (name === '.' || name === '..') {
                    continue;
                }

                const mode = info.mode();
                const fileType = getFileType(mode);
                const isDir = fileType === 'directory';

                const relativePath = dirPath ? `${dirPath}/${name}` : name;

                if (options?.withFileTypes) {
                    results.push({
                        name: name,
                        isFile: () => fileType === 'file',
                        isDirectory: () => isDir,
                        isSymbolicLink: () => fileType === 'symbolicLink',
                        isBlockDevice: () => fileType === 'blockDevice',
                        isCharacterDevice: () => fileType === 'characterDevice',
                        isFIFO: () => fileType === 'fifo',
                        isSocket: () => fileType === 'socket',
                        path: dirPath || ''
                    });
                } else {
                    results.push(relativePath);
                }

                if (isDir) {
                    const fullSubPath = `${fullDirPath}/${name}`;
                    processDir(relativePath, fullSubPath);
                }
            }
        };

        processDir('', fullPath);
        return results;
    } catch (e) {
        throw createFilesystemError(e, 'readdir', path, 'EIO');
    }
}

/**
 * Create a directory synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (recursive: boolean, mode: number)
 */
function mkdirSync(path, options) {
    const fullPath = _fs.resolvePath(path);
    const mode = options?.mode === undefined ? 0o755 : options.mode;

    if (options?.recursive) {
        const parts = fullPath.split('/').filter(p => p);
        let current = '';

        for (const part of parts) {
            current += '/' + part;
            try {
                _fs.mkdir(current, mode);
            } catch (e) {
                let existingDirectory = false;
                try {
                    const info = _fs.stat(current);
                    existingDirectory = getFileType(info.mode()) === 'directory';
                } catch (statError) {
                    // Preserve the original mkdir error.
                }
                if (existingDirectory) {
                    continue;
                }
                throw createFilesystemError(e, 'mkdir', current, 'EIO');
            }
        }
        return;
    }

    try {
        _fs.mkdir(fullPath, mode);
    } catch (e) {
        throw createFilesystemError(e, 'mkdir', path, 'EIO');
    }
}

/**
 * Remove a directory synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (recursive: boolean)
 */
function rmdirSync(path, options) {
    const fullPath = _fs.resolvePath(path);
    try {
        if (options?.recursive) {
            // Never traverse through a symlink; rmdir on a link must fail without touching its target.
            if (!lstatSync(path).isDirectory()) {
                throw makeFilesystemError('ENOTDIR', 'rmdir', path);
            }
            // Remove directory and all contents
            const entries = readdirSync(path, { withFileTypes: true });
            let busyError = null;

            for (const entry of entries) {
                if (entry.name === '.' || entry.name === '..') {
                    continue;
                }
                const entryPath = path + '/' + entry.name;
                try {
                    if (entry.isDirectory()) {
                        rmdirSync(entryPath, { recursive: true });
                    } else {
                        unlinkSync(entryPath);
                    }
                } catch (err) {
                    // A nested mount point keeps its directory; finish the siblings first.
                    if (!err || err.code !== 'EBUSY') {
                        throw err;
                    }
                    busyError = busyError || err;
                }
            }
            if (busyError) {
                throw busyError;
            }
        }

        _fs.rmdir(fullPath);
    } catch (e) {
        throw createFilesystemError(e, 'rmdir', path, 'EIO');
    }
}

/**
 * Remove a file or directory synchronously (newer API)
 * @param {string} path - File or directory path
 * @param {object} options - Options (recursive: boolean, force: boolean)
 */
function rmSync(path, options) {
    try {
        const stats = lstatSync(path);
        if (stats.isDirectory()) {
            rmdirSync(path, options);
        } else {
            unlinkSync(path);
        }
    } catch (e) {
        if (!options?.force || !e || e.code !== 'ENOENT') {
            throw e;
        }
    }
}

/**
 * Delete a file synchronously
 * @param {string} path - File path
 */
function unlinkSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        _fs.unlink(fullPath);
    } catch (e) {
        throw createFilesystemError(e, 'unlink', path, 'EIO');
    }
}

/**
 * Rename a file or directory synchronously
 * @param {string} oldPath - Old path
 * @param {string} newPath - New path
 */
function renameSync(oldPath, newPath) {
    const fullOldPath = _fs.resolvePath(oldPath);
    const fullNewPath = _fs.resolvePath(newPath);

    try {
        _fs.rename(fullOldPath, fullNewPath);
    } catch (e) {
        const error = createFilesystemError(e, 'rename', oldPath, 'EIO');
        error.dest = newPath;
        error.message = `${error.code}: ${error.message.slice(error.message.indexOf(': ') + 2)} -> '${newPath}'`;
        throw error;
    }
}

/**
 * Copy a file synchronously
 * @param {string} src - Source path
 * @param {string} dest - Destination path
 * @param {number} flags - Copy flags (COPYFILE_EXCL, etc.)
 */
function copyFileSync(src, dest, flags) {
    const COPYFILE_EXCL = 1;

    // Check if destination exists when EXCL flag is set
    if (flags & COPYFILE_EXCL) {
        if (existsSync(dest)) {
            const error = new Error(`EEXIST: file already exists, copyfile '${src}' -> '${dest}'`);
            error.code = 'EEXIST';
            error.errno = -17;
            error.path = dest;
            throw error;
        }
    }

    const content = readFileSync(src, { encoding: null });
    writeFileSync(dest, content, { encoding: null });
}

/**
 * Copy file or directory synchronously
 * @param {string} src - Source path
 * @param {string} dest - Destination path
 * @param {object} options - Options (recursive: boolean, force: boolean)
 */
function cpSync(src, dest, options) {
    const srcStat = statSync(src);
    const srcReal = resolveCpPath(src);
    const destReal = resolveCpPath(dest);
    if (srcReal === destReal) {
        throw makeCpInvalidError('src and dest cannot be the same', src, dest);
    }
    if (srcStat.isDirectory() && isSameOrInsidePath(destReal, srcReal)) {
        throw makeCpInvalidError(`cannot copy ${src} to a subdirectory of self ${dest}`, src, dest);
    }
    copyTree(src, dest, srcStat, options, [srcReal]);
}

function isSameOrInsidePath(path, base) {
    return path === base || path.startsWith(base === '/' ? '/' : base + '/');
}

// Resolves symlinks of the existing part of a path so that cp can detect
// copying onto itself or into its own subtree before creating anything.
function resolveCpPath(path) {
    const fullPath = _fs.resolvePath(path);
    const parts = fullPath.split('/').filter((part) => part !== '');
    const rest = [];
    while (true) {
        const base = '/' + parts.join('/');
        try {
            const real = realpathSync(base);
            if (rest.length === 0) {
                return real;
            }
            return (real === '/' ? '' : real) + '/' + rest.join('/');
        } catch (e) {
            if (parts.length === 0) {
                return fullPath;
            }
            rest.unshift(parts.pop());
        }
    }
}

function makeCpInvalidError(reason, src, dest) {
    const error = new Error(`ERR_FS_CP_EINVAL: Invalid src or dest: cp returned EINVAL (${reason}) ${dest}`);
    error.code = 'ERR_FS_CP_EINVAL';
    error.syscall = 'cp';
    error.path = src;
    error.dest = dest;
    return error;
}

// ancestors holds the real paths of the source directories currently being
// copied, so a nested symlink pointing back into them is reported as ELOOP.
function copyTree(src, dest, srcStat, options, ancestors) {

    if (srcStat.isDirectory()) {
        if (!options?.recursive) {
            const error = new Error(`EISDIR: illegal operation on a directory, cp '${src}' -> '${dest}'`);
            error.code = 'EISDIR';
            error.errno = -21;
            error.path = src;
            error.syscall = 'cp';
            throw error;
        }

        // Create destination directory if it doesn't exist
        if (!existsSync(dest)) {
            mkdirSync(dest, { recursive: true });
        }

        // Copy all entries, excluding . and ..
        const entries = readdirSync(src, { withFileTypes: true });
        for (const entry of entries) {
            const name = entry.name;

            // Skip special entries
            if (name === '.' || name === '..') {
                continue;
            }

            const srcPath = `${src}/${name}`;
            const destPath = `${dest}/${name}`;

            if (entry.isSymbolicLink()) {
                const targetStat = statSync(srcPath);
                if (!targetStat.isDirectory()) {
                    copyFileSync(srcPath, destPath, options?.force ? 0 : 1);
                    continue;
                }
                const targetReal = realpathSync(srcPath);
                if (ancestors.some((ancestor) => isSameOrInsidePath(ancestor, targetReal))) {
                    throw makeFilesystemError('ELOOP', 'cp', srcPath);
                }
                if (isSameOrInsidePath(resolveCpPath(destPath), targetReal)) {
                    throw makeCpInvalidError(`cannot copy ${srcPath} to a subdirectory of self ${destPath}`, srcPath, destPath);
                }
                copyTree(srcPath, destPath, targetStat, options, ancestors.concat([targetReal]));
            } else if (entry.isDirectory()) {
                copyTree(srcPath, destPath, statSync(srcPath), options, ancestors.concat([realpathSync(srcPath)]));
            } else {
                copyFileSync(srcPath, destPath, options?.force ? 0 : 1);
            }
        }
    } else {
        // Copy single file
        copyFileSync(src, dest, options?.force ? 0 : 1);
    }
}

/**
 * Change file permissions synchronously
 * @param {string} path - File path
 * @param {number|string} mode - Permission mode
 */
function chmodSync(path, mode) {
    const fullPath = _fs.resolvePath(path);

    try {
        _fs.chmod(fullPath, mode);
    } catch (e) {
        throw createFilesystemError(e, 'chmod', path, 'EIO');
    }
}

/**
 * Change file owner synchronously
 * @param {string} path - File path
 * @param {number} uid - User ID
 * @param {number} gid - Group ID
 */
function chownSync(path, uid, gid) {
    const fullPath = _fs.resolvePath(path);

    try {
        _fs.chown(fullPath, uid, gid);
    } catch (e) {
        throw createFilesystemError(e, 'chown', path, 'EIO');
    }
}

/**
 * Create a symbolic link synchronously
 * @param {string} target - Link target
 * @param {string} path - Link path
 */
function symlinkSync(target, path) {
    const fullPath = _fs.resolvePath(path);

    try {
        _fs.symlink(target, fullPath);
    } catch (e) {
        const error = createFilesystemError(e, 'symlink', path, 'EACCES');
        error.message = `${error.code}: ${error.code === 'EACCES' ? 'permission denied' : error.message.slice(error.message.indexOf(': ') + 2)}, symlink '${target}' -> '${path}'`;
        throw error;
    }
}

/**
 * Read a symbolic link synchronously
 * @param {string} path - Link path
 * @returns {string} Link target
 */
function readlinkSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        return _fs.readlink(fullPath);
    } catch (e) {
        throw createFilesystemError(e, 'readlink', path, 'ENOENT');
    }
}

/**
 * Get real path (resolving symlinks) synchronously
 * @param {string} path - Path to resolve
 * @returns {string} Real path
 */
function realpathSync(path) {
    const fullPath = _fs.resolvePath(path);
    let pending = fullPath.split('/').filter((part) => part !== '' && part !== '.');
    let resolved = '/';
    let symlinkCount = 0;

    while (pending.length > 0) {
        const part = pending.shift();
        if (part === '..') {
            const separator = resolved.lastIndexOf('/');
            resolved = separator <= 0 ? '/' : resolved.slice(0, separator);
            continue;
        }

        const candidate = resolved === '/' ? `/${part}` : `${resolved}/${part}`;
        const stats = lstatSync(candidate);
        if (!stats.isSymbolicLink()) {
            resolved = candidate;
            continue;
        }

        symlinkCount++;
        if (symlinkCount > 40) {
            const error = new Error(`ELOOP: too many symbolic links encountered, realpath '${path}'`);
            error.code = 'ELOOP';
            error.errno = -40;
            error.path = path;
            error.syscall = 'realpath';
            throw error;
        }

        const target = readlinkSync(candidate);
        pending = target.split('/').filter((part) => part !== '' && part !== '.').concat(pending);
        resolved = '/';
    }

    return resolved;
}

/**
 * Access check for file/directory
 * @param {string} path - Path to check
 * @param {number} mode - Access mode (F_OK, R_OK, W_OK, X_OK)
 */
function accessSync(path, mode) {
    const F_OK = 0; // File exists
    mode = mode === undefined ? F_OK : mode;
    const fullPath = _fs.resolvePath(path);
    try {
        _fs.access(fullPath, mode);
    } catch (e) {
        throw createFilesystemError(e, 'access', path, 'EIO');
    }
}

/**
 * Truncate file to specified length
 * @param {string} path - File path
 * @param {number} len - Length to truncate to (default: 0)
 */
function truncateSync(path, len) {
    len = len === undefined || len === null ? 0 : len;
    if (!Number.isSafeInteger(len) || len < 0) {
        const error = new RangeError(`length is out of range: ${len}`);
        error.code = 'ERR_OUT_OF_RANGE';
        error.syscall = 'truncate';
        throw error;
    }

    const fullPath = _fs.resolvePath(path);
    try {
        _fs.truncate(fullPath, len);
    } catch (e) {
        throw createFilesystemError(e, 'truncate', path, 'EIO');
    }
}

/**
 * Open a file and return file descriptor
 * @param {string} path - File path
 * @param {string|number} flags - File open flags
 * @param {number} mode - File mode (default: 0o666)
 * @returns {number} File descriptor
 */
function openSync(path, flags, mode) {
    const fullPath = _fs.resolvePath(path);
    mode = mode || 0o666;

    // Convert string flags to numeric flags
    // Use the OS-specific constants from the native module
    let numFlags = 0;
    if (typeof flags === 'string') {
        switch (flags) {
            case 'r': numFlags = _fs.O_RDONLY; break;
            case 'r+': numFlags = _fs.O_RDWR; break;
            case 'w': numFlags = _fs.O_WRONLY | _fs.O_CREAT | _fs.O_TRUNC; break;
            case 'w+': numFlags = _fs.O_RDWR | _fs.O_CREAT | _fs.O_TRUNC; break;
            case 'a': numFlags = _fs.O_WRONLY | _fs.O_CREAT | _fs.O_APPEND; break;
            case 'a+': numFlags = _fs.O_RDWR | _fs.O_CREAT | _fs.O_APPEND; break;
            case 'wx': numFlags = _fs.O_WRONLY | _fs.O_CREAT | _fs.O_TRUNC | _fs.O_EXCL; break;
            case 'wx+': numFlags = _fs.O_RDWR | _fs.O_CREAT | _fs.O_TRUNC | _fs.O_EXCL; break;
            case 'ax': numFlags = _fs.O_WRONLY | _fs.O_CREAT | _fs.O_APPEND | _fs.O_EXCL; break;
            case 'ax+': numFlags = _fs.O_RDWR | _fs.O_CREAT | _fs.O_APPEND | _fs.O_EXCL; break;
            default: numFlags = _fs.O_RDONLY; break;
        }
    } else {
        numFlags = flags;
    }

    try {
        const fd = _fs.open(fullPath, numFlags, mode);
        return fd;
    } catch (e) {
        throw createFilesystemError(e, 'open', path, 'EIO');
    }
}

/**
 * Close a file descriptor
 * @param {number} fd - File descriptor
 */
function closeSync(fd) {
    try {
        _fs.close(fd);
    } catch (e) {
        throw createDescriptorError(e, 'close');
    }
}

function describeReceived(value) {
    if (value === null || value === undefined) {
        return `${value}`;
    }
    if (typeof value === 'function') {
        return `function ${value.name || '<anonymous>'}`;
    }
    if (typeof value === 'object') {
        const name = value.constructor && value.constructor.name;
        return name ? `an instance of ${name}` : 'an object';
    }
    return `type ${typeof value} (${String(value)})`;
}

function makeInvalidArgTypeError(name, expected, value) {
    const error = new TypeError(`The "${name}" argument must be ${expected}. Received ${describeReceived(value)}`);
    error.code = 'ERR_INVALID_ARG_TYPE';
    return error;
}

// Offsets and lengths are byte based for every view type, as in Node.js.
function viewToBytes(view) {
    return view instanceof Uint8Array
        ? view
        : new Uint8Array(view.buffer, view.byteOffset, view.byteLength);
}

// Returns a Uint8Array sharing memory with an ArrayBufferView or ArrayBuffer, or undefined.
// Accepting ArrayBuffer is a JSH extension beyond Node.js because native modules
// (zlib, archive, mqtt, ...) return ArrayBuffer.
function binaryToBytes(value) {
    if (ArrayBuffer.isView(value)) {
        return viewToBytes(value);
    }
    if (value instanceof ArrayBuffer) {
        return new Uint8Array(value);
    }
    return undefined;
}

/**
 * Read from a file descriptor
 * @param {number} fd - File descriptor
 * @param {Buffer|TypedArray|DataView|ArrayBuffer} buffer - Buffer to read into
 * @param {number} offset - Byte offset in buffer to start writing (default: 0)
 * @param {number} length - Number of bytes to read
 * @param {number} position - Position to read from (null = current position)
 * @returns {number} Number of bytes read
 */
function readSync(fd, buffer, offset, length, position) {
    const bytes = binaryToBytes(buffer);
    if (!bytes) {
        throw makeInvalidArgTypeError('buffer', 'an instance of Buffer, TypedArray, DataView, or ArrayBuffer', buffer);
    }

    offset = offset === undefined || offset === null ? 0 : offset;
    length = length === undefined || length === null ? bytes.length - offset : length;

    if (!Number.isInteger(offset) || offset < 0 || offset > bytes.length) {
        const error = new RangeError(`offset is out of range: ${offset}`);
        error.code = 'ERR_OUT_OF_RANGE';
        error.syscall = 'read';
        throw error;
    }
    if (!Number.isInteger(length) || length < 0 || length > bytes.length - offset) {
        const error = new RangeError(`length is out of range: ${length}`);
        error.code = 'ERR_OUT_OF_RANGE';
        error.syscall = 'read';
        throw error;
    }
    if (position !== undefined && position !== null && (!Number.isSafeInteger(position) || position < 0)) {
        const error = new RangeError(`position is out of range: ${position}`);
        error.code = 'ERR_OUT_OF_RANGE';
        error.syscall = 'read';
        throw error;
    }

    try {
        // _fs.read will copy data into the buffer (Array or Uint8Array)
        // at the specified offset
        const bytesRead = position === undefined || position === null
            ? _fs.read(fd, bytes, offset, length)
            : _fs.readAt(fd, bytes, offset, length, position);
        return bytesRead;
    } catch (e) {
        throw createDescriptorError(e, 'read');
    }
}

function makeWriteRangeError(name, value) {
    const error = new RangeError(`${name} is out of range: ${value}`);
    error.code = 'ERR_OUT_OF_RANGE';
    error.syscall = 'write';
    return error;
}

/**
 * Write to a file descriptor.
 *
 * Supported forms (Node.js compatible):
 *   writeSync(fd, buffer[, offset[, length[, position]]])
 *   writeSync(fd, buffer[, options])  // options: { offset, length, position }
 *   writeSync(fd, string[, position[, encoding]])
 *   writeSync(fd, string, offset, length[, position])  // JSH legacy form (UTF-8 byte offset/length)
 *
 * @param {number} fd - File descriptor
 * @param {string|Buffer|TypedArray|DataView|ArrayBuffer|number[]} buffer - Data to write
 * @param {number|object|null} offset - Buffer offset, options object, or file position for strings
 * @param {number|string} length - Number of bytes to write, or encoding for strings
 * @param {number|null} position - Position to write to (null = current position)
 * @returns {number} Number of bytes written
 */
function writeSync(fd, buffer, offset, length, position) {
    // JSH extension beyond Node.js: plain byte arrays are written like Uint8Array.
    let data = binaryToBytes(buffer) || (Array.isArray(buffer) ? Uint8Array.from(buffer) : undefined);
    if (data) {
        if (offset === null || typeof offset === 'object') {
            const options = offset || {};
            offset = options.offset === undefined ? 0 : options.offset;
            length = options.length === undefined ? data.length - offset : options.length;
            position = options.position === undefined ? null : options.position;
        }
        if (typeof offset !== 'number') {
            offset = 0;
        }
        if (typeof length !== 'number') {
            length = data.length - offset;
        }
    } else if (typeof buffer === 'string' && typeof length === 'number') {
        // JSH extension: legacy form writeSync(fd, string, offset, length[, position]).
        // Node.js treats the fourth argument as an encoding name, so a number never occurs there.
        data = stringToBytes(buffer);
        if (typeof offset !== 'number') {
            offset = 0;
        }
    } else if (typeof buffer === 'string') {
        position = offset;
        const name = length === undefined || length === null ? 'utf8' : normalizeEncodingName(length);
        if (name === 'hex' && buffer.length % 2 !== 0) {
            const error = new TypeError(`The argument 'encoding' is invalid for data of length ${buffer.length}. Received 'hex'`);
            error.code = 'ERR_INVALID_ARG_VALUE';
            throw error;
        }
        // Like Node.js, an unrecognized encoding for string writes falls back to UTF-8.
        data = encodeString(buffer, name || 'utf8');
        offset = 0;
        length = data.length;
    } else {
        throw makeInvalidArgTypeError('buffer', 'of type string, Array, or an instance of Buffer, TypedArray, DataView, or ArrayBuffer', buffer);
    }

    if (!Number.isInteger(offset) || offset < 0 || offset > data.length) {
        throw makeWriteRangeError('offset', offset);
    }
    if (!Number.isInteger(length) || length < 0 || length > data.length - offset) {
        throw makeWriteRangeError('length', length);
    }
    if (typeof position !== 'number') {
        position = null;
    } else if (!Number.isSafeInteger(position) || position < 0) {
        throw makeWriteRangeError('position', position);
    }

    const writeData = offset === 0 && length === data.length ? data : data.subarray(offset, offset + length);

    try {
        return _fs.write(fd, writeData, position);
    } catch (e) {
        throw createDescriptorError(e, 'write');
    }
}

/**
 * Get file status from file descriptor
 * @param {number} fd - File descriptor
 * @returns {object} Stats object
 */
function fstatSync(fd) {
    try {
        const info = _fs.fstat(fd);
        const mode = info.mode();
        const fileType = getFileType(mode);

        return {
            name: info.name(),
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            isFile: () => fileType === 'file',
            isDirectory: () => fileType === 'directory',
            isSymbolicLink: () => fileType === 'symbolicLink',
            isBlockDevice: () => fileType === 'blockDevice',
            isCharacterDevice: () => fileType === 'characterDevice',
            isFIFO: () => fileType === 'fifo',
            isSocket: () => fileType === 'socket'
        };
    } catch (e) {
        throw createDescriptorError(e, 'fstat');
    }
}

/**
 * Change file permissions via file descriptor
 * @param {number} fd - File descriptor
 * @param {number|string} mode - Permission mode
 */
function fchmodSync(fd, mode) {
    try {
        _fs.fchmod(fd, mode);
    } catch (e) {
        throw createDescriptorError(e, 'fchmod');
    }
}

/**
 * Change file owner via file descriptor
 * @param {number} fd - File descriptor
 * @param {number} uid - User ID
 * @param {number} gid - Group ID
 */
function fchownSync(fd, uid, gid) {
    try {
        _fs.fchown(fd, uid, gid);
    } catch (e) {
        throw createDescriptorError(e, 'fchown');
    }
}

/**
 * Synchronize file data to storage
 * @param {number} fd - File descriptor
 */
function fsyncSync(fd) {
    try {
        _fs.fsync(fd);
    } catch (e) {
        throw createDescriptorError(e, 'fsync');
    }
}

/**
 * Synchronize file data (but not metadata) to storage
 * @param {number} fd - File descriptor
 */
function fdatasyncSync(fd) {
    // Go's file.Sync() doesn't distinguish between fsync and fdatasync
    // So we just call fsync
    fsyncSync(fd);
}

function isSharedWriteConflictError(error) {
    const message = error && error.message ? String(error.message) : String(error || '');
    return message.indexOf('ECONFLICT') >= 0 || message.indexOf('shared file changed while descriptor was open') >= 0;
}

// Constants
// Note: O_* constants are now sourced from the native module (_fs)
// to ensure platform-specific values are used correctly
const constants = {
    // File Access Constants
    F_OK: 0,
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,

    // File Copy Constants
    COPYFILE_EXCL: 1,
    COPYFILE_FICLONE: 2,
    COPYFILE_FICLONE_FORCE: 4,

    // File Open Constants - use OS-specific values from native module
    O_RDONLY: _fs.O_RDONLY,
    O_WRONLY: _fs.O_WRONLY,
    O_RDWR: _fs.O_RDWR,
    O_CREAT: _fs.O_CREAT,
    O_EXCL: _fs.O_EXCL,
    O_TRUNC: _fs.O_TRUNC,
    O_APPEND: _fs.O_APPEND,
};

function platform() {
    return _fs.platform();
}
function arch() {
    return _fs.arch();
}

// Export all functions
module.exports = {
    resolvePath: _fs.resolvePath,
    resolveAbsPath: _fs.resolveAbsPath,
    resolveHostPath: _fs.resolveHostPath,

    // File operations
    readFileSync,
    writeFileSync,
    appendFileSync,
    copyFileSync,
    cpSync,
    unlinkSync,
    renameSync,
    truncateSync,

    // Directory operations
    readdirSync,
    mkdirSync,
    rmdirSync,
    rmSync,

    // File info
    statSync,
    lstatSync,
    existsSync,
    accessSync,
    countLinesSync,

    // helper functions
    platform,
    arch,

    // Symlink operations
    symlinkSync,
    readlinkSync,
    realpathSync,

    // Permissions
    chmodSync,
    chownSync,

    // File descriptor operations
    openSync,
    closeSync,
    readSync,
    writeSync,
    fstatSync,
    fchmodSync,
    fchownSync,
    fsyncSync,
    fdatasyncSync,

    // Stream operations
    createWriteStream,
    createReadStream,

    // Constants
    constants,

    // Aliases for Node.js compatibility
    readFile: readFileSync,
    writeFile: writeFileSync,
    appendFile: appendFileSync,
    copyFile: copyFileSync,
    cp: cpSync,
    unlink: unlinkSync,
    rename: renameSync,
    readdir: readdirSync,
    mkdir: mkdirSync,
    rmdir: rmdirSync,
    rm: rmSync,
    stat: statSync,
    lstat: lstatSync,
    countLines: countLinesSync,
    exists: existsSync,
    access: accessSync,
    symlink: symlinkSync,
    readlink: readlinkSync,
    realpath: realpathSync,
    chmod: chmodSync,
    chown: chownSync,
    truncate: truncateSync,
    open: openSync,
    close: closeSync,
    read: readSync,
    write: writeSync,
    fstat: fstatSync,
    fchmod: fchmodSync,
    fchown: fchownSync,
    fsync: fsyncSync,
    fdatasync: fdatasyncSync
};
