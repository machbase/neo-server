'use strict';

const EventEmitter = require('events');
const process = require('process');
const _stream = require('@jsh/stream');

// Readable Stream - wraps Go io.Reader
class Readable extends EventEmitter {
    constructor(reader) {
        super();
        if (!reader) {
            throw new TypeError('Readable stream requires a reader');
        }
        this.raw = _stream.NewReadable(this, reader, process.dispatchEvent);
        this.readable = true;
        this.readableEnded = false;
        this.readableFlowing = null; // null, true, or false
        this.readableHighWaterMark = 16384; // 16KB default
        this.readableLength = 0;
        this._paused = false;
    }

    read(size) {
        try {
            const data = this.raw.read(size || 0);
            if (data && data.length > 0) {
                return Buffer.from(data);
            }
            return null;
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return null;
            }
            this.emit('error', err);
            return null;
        }
    }

    readString(size, encoding = 'utf-8') {
        try {
            return this.raw.readString(size || 0, encoding);
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return '';
            }
            this.emit('error', err);
            return '';
        }
    }

    pause() {
        if (this._paused) return this;
        this._paused = true;
        this.readableFlowing = false;
        this.raw.pause();
        return this;
    }

    resume() {
        if (!this._paused) return this;
        this._paused = false;
        this.readableFlowing = true;
        this.raw.resume();
        return this;
    }

    isPaused() {
        return this._paused;
    }

    pipe(destination, options) {
        if (!destination || typeof destination.write !== 'function') {
            throw new TypeError('Destination must be a writable stream');
        }

        const end = options && options.end !== undefined ? options.end : true;

        const onData = (chunk) => {
            const canContinue = destination.write(chunk);
            if (!canContinue) {
                this.pause();
            }
        };

        const onDrain = () => {
            if (this.readable) {
                this.resume();
            }
        };

        const onEnd = () => {
            if (end) {
                destination.end();
            }
        };

        const onError = (err) => {
            destination.emit('error', err);
        };

        this.on('data', onData);
        this.on('end', onEnd);
        this.on('error', onError);
        destination.on('drain', onDrain);

        // Cleanup on finish
        destination.once('close', () => {
            this.removeListener('data', onData);
            this.removeListener('end', onEnd);
            this.removeListener('error', onError);
            destination.removeListener('drain', onDrain);
        });

        this.resume();
        return destination;
    }

    unpipe(destination) {
        // Remove all piping if no destination specified
        this.removeAllListeners('data');
        return this;
    }

    destroy(error) {
        if (this.destroyed) return this;
        this.destroyed = true;
        this.readable = false;

        if (error) {
            this.emit('error', error);
        }

        this.raw.close();
        this.emit('close');
        return this;
    }

    close() {
        return this.destroy();
    }
}

// Writable Stream - wraps Go io.Writer
class Writable extends EventEmitter {
    constructor(writer) {
        super();
        if (!writer) {
            throw new TypeError('Writable stream requires a writer');
        }
        this.raw = _stream.NewWritable(this, writer, process.dispatchEvent);
        this.writable = true;
        this.writableEnded = false;
        this.writableFinished = false;
        this.writableHighWaterMark = 16384; // 16KB default
        this.writableLength = 0;
    }

    write(chunk, encoding, callback) {
        if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (!this.writable) {
            const err = new Error('Stream is not writable');
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return false;
        }

        try {
            let data;
            if (typeof chunk === 'string') {
                data = Buffer.from(chunk, encoding || 'utf8');
            } else if (Buffer.isBuffer(chunk)) {
                data = chunk;
            } else if (chunk instanceof Uint8Array) {
                data = Buffer.from(chunk);
            } else {
                throw new TypeError('Invalid chunk type: ' + typeof chunk);
            }

            this.raw.write(Array.from(data));

            if (callback) {
                setImmediate(callback);
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

    end(chunk, encoding, callback) {
        if (typeof chunk === 'function') {
            callback = chunk;
            chunk = null;
            encoding = null;
        } else if (typeof encoding === 'function') {
            callback = encoding;
            encoding = null;
        }

        if (this.writableEnded) {
            if (callback) {
                callback(new Error('Stream already ended'));
            }
            return this;
        }

        this.writableEnded = true;

        try {
            if (chunk) {
                let data;
                if (typeof chunk === 'string') {
                    data = Buffer.from(chunk, encoding || 'utf8');
                } else if (Buffer.isBuffer(chunk)) {
                    data = chunk;
                } else if (chunk instanceof Uint8Array) {
                    data = Buffer.from(chunk);
                } else {
                    throw new TypeError('Invalid chunk type');
                }
                this.raw.end(Array.from(data));
            } else {
                this.raw.end([]);
            }

            this.writableFinished = true;
            this.writable = false;

            if (callback) {
                setImmediate(callback);
            }

            return this;
        } catch (err) {
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return this;
        }
    }

    destroy(error) {
        if (this.destroyed) return this;
        this.destroyed = true;
        this.writable = false;

        if (error) {
            this.emit('error', error);
        }

        this.raw.close();
        this.emit('close');
        return this;
    }

    close() {
        return this.destroy();
    }
}

// Duplex Stream - combines readable and writable
class Duplex extends EventEmitter {
    constructor(reader, writer) {
        super();
        if (!reader || !writer) {
            throw new TypeError('Duplex stream requires both reader and writer');
        }
        this.raw = _stream.NewDuplex(this, reader, writer, process.dispatchEvent);

        // Readable properties
        this.readable = true;
        this.readableEnded = false;
        this.readableFlowing = null;
        this.readableHighWaterMark = 16384;
        this.readableLength = 0;

        // Writable properties
        this.writable = true;
        this.writableEnded = false;
        this.writableFinished = false;
        this.writableHighWaterMark = 16384;
        this.writableLength = 0;

        this._paused = false;
    }

    // Readable methods
    read(size) {
        try {
            const data = this.raw.read(size || 0);
            if (data && data.length > 0) {
                return Buffer.from(data);
            }
            return null;
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return null;
            }
            this.emit('error', err);
            return null;
        }
    }

    readString(size, encoding = 'utf-8') {
        try {
            return this.raw.readString(size || 0, encoding);
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return '';
            }
            this.emit('error', err);
            return '';
        }
    }

    pause() {
        if (this._paused) return this;
        this._paused = true;
        this.readableFlowing = false;
        this.raw.pause();
        return this;
    }

    resume() {
        if (!this._paused) return this;
        this._paused = false;
        this.readableFlowing = true;
        this.raw.resume();
        return this;
    }

    isPaused() {
        return this._paused;
    }

    // Writable methods
    write(chunk, encoding, callback) {
        if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (!this.writable) {
            const err = new Error('Stream is not writable');
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return false;
        }

        try {
            let data;
            if (typeof chunk === 'string') {
                data = Buffer.from(chunk, encoding || 'utf8');
            } else if (Buffer.isBuffer(chunk)) {
                data = chunk;
            } else if (chunk instanceof Uint8Array) {
                data = Buffer.from(chunk);
            } else {
                throw new TypeError('Invalid chunk type: ' + typeof chunk);
            }

            this.raw.write(Array.from(data));

            if (callback) {
                setImmediate(callback);
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

    end(chunk, encoding, callback) {
        if (typeof chunk === 'function') {
            callback = chunk;
            chunk = null;
            encoding = null;
        } else if (typeof encoding === 'function') {
            callback = encoding;
            encoding = null;
        }

        if (this.writableEnded) {
            if (callback) {
                callback(new Error('Stream already ended'));
            }
            return this;
        }

        this.writableEnded = true;

        try {
            if (chunk) {
                let data;
                if (typeof chunk === 'string') {
                    data = Buffer.from(chunk, encoding || 'utf8');
                } else if (Buffer.isBuffer(chunk)) {
                    data = chunk;
                } else if (chunk instanceof Uint8Array) {
                    data = Buffer.from(chunk);
                } else {
                    throw new TypeError('Invalid chunk type');
                }
                this.raw.end(Array.from(data));
            } else {
                this.raw.end([]);
            }

            this.writableFinished = true;
            this.writable = false;

            if (callback) {
                setImmediate(callback);
            }

            return this;
        } catch (err) {
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return this;
        }
    }

    pipe(destination, options) {
        if (!destination || typeof destination.write !== 'function') {
            throw new TypeError('Destination must be a writable stream');
        }

        const end = options && options.end !== undefined ? options.end : true;

        const onData = (chunk) => {
            const canContinue = destination.write(chunk);
            if (!canContinue) {
                this.pause();
            }
        };

        const onDrain = () => {
            if (this.readable) {
                this.resume();
            }
        };

        const onEnd = () => {
            if (end) {
                destination.end();
            }
        };

        const onError = (err) => {
            destination.emit('error', err);
        };

        this.on('data', onData);
        this.on('end', onEnd);
        this.on('error', onError);
        destination.on('drain', onDrain);

        destination.once('close', () => {
            this.removeListener('data', onData);
            this.removeListener('end', onEnd);
            this.removeListener('error', onError);
            destination.removeListener('drain', onDrain);
        });

        this.resume();
        return destination;
    }

    destroy(error) {
        if (this.destroyed) return this;
        this.destroyed = true;
        this.readable = false;
        this.writable = false;

        if (error) {
            this.emit('error', error);
        }

        this.raw.close();
        this.emit('close');
        return this;
    }

    close() {
        return this.destroy();
    }
}

// PassThrough Stream - a simple transform stream
class PassThrough extends EventEmitter {
    constructor() {
        super();
        this.raw = _stream.NewPassThrough(this, process.dispatchEvent);

        // Readable properties
        this.readable = true;
        this.readableEnded = false;
        this.readableFlowing = null;
        this.readableHighWaterMark = 16384;
        this.readableLength = 0;

        // Writable properties
        this.writable = true;
        this.writableEnded = false;
        this.writableFinished = false;
        this.writableHighWaterMark = 16384;
        this.writableLength = 0;

        this._paused = false;
    }

    // Readable methods
    read(size) {
        try {
            const data = this.raw.read(size || 0);
            if (data && data.length > 0) {
                return Buffer.from(data);
            }
            return null;
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return null;
            }
            this.emit('error', err);
            return null;
        }
    }

    readString(size, encoding = 'utf-8') {
        try {
            return this.raw.readString(size || 0, encoding);
        } catch (err) {
            if (err.message && err.message.includes('EOF')) {
                this.readableEnded = true;
                this.readable = false;
                return '';
            }
            this.emit('error', err);
            return '';
        }
    }

    pause() {
        if (this._paused) return this;
        this._paused = true;
        this.readableFlowing = false;
        this.raw.pause();
        return this;
    }

    resume() {
        if (!this._paused) return this;
        this._paused = false;
        this.readableFlowing = true;
        this.raw.resume();
        return this;
    }

    isPaused() {
        return this._paused;
    }

    // Writable methods
    write(chunk, encoding, callback) {
        if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (!this.writable) {
            const err = new Error('Stream is not writable');
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return false;
        }

        try {
            let data;
            if (typeof chunk === 'string') {
                data = Buffer.from(chunk, encoding || 'utf8');
            } else if (Buffer.isBuffer(chunk)) {
                data = chunk;
            } else if (chunk instanceof Uint8Array) {
                data = Buffer.from(chunk);
            } else {
                throw new TypeError('Invalid chunk type: ' + typeof chunk);
            }

            this.raw.write(Array.from(data));

            if (callback) {
                setImmediate(callback);
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

    end(chunk, encoding, callback) {
        if (typeof chunk === 'function') {
            callback = chunk;
            chunk = null;
            encoding = null;
        } else if (typeof encoding === 'function') {
            callback = encoding;
            encoding = null;
        }

        if (this.writableEnded) {
            if (callback) {
                callback(new Error('Stream already ended'));
            }
            return this;
        }

        this.writableEnded = true;

        try {
            if (chunk) {
                let data;
                if (typeof chunk === 'string') {
                    data = Buffer.from(chunk, encoding || 'utf8');
                } else if (Buffer.isBuffer(chunk)) {
                    data = chunk;
                } else if (chunk instanceof Uint8Array) {
                    data = Buffer.from(chunk);
                } else {
                    throw new TypeError('Invalid chunk type');
                }
                this.raw.end(Array.from(data));
            } else {
                this.raw.end([]);
            }

            this.writableFinished = true;
            this.writable = false;

            if (callback) {
                setImmediate(callback);
            }

            return this;
        } catch (err) {
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return this;
        }
    }

    pipe(destination, options) {
        if (!destination || typeof destination.write !== 'function') {
            throw new TypeError('Destination must be a writable stream');
        }

        const end = options && options.end !== undefined ? options.end : true;

        const onData = (chunk) => {
            const canContinue = destination.write(chunk);
            if (!canContinue) {
                this.pause();
            }
        };

        const onDrain = () => {
            if (this.readable) {
                this.resume();
            }
        };

        const onEnd = () => {
            if (end) {
                destination.end();
            }
        };

        const onError = (err) => {
            destination.emit('error', err);
        };

        this.on('data', onData);
        this.on('end', onEnd);
        this.on('error', onError);
        destination.on('drain', onDrain);

        destination.once('close', () => {
            this.removeListener('data', onData);
            this.removeListener('end', onEnd);
            this.removeListener('error', onError);
            destination.removeListener('drain', onDrain);
        });

        this.resume();
        return destination;
    }

    destroy(error) {
        if (this.destroyed) return this;
        this.destroyed = true;
        this.readable = false;
        this.writable = false;

        if (error) {
            this.emit('error', error);
        }

        this.raw.close();
        this.emit('close');
        return this;
    }

    close() {
        return this.destroy();
    }
}

// Transform Stream - for transforming data
class Transform extends EventEmitter {
    constructor(options) {
        super();
        options = options || {};

        // Readable properties
        this.readable = true;
        this.readableEnded = false;
        this.readableFlowing = null;
        this.readableHighWaterMark = 16384;
        this.readableLength = 0;

        // Writable properties
        this.writable = true;
        this.writableEnded = false;
        this.writableFinished = false;
        this.writableHighWaterMark = 16384;
        this.writableLength = 0;

        // Internal state
        this._buffer = [];
        this._transforming = false;
    }

    _transform(chunk, encoding, callback) {
        // Default implementation: pass through
        callback(null, chunk);
    }

    _flush(callback) {
        // Default implementation: do nothing
        callback(null);
    }

    write(chunk, encoding, callback) {
        if (typeof encoding === 'function') {
            callback = encoding;
            encoding = 'utf8';
        }

        if (!this.writable) {
            const err = new Error('Stream is not writable');
            if (callback) {
                callback(err);
            } else {
                this.emit('error', err);
            }
            return false;
        }

        try {
            let data;
            if (typeof chunk === 'string') {
                data = Buffer.from(chunk, encoding || 'utf8');
            } else if (Buffer.isBuffer(chunk)) {
                data = chunk;
            } else if (chunk instanceof Uint8Array) {
                data = Buffer.from(chunk);
            } else {
                throw new TypeError('Invalid chunk type: ' + typeof chunk);
            }

            this._transform(data, encoding, (err, transformed) => {
                if (err) {
                    if (callback) {
                        callback(err);
                    } else {
                        this.emit('error', err);
                    }
                    return;
                }

                if (callback) {
                    callback();
                }
            });

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

    push(chunk) {
        if (chunk === null) {
            this.readableEnded = true;
            this.readable = false;
            this.emit('end');
            return false;
        }

        let data;
        if (typeof chunk === 'string') {
            data = Buffer.from(chunk, 'utf8');
        } else if (Buffer.isBuffer(chunk)) {
            data = chunk;
        } else if (chunk instanceof Uint8Array) {
            data = Buffer.from(chunk);
        } else {
            data = Buffer.from(String(chunk));
        }

        this._buffer.push(data);
        this.emit('data', data);
        return true;
    }

    end(chunk, encoding, callback) {
        if (typeof chunk === 'function') {
            callback = chunk;
            chunk = null;
            encoding = null;
        } else if (typeof encoding === 'function') {
            callback = encoding;
            encoding = null;
        }

        if (this.writableEnded) {
            if (callback) {
                callback(new Error('Stream already ended'));
            }
            return this;
        }

        const finishEnd = () => {
            this._flush((err, data) => {
                if (err) {
                    if (callback) {
                        callback(err);
                    } else {
                        this.emit('error', err);
                    }
                    return;
                }

                if (data) {
                    this.push(data);
                }

                this.writableEnded = true;
                this.writableFinished = true;
                this.writable = false;

                this.emit('finish');

                // Push null to signal end of readable side
                this.push(null);

                if (callback) {
                    callback();
                }
            });
        };

        if (chunk) {
            this.write(chunk, encoding, (err) => {
                if (err) {
                    if (callback) {
                        callback(err);
                    } else {
                        this.emit('error', err);
                    }
                    return;
                }
                finishEnd();
            });
        } else {
            finishEnd();
        }

        return this;
    }

    pipe(destination, options) {
        if (!destination || typeof destination.write !== 'function') {
            throw new TypeError('Destination must be a writable stream');
        }

        const end = options && options.end !== undefined ? options.end : true;

        const onData = (chunk) => {
            const canContinue = destination.write(chunk);
            if (!canContinue && this.pause) {
                this.pause();
            }
        };

        const onDrain = () => {
            if (this.readable && this.resume) {
                this.resume();
            }
        };

        const onEnd = () => {
            if (end) {
                destination.end();
            }
        };

        const onError = (err) => {
            destination.emit('error', err);
        };

        this.on('data', onData);
        this.on('end', onEnd);
        this.on('error', onError);
        if (destination.on) {
            destination.on('drain', onDrain);
        }

        // Cleanup on finish
        const cleanup = () => {
            this.removeListener('data', onData);
            this.removeListener('end', onEnd);
            this.removeListener('error', onError);
            if (destination.removeListener) {
                destination.removeListener('drain', onDrain);
            }
        };

        if (destination.once) {
            destination.once('close', cleanup);
        }
        this.once('end', cleanup);

        return destination;
    }

    destroy(error) {
        if (this.destroyed) return this;
        this.destroyed = true;
        this.readable = false;
        this.writable = false;

        if (error) {
            this.emit('error', error);
        }

        this.emit('close');
        return this;
    }

    pause() {
        this.readableFlowing = false;
        return this;
    }

    resume() {
        this.readableFlowing = true;
        return this;
    }
}

// Helper to create a Buffer-like object from Uint8Array
if (typeof Buffer === 'undefined') {
    global.Buffer = class Buffer extends Uint8Array {
        static from(data, encoding) {
            if (typeof data === 'string') {
                const encoder = new TextEncoder();
                return new Buffer(encoder.encode(data));
            } else if (data instanceof Uint8Array) {
                return new Buffer(data);
            } else if (Array.isArray(data)) {
                return new Buffer(new Uint8Array(data));
            }
            throw new TypeError('Unsupported data type for Buffer.from');
        }

        static isBuffer(obj) {
            return obj instanceof Buffer || obj instanceof Uint8Array;
        }

        toString(encoding = 'utf8') {
            const decoder = new TextDecoder(encoding);
            return decoder.decode(this);
        }
    };
} else if (!Buffer.isBuffer) {
    // Buffer exists but isBuffer method is missing
    Buffer.isBuffer = function (obj) {
        return obj instanceof Buffer || obj instanceof Uint8Array;
    };
}

module.exports = {
    Readable,
    Writable,
    Duplex,
    PassThrough,
    Transform,
};
