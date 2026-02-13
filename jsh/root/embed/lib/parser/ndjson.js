'use strict';

const { Transform } = require('stream');
const _parser = require('@jsh/parser');

/**
 * NDJSON (Newline Delimited JSON) Parser
 * Parses newline-delimited JSON data and emits objects
 */
class NDJSONParser extends Transform {
    constructor(options) {
        options = options || {};
        super(options);

        this.strict = options.strict !== false; // default: true
        this.bufferChunks = [];  // Array of string chunks for O(n) concatenation
        this.bufferLength = 0;   // Track total buffer length
        this.lineNumber = 0;
    }

    _transform(chunk, encoding, callback) {
        try {
            // Convert chunk to string
            const str = chunk.toString('utf-8');

            // Use array buffering to avoid O(n²) string concatenation
            this.bufferChunks.push(str);
            this.bufferLength += str.length;

            // Join all buffered chunks and process complete lines
            const buffer = this.bufferChunks.join('');
            const lines = buffer.split('\n');

            // Keep the last incomplete line in buffer as a single chunk
            const incompleteLine = lines.pop() || '';
            this.bufferChunks = incompleteLine ? [incompleteLine] : [];
            this.bufferLength = incompleteLine.length;

            // Parse each complete line
            for (const line of lines) {
                this.lineNumber++;

                // Skip empty lines
                const trimmed = line.trim();
                if (trimmed.length === 0) {
                    continue;
                }

                try {
                    const obj = JSON.parse(trimmed);
                    // Emit the parsed object as 'data' event only
                    this.emit('data', obj);
                } catch (err) {
                    if (this.strict) {
                        return callback(new Error(
                            `Invalid JSON at line ${this.lineNumber}: ${err.message}`
                        ));
                    }
                    // In non-strict mode, skip invalid lines
                    this.emit('warning', {
                        line: this.lineNumber,
                        data: trimmed,
                        error: err.message
                    });
                }
            }

            callback();
        } catch (err) {
            callback(err);
        }
    }

    _flush(callback) {
        try {
            // Join any remaining buffered chunks and process
            const remaining = this.bufferChunks.join('').trim();
            if (remaining.length > 0) {
                this.lineNumber++;

                try {
                    const obj = JSON.parse(remaining);
                    // Emit the parsed object as 'data' event only
                    this.emit('data', obj);
                } catch (err) {
                    if (this.strict) {
                        return callback(new Error(
                            `Invalid JSON at line ${this.lineNumber}: ${err.message}`
                        ));
                    }
                    this.emit('warning', {
                        line: this.lineNumber,
                        data: remaining,
                        error: err.message
                    });
                }
            }

            this.bufferChunks = [];
            this.bufferLength = 0;
            callback();
        } catch (err) {
            callback(err);
        }
    }
}

module.exports = NDJSONParser;
