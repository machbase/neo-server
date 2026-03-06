'use strict';

const _net = require('@jsh/net');
const EventEmitter = require('events');
const Stream = require('stream');
const process = require('process');
const { Buffer } = require('buffer');

// Helper function to encode strings to UTF-8 byte arrays
function stringToBytes(str, encoding) {
    encoding = encoding || 'utf8';

    if (encoding === 'utf8' || encoding === 'utf-8') {
        // UTF-8 encoding
        const bytes = [];
        for (let i = 0; i < str.length; i++) {
            let charCode = str.charCodeAt(i);

            if (charCode < 0x80) {
                // Single byte (ASCII)
                bytes.push(charCode);
            } else if (charCode < 0x800) {
                // Two bytes
                bytes.push(0xC0 | (charCode >> 6));
                bytes.push(0x80 | (charCode & 0x3F));
            } else if (charCode < 0x10000) {
                // Three bytes
                bytes.push(0xE0 | (charCode >> 12));
                bytes.push(0x80 | ((charCode >> 6) & 0x3F));
                bytes.push(0x80 | (charCode & 0x3F));
            } else {
                // Four bytes (for surrogate pairs)
                bytes.push(0xF0 | (charCode >> 18));
                bytes.push(0x80 | ((charCode >> 12) & 0x3F));
                bytes.push(0x80 | ((charCode >> 6) & 0x3F));
                bytes.push(0x80 | (charCode & 0x3F));
            }
        }
        return bytes;
    } else if (encoding === 'ascii') {
        // ASCII encoding - only first 7 bits
        const bytes = [];
        for (let i = 0; i < str.length; i++) {
            bytes.push(str.charCodeAt(i) & 0x7F);
        }
        return bytes;
    } else if (encoding === 'latin1' || encoding === 'binary') {
        // Latin-1 / Binary - first 8 bits only
        const bytes = [];
        for (let i = 0; i < str.length; i++) {
            bytes.push(str.charCodeAt(i) & 0xFF);
        }
        return bytes;
    } else if (encoding === 'hex') {
        // Hex encoding
        const bytes = [];
        for (let i = 0; i < str.length; i += 2) {
            bytes.push(parseInt(str.substr(i, 2), 16));
        }
        return bytes;
    } else if (encoding === 'base64') {
        // Base64 decoding
        const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
        const bytes = [];
        let i = 0;

        while (i < str.length) {
            const a = chars.indexOf(str[i++]);
            const b = chars.indexOf(str[i++]);
            const c = chars.indexOf(str[i++]);
            const d = chars.indexOf(str[i++]);

            bytes.push((a << 2) | (b >> 4));
            if (c !== -1) bytes.push(((b & 15) << 4) | (c >> 2));
            if (d !== -1) bytes.push(((c & 3) << 6) | d);
        }
        return bytes;
    }

    // Default to UTF-8 for unknown encodings
    return stringToBytes(str, 'utf8');
}

// Server class
class Server extends EventEmitter {
    constructor(options, connectionListener) {
        super();

        if (typeof options === 'function') {
            connectionListener = options;
            options = {};
        }

        this.options = options || {};
        this._raw = _net.CreateServer(this, process.dispatchEvent);

        this.listening = false;
        this.connections = 0;

        this.on('accept', (nativeSocket) => {
            const socket = new Socket();
            socket._attachNativeSocket(nativeSocket);
            this.connections++;

            this.emit('connection', socket);
        });

        if (connectionListener) {
            this.on('connection', connectionListener);
        }
    }

    listen(...args) {
        // listen(port[, host][, backlog][, callback])
        // listen(options[, callback])
        let port = 0;
        let host = '';
        let backlog = 511;
        let callback = null;

        if (args.length === 0) {
            throw new Error('listen() requires at least one argument');
        }

        // Parse arguments
        if (typeof args[0] === 'object') {
            const options = args[0];
            port = options.port || 0;
            host = options.host || '';
            backlog = options.backlog || 511;
            if (typeof args[1] === 'function') {
                callback = args[1];
            }
        } else {
            port = args[0];
            let idx = 1;

            if (typeof args[idx] === 'string') {
                host = args[idx];
                idx++;
            }

            if (typeof args[idx] === 'number') {
                backlog = args[idx];
                idx++;
            }

            if (typeof args[idx] === 'function') {
                callback = args[idx];
            }
        }

        if (callback) {
            this.once('listening', callback);
        }

        try {
            this._raw.listen(port, host, backlog);
            this.listening = true;

            const interval = setInterval(() => {
                if (!this.listening) {
                    clearInterval(interval);
                }
            }, 200);
        } catch (err) {
            this.emit('error', err);
            throw err;
        }

        return this;
    }

    close(callback) {
        if (callback) {
            this.once('close', callback);
        }

        try {
            this._raw.close();
            this.listening = false;
        } catch (err) {
            this.emit('error', err);
        }

        return this;
    }

    address() {
        try {
            return this._raw.address();
        } catch (err) {
            return null;
        }
    }

    getConnections(callback) {
        const count = this._raw.getConnections();
        if (callback) {
            process.nextTick(() => callback(null, count));
        }
        return count;
    }

    ref() {
        return this;
    }

    unref() {
        return this;
    }
}

// Socket class
class Socket extends EventEmitter {
    constructor(options) {
        super();

        this.options = options || {};
        this._raw = null;
        this.connecting = false;
        this.readable = true;
        this.writable = true;
        this.destroyed = false;
        this.bytesRead = 0;
        this.bytesWritten = 0;
        this.localAddress = null;
        this.localPort = null;
        this.remoteAddress = null;
        this.remotePort = null;
        this.remoteFamily = null;
        this._encoding = null; // null means emit Buffer, string means emit string with that encoding
    }

    _attachNativeSocket(nativeSocket) {        
        this._raw = nativeSocket;
        
        // Wrap the emit to transform data to Buffer objects
        const originalEmit = this.emit.bind(this);
        this.emit = function (event, ...args) {
            if (event === 'data' && args.length > 0) {
                const data = args[0];
                // Update bytes read counter and convert to Buffer
                if (Array.isArray(data)) {
                    args[0] = Buffer.from(data);
                    this.bytesRead += data.length;
                } else if (typeof data === 'string') {
                    args[0] = Buffer.from(data, this._encoding || 'utf8');
                    this.bytesRead += data.length;
                }
            }
            return originalEmit(event, ...args);
        };
        
        // Set up javascript object reference in native socket
        nativeSocket.setObject(this);
        
        this._setupRaw();
        this.connecting = false;
        this.readable = true;
        this.writable = true;
        
        // Start the read loop now that event handlers are set up
        nativeSocket.startReading();
    }

    _setupRaw() {
        if (!this._raw) return;

        // Update address information
        try {
            const local = this._raw.address();
            if (local) {
                this.localAddress = local.address;
                this.localPort = local.port;
            }

            const remote = this._raw.remoteAddress();
            if (remote) {
                this.remoteAddress = remote.address;
                this.remotePort = remote.port;
                this.remoteFamily = remote.family || 'IPv4';
            }
        } catch (err) {
            // Ignore errors getting addresses
        }
    }

    connect(...args) {
        // connect(port[, host][, connectListener])
        // connect(options[, connectListener])
        let port = 0;
        let host = 'localhost';
        let connectListener = null;

        if (args.length === 0) {
            throw new Error('connect() requires at least one argument');
        }

        // Parse arguments
        if (typeof args[0] === 'object') {
            const options = args[0];
            port = options.port;
            host = options.host || 'localhost';
            if (typeof args[1] === 'function') {
                connectListener = args[1];
            }
        } else {
            port = args[0];
            let idx = 1;

            if (typeof args[idx] === 'string') {
                host = args[idx];
                idx++;
            }

            if (typeof args[idx] === 'function') {
                connectListener = args[idx];
            }
        }

        if (connectListener) {
            this.once('connect', connectListener);
        }

        this.connecting = true;

        // Set up event handlers BEFORE calling Connect
        const connectHandler = () => {
            this.connecting = false;
            this._setupRaw();
        };

        // Wrap the emit to transform data to Buffer objects
        const originalEmit = this.emit.bind(this);
        this.emit = function (event, ...args) {
            if (event === 'data' && args.length > 0) {
                const data = args[0];
                // Update bytes read counter and convert to Buffer
                if (Array.isArray(data)) {
                    args[0] = Buffer.from(data);
                    this.bytesRead += data.length;
                } else if (typeof data === 'string') {
                    args[0] = Buffer.from(data, this._encoding || 'utf8');
                    this.bytesRead += data.length;
                }
            }
            return originalEmit(event, ...args);
        };

        const endHandler = () => {
            this.readable = false;
        };

        const closeHandler = () => {
            this.readable = false;
            this.writable = false;
        };

        const errorHandler = () => {
            this.connecting = false;
        };

        this.on('connect', connectHandler);
        this.on('end', endHandler);
        this.on('close', closeHandler);
        this.on('error', errorHandler);

        try {
            this._raw = _net.Connect(this, port, host, process.dispatchEvent);
        } catch (err) {
            this.connecting = false;
            this.emit('error', err);
            throw err;
        }

        return this;
    }

    write(data, encoding, callback) {
        if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (!this._raw) {
            const err = new Error('Socket is not connected');
            if (callback) {
                callback(err);
            } else {
                throw err;
            }
            return false;
        }

        if (this.destroyed || !this.writable) {
            const err = new Error('Socket is not writable');
            if (callback) {
                callback(err);
            }
            return false;
        }

        try {
            let bytes;
            
            // Convert data to byte array
            if (typeof data === 'string') {
                bytes = stringToBytes(data, encoding || 'utf8');
            } else if (Buffer.isBuffer(data)) {
                bytes = Array.from(data);
            } else if (Array.isArray(data)) {
                bytes = data;
            } else {
                bytes = Array.from(data); // Try to convert to array
            }

            const n = this._raw.write(bytes);
            this.bytesWritten += n;

            if (callback) {
                process.nextTick(callback);
            }

            return true;
        } catch (err) {
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return false;
        }
    }

    end(data, encoding, callback) {
        if (typeof data === 'function') {
            callback = data;
            data = null;
        } else if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (callback) {
            this.once('finish', callback);
        }

        this.writable = false;

        if (!this._raw) {
            this.emit('finish');
            return this;
        }

        try {
            if (data) {
                let buffer;
                if (data && typeof data === 'object' && (data.constructor.name === 'Uint8Array' || data.constructor.name === 'Array')) {
                    buffer = Array.isArray(data) ? data : Array.from(data);
                } else if (typeof data === 'string') {
                    buffer = stringToBytes(data, encoding || 'utf8');
                } else {
                    buffer = stringToBytes(String(data), encoding || 'utf8');
                }
                this._raw.end(buffer);
            } else {
                this._raw.end(null);
            }
            this.emit('finish');
        } catch (err) {
            this.emit('error', err);
        }

        return this;
    }

    destroy(error) {
        if (this.destroyed) {
            return this;
        }

        this.destroyed = true;
        this.readable = false;
        this.writable = false;

        if (this._raw) {
            try {
                this._raw.destroy();
            } catch (err) {
                // Ignore
            }
        }

        if (error) {
            this.emit('error', error);
        }

        this.emit('close', true);
        return this;
    }

    setTimeout(timeout, callback) {
        if (callback) {
            this.once('timeout', callback);
        }

        if (this._raw) {
            try {
                this._raw.setTimeout(timeout);
            } catch (err) {
                this.emit('error', err);
            }
        }

        return this;
    }

    setNoDelay(noDelay) {
        if (this._raw) {
            try {
                this._raw.setNoDelay(noDelay === undefined ? true : noDelay);
            } catch (err) {
                this.emit('error', err);
            }
        }
        return this;
    }

    setKeepAlive(enable, initialDelay) {
        if (this._raw) {
            try {
                this._raw.setKeepAlive(
                    enable === undefined ? false : enable,
                    initialDelay || 0
                );
            } catch (err) {
                this.emit('error', err);
            }
        }
        return this;
    }

    setEncoding(encoding) {
        this._encoding = encoding || 'utf8';
        return this;
    }

    address() {
        if (!this._raw) {
            return {};
        }

        try {
            return this._raw.address();
        } catch (err) {
            return {};
        }
    }

    pause() {
        this.readable = false;
        if (this._raw) {
            try {
                this._raw.pause();
            } catch (err) {
                // Ignore
            }
        }
        return this;
    }

    resume() {
        this.readable = true;
        if (this._raw) {
            try {
                this._raw.resume();
            } catch (err) {
                // Ignore
            }
        }
        return this;
    }

    ref() {
        return this;
    }

    unref() {
        return this;
    }
}

// Factory functions
function createServer(options, connectionListener) {
    return new Server(options, connectionListener);
}

function createConnection(...args) {
    const socket = new Socket();
    return socket.connect(...args);
}

function connect(...args) {
    return createConnection(...args);
}

// Check if an IP address is valid
function isIP(input) {
    if (typeof input !== 'string') {
        return 0;
    }

    // IPv4 pattern
    const ipv4Pattern = /^(\d{1,3}\.){3}\d{1,3}$/;
    if (ipv4Pattern.test(input)) {
        const parts = input.split('.');
        for (const part of parts) {
            const num = parseInt(part, 10);
            if (num < 0 || num > 255) {
                return 0;
            }
        }
        return 4;
    }

    // IPv6 pattern (simplified)
    const ipv6Pattern = /^([\da-fA-F]{1,4}:){7}[\da-fA-F]{1,4}$/;
    if (ipv6Pattern.test(input)) {
        return 6;
    }

    // IPv6 with :: shorthand (simplified check)
    if (input.includes('::')) {
        return 6;
    }

    return 0;
}

function isIPv4(input) {
    return isIP(input) === 4;
}

function isIPv6(input) {
    return isIP(input) === 6;
}

// Exports
module.exports = {
    Server,
    Socket,
    createServer,
    createConnection,
    connect,
    isIP,
    isIPv4,
    isIPv6
};
