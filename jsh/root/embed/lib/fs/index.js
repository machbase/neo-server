'use strict';

const _fs = require('@jsh/fs');
const EventEmitter = require('events');

// Polyfill for Buffer.isBuffer if not available
if (!Buffer.isBuffer) {
    Buffer.isBuffer = function (obj) {
        return obj && obj.constructor && obj.constructor.name === 'Buffer';
    };
}

// Convert byte array to string
function bytesToString(bytes) {
    return String.fromCharCode(...bytes);
}

// Convert string to byte array
function stringToBytes(str) {
    const bytes = [];
    for (let i = 0; i < str.length; i++) {
        bytes.push(str.charCodeAt(i));
    }
    return bytes;
}

/**
 * Read file contents synchronously
 * @param {string} path - File path
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 * @returns {string|Array} File contents as string or byte array
 */
function readFileSync(path, options) {
    const fullPath = _fs.resolveAbsPath(path);
    try {
        const raw = _fs.readFile(fullPath);
        // Default to utf8 encoding if not specified
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        if (encoding === null || encoding === 'buffer') {
            return raw;
        }
        return bytesToString(raw);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, open '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        error.syscall = 'open';
        throw error;
    }
}

/**
 * Write file contents synchronously
 * @param {string} path - File path
 * @param {string|Array} data - Data to write (string or byte array)
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 */
function writeFileSync(path, data, options) {
    const fullPath = _fs.resolvePath(path);
    try {
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        const bytes = (encoding === null || encoding === 'buffer' || Array.isArray(data))
            ? data
            : stringToBytes(data);
        _fs.writeFile(fullPath, bytes);
    } catch (e) {
        const error = new Error(`EACCES: permission denied, open '${path}'`);
        error.code = 'EACCES';
        error.errno = -13;
        error.path = path;
        error.syscall = 'open';
        throw error;
    }
}

class ReadStream extends EventEmitter {
    constructor(path, options) {
        super();
        this.fullPath = _fs.resolvePath(path);
        this.encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        this.flags = constants.O_RDONLY;
        // Use highWaterMark for Node.js compatibility, fallback to bufferSize for backward compatibility
        this.bufferSize = options?.highWaterMark || options?.bufferSize || 64 * 1024; // 64KB (Node.js default)
        this.eof = false;
        try {
            this.fd = _fs.open(this.fullPath, this.flags);
            this.reader = _fs.hostReader(this.fd);
        } catch (e) {
            const error = new Error(`EACCES: permission denied, open '${path}'`);
            error.code = 'EACCES';
            error.errno = -13;
            error.path = path;
            error.syscall = 'open';
            throw error;
        }
    }
    _read() {
        try {
            // Allocate buffer once and reuse it
            if (!this._buffer) {
                // Use Array for compatibility with _fs.read()
                this._buffer = Buffer.alloc(this.bufferSize);
            }
            const bytesRead = _fs.read(this.fd, this._buffer, 0, this.bufferSize);
            if (bytesRead <= 0) {
                this.eof = true;
                _fs.close(this.fd);
                this.emit('end');
                return bytesRead;
            }

            if (this.encoding === null || this.encoding === 'buffer') {
                // Create Uint8Array view directly from buffer without slice
                // Must copy since buffer will be reused in next read
                const uint8Data = new Uint8Array(bytesRead);
                for (let i = 0; i < bytesRead; i++) {
                    uint8Data[i] = this._buffer[i];
                }
                this.emit('data', uint8Data);
                return uint8Data;
            } else {
                // For string mode, convert only the bytes read
                let str = '';
                for (let i = 0; i < bytesRead; i++) {
                    str += String.fromCharCode(this._buffer[i]);
                }
                this.emit('data', str);
                return str;
            }
        } catch (e) {
            _fs.close(this.fd);
            if (e.message === 'EOF') {
                this.eof = true;
                this.emit('end');
            } else {
                this.emit('error', e);
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
        this.fullPath = _fs.resolvePath(path);
        this.encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        this.flags = constants.O_WRONLY | constants.O_CREAT | constants.O_TRUNC;
        try {
            this.fd = _fs.open(this.fullPath, this.flags, options?.mode || 0o666);
            this.writer = _fs.hostWriter(this.fd);
        } catch (e) {
            const error = new Error(`EACCES: permission denied, open '${path}'`);
            error.code = 'EACCES';
            error.errno = -13;
            error.path = path;
            error.syscall = 'open';
            throw error;
        }
    }
    write(data) {
        try {
            const bytes = (this.encoding === null || this.encoding === 'buffer' || Array.isArray(data))
                ? data
                : stringToBytes(data);
            let n = _fs.write(this.fd, bytes);
            this.emit('drain');
            return true;
        } catch (e) {
            this.emit('error', e);
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
 * @param {string|Array} data - Data to append
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 */
function appendFileSync(path, data, options) {
    const fullPath = _fs.resolvePath(path);
    try {
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        const newBytes = (encoding === null || encoding === 'buffer' || Array.isArray(data))
            ? data
            : stringToBytes(data);
        _fs.appendFile(fullPath, newBytes);
    } catch (e) {
        const error = new Error(`EACCES: permission denied, open '${path}'`);
        error.code = 'EACCES';
        error.errno = -13;
        error.path = path;
        error.syscall = 'open';
        throw error;
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
        const modeStr = mode.string();

        return {
            isFile: () => !modeStr.startsWith('d') && !modeStr.startsWith('l'),
            isDirectory: () => modeStr.startsWith('d'),
            isSymbolicLink: () => modeStr.startsWith('l'),
            isBlockDevice: () => modeStr.startsWith('b'),
            isCharacterDevice: () => modeStr.startsWith('c'),
            isFIFO: () => modeStr.startsWith('p'),
            isSocket: () => modeStr.startsWith('s'),
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            atime: info.modTime(), // jsh may not have separate atime
            ctime: info.modTime(), // jsh may not have separate ctime
            birthtime: info.modTime(),
            name: info.name()
        };
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, stat '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

function countLinesSync(path) {
    const fullPath = _fs.resolvePath(path);
    try {
        return _fs.countLines(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, open '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        error.syscall = 'open';
        throw error;
    }
}

/**
 * Get file or directory stats (follows symlinks)
 * @param {string} path - File or directory path
 * @returns {object} Stats object
 */
function lstatSync(path) {
    // For now, same as statSync since we don't have explicit lstat support
    return statSync(path);
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
                    const modeStr = mode.string();

                    return {
                        name: info.name(),
                        isFile: () => !modeStr.startsWith('d') && !modeStr.startsWith('l'),
                        isDirectory: () => modeStr.startsWith('d'),
                        isSymbolicLink: () => modeStr.startsWith('l'),
                        isBlockDevice: () => modeStr.startsWith('b'),
                        isCharacterDevice: () => modeStr.startsWith('c'),
                        isFIFO: () => modeStr.startsWith('p'),
                        isSocket: () => modeStr.startsWith('s')
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

            try {
                const dirEntries = _fs.readDir(fullDirPath);

                for (const entry of dirEntries) {
                    const info = entry.info();
                    const name = info.name();

                    // Skip special entries
                    if (name === '.' || name === '..') {
                        continue;
                    }

                    const mode = info.mode();
                    const modeStr = mode.string();
                    const isDir = modeStr.startsWith('d');

                    const relativePath = dirPath ? `${dirPath}/${name}` : name;

                    if (options?.withFileTypes) {
                        results.push({
                            name: name,
                            isFile: () => !isDir && !modeStr.startsWith('l'),
                            isDirectory: () => isDir,
                            isSymbolicLink: () => modeStr.startsWith('l'),
                            isBlockDevice: () => modeStr.startsWith('b'),
                            isCharacterDevice: () => modeStr.startsWith('c'),
                            isFIFO: () => modeStr.startsWith('p'),
                            isSocket: () => modeStr.startsWith('s'),
                            path: dirPath || ''
                        });
                    } else {
                        results.push(relativePath);
                    }

                    // Recurse into subdirectories
                    if (isDir) {
                        const fullSubPath = `${fullDirPath}/${name}`;
                        processDir(relativePath, fullSubPath);
                    }
                }
            } catch (e) {
                // Ignore errors reading subdirectories
            }
        };

        processDir('', fullPath);
        return results;
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, readdir '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        error.syscall = 'readdir';
        throw error;
    }
}

/**
 * Create a directory synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (recursive: boolean, mode: number)
 */
function mkdirSync(path, options) {
    const fullPath = _fs.resolvePath(path);

    try {
        if (options?.recursive) {
            // Create parent directories if needed
            const parts = fullPath.split('/').filter(p => p);
            let current = '/';

            for (const part of parts) {
                current += part + '/';
                try {
                    _fs.mkdir(current.slice(0, -1));
                } catch (e) {
                    // Directory may already exist, continue
                }
            }
        } else {
            _fs.mkdir(fullPath);
        }
    } catch (e) {
        if (!options?.recursive || !existsSync(path)) {
            const error = new Error(`EACCES: permission denied, mkdir '${path}'`);
            error.code = 'EACCES';
            error.errno = -13;
            error.path = path;
            throw error;
        }
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
            // Remove directory and all contents
            const entries = readdirSync(path, { withFileTypes: true });

            for (const entry of entries) {
                if (entry.name === '.' || entry.name === '..') {
                    continue;
                }
                const entryPath = path + '/' + entry.name;
                if (entry.isDirectory()) {
                    rmdirSync(entryPath, { recursive: true });
                } else {
                    unlinkSync(entryPath);
                }
            }
        }

        _fs.rmdir(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, rmdir '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Remove a file or directory synchronously (newer API)
 * @param {string} path - File or directory path
 * @param {object} options - Options (recursive: boolean, force: boolean)
 */
function rmSync(path, options) {
    try {
        const stats = statSync(path);
        if (stats.isDirectory()) {
            rmdirSync(path, options);
        } else {
            unlinkSync(path);
        }
    } catch (e) {
        if (!options?.force) {
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
        _fs.remove(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, unlink '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
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
        const error = new Error(`ENOENT: no such file or directory, rename '${oldPath}' -> '${newPath}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = oldPath;
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

            if (entry.isDirectory()) {
                cpSync(srcPath, destPath, options);
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
        const error = new Error(`ENOENT: no such file or directory, chmod '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
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
        const error = new Error(`ENOENT: no such file or directory, chown '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
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
        const error = new Error(`EACCES: permission denied, symlink '${target}' -> '${path}'`);
        error.code = 'EACCES';
        error.errno = -13;
        error.path = path;
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
        const error = new Error(`ENOENT: no such file or directory, readlink '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Get real path (resolving symlinks) synchronously
 * @param {string} path - Path to resolve
 * @returns {string} Real path
 */
function realpathSync(path) {
    const fullPath = _fs.resolvePath(path);

    try {
        // Try to resolve symlinks
        let current = fullPath;
        let visited = new Set();

        while (true) {
            if (visited.has(current)) {
                // Circular symlink
                const error = new Error(`ELOOP: too many symbolic links encountered, realpath '${path}'`);
                error.code = 'ELOOP';
                error.errno = -40;
                error.path = path;
                throw error;
            }

            visited.add(current);

            try {
                const stats = statSync(current);
                if (!stats.isSymbolicLink()) {
                    return current;
                }
                current = readlinkSync(current);
            } catch (e) {
                return current;
            }
        }
    } catch (e) {
        throw e;
    }
}

/**
 * Access check for file/directory
 * @param {string} path - Path to check
 * @param {number} mode - Access mode (F_OK, R_OK, W_OK, X_OK)
 */
function accessSync(path, mode) {
    const F_OK = 0; // File exists
    const R_OK = 4; // Read permission
    const W_OK = 2; // Write permission
    const X_OK = 1; // Execute permission

    if (!existsSync(path)) {
        const error = new Error(`ENOENT: no such file or directory, access '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }

    // For simplicity, we assume if file exists, we have access
    // A real implementation would check actual permissions
}

/**
 * Truncate file to specified length
 * @param {string} path - File path
 * @param {number} len - Length to truncate to (default: 0)
 */
function truncateSync(path, len) {
    len = len || 0;

    if (len === 0) {
        writeFileSync(path, '', 'utf8');
    } else {
        const content = readFileSync(path, { encoding: null });
        if (content.length > len) {
            writeFileSync(path, content.slice(0, len), { encoding: null });
        }
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
        const error = new Error(`ENOENT: no such file or directory, open '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        error.syscall = 'open';
        throw error;
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
        const error = new Error(`EBADF: bad file descriptor, close`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'close';
        throw error;
    }
}

/**
 * Read from a file descriptor
 * @param {number} fd - File descriptor
 * @param {Array|Buffer} buffer - Buffer or Array to read into
 * @param {number} offset - Offset in buffer to start writing (default: 0)
 * @param {number} length - Number of bytes to read
 * @param {number} position - Position to read from (null = current position)
 * @returns {number} Number of bytes read
 */
function readSync(fd, buffer, offset, length, position) {
    offset = offset || 0;

    try {
        // _fs.read will copy data into the buffer (Array or Uint8Array)
        // at the specified offset
        const bytesRead = _fs.read(fd, buffer, offset, length);
        return bytesRead;
    } catch (e) {
        const error = new Error(`EBADF: bad file descriptor, read`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'read';
        throw error;
    }
}

/**
 * Write to a file descriptor
 * @param {number} fd - File descriptor
 * @param {Array|string} buffer - Data to write
 * @param {number} offset - Offset in buffer to start reading (default: 0)
 * @param {number} length - Number of bytes to write (default: buffer.length - offset)
 * @param {number} position - Position to write to (null = current position)
 * @returns {number} Number of bytes written
 */
function writeSync(fd, buffer, offset, length, position) {
    offset = offset || 0;

    let data;
    if (typeof buffer === 'string') {
        // Convert string to bytes
        data = stringToBytes(buffer);
        length = length || data.length;
    } else {
        data = buffer;
        length = length || (buffer.length - offset);
    }

    // Extract the portion to write
    const writeData = data.slice(offset, offset + length);

    try {
        return _fs.write(fd, writeData);
    } catch (e) {
        const error = new Error(`EBADF: bad file descriptor, write`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'write';
        throw error;
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
        const modeStr = mode.string();

        return {
            name: info.name(),
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            isFile: () => !modeStr.startsWith('d') && !modeStr.startsWith('l'),
            isDirectory: () => modeStr.startsWith('d'),
            isSymbolicLink: () => modeStr.startsWith('l'),
            isBlockDevice: () => modeStr.startsWith('b'),
            isCharacterDevice: () => modeStr.startsWith('c'),
            isFIFO: () => modeStr.startsWith('p'),
            isSocket: () => modeStr.startsWith('s')
        };
    } catch (e) {
        const error = new Error(`EBADF: bad file descriptor, fstat`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'fstat';
        throw error;
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
        const error = new Error(`EBADF: bad file descriptor, fchmod`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'fchmod';
        throw error;
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
        const error = new Error(`EBADF: bad file descriptor, fchown`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'fchown';
        throw error;
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
        const error = new Error(`EBADF: bad file descriptor, fsync`);
        error.code = 'EBADF';
        error.errno = -9;
        error.syscall = 'fsync';
        throw error;
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

// Export all functions
module.exports = {
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
