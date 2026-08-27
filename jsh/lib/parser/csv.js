'use strict';

const { Transform } = require('stream');
const _parser = require('@jsh/parser');

/**
 * CSV Parser
 * Parses CSV data and emits row objects
 */
class CSVParser extends Transform {
    constructor(options) {
        options = options || {};
        super(options);

        // CSV parsing options
        this.separator = options.separator || ',';
        this.quote = options.quote || '"';
        this.escape = options.escape || this.quote;
        this.headers = options.headers; // true, false, or array of header names
        this.skipLines = options.skipLines || 0;
        this.skipComments = options.skipComments || false;
        this.commentChar = typeof options.skipComments === 'string' ? options.skipComments : '#';
        this.strict = options.strict || false;
        this.mapHeaders = options.mapHeaders || null;
        this.mapValues = options.mapValues || null;
        this.trimLeadingSpace = options.trimLeadingSpace !== false;
        // 'object' emits {header: value}, 'array' emits the raw field list (much cheaper).
        this.rowMode = options.rowMode === 'array' ? 'array' : 'object';
        this.valueTypes = Array.isArray(options.valueTypes) ? options.valueTypes : null;
        this.timeformat = options.timeformat || '';
        this.tz = options.tz || 'local';
        this.nullValue = options.nullValue || '';
        this.convertAfterRows = options.convertAfterRows || 0;

        // Internal state
        this.lineNumber = 0;
        this.headersParsed = false;
        this.columnHeaders = null;
        this.bytesWritten = 0;
        this.bytesRead = 0;
        this._decoder = null;
        this._convertCurrentBatch = false;
    }

    // The decoder is created lazily so that options assigned after construction are honored.
    _ensureDecoder() {
        if (!this._decoder) {
            this._decoder = _parser.NewCSVDecoder({
                separator: this.separator,
                quote: this.quote,
                escape: this.escape,
                commentChar: this.commentChar,
                skipComments: !!this.skipComments,
                trimLeadingSpace: this.trimLeadingSpace,
                skipLines: this.skipLines,
                valueTypes: this.valueTypes,
                timeformat: this.timeformat,
                tz: this.tz,
                nullValue: this.nullValue,
                convertAfterRows: this.convertAfterRows,
            });
        }
        return this._decoder;
    }

    setValueTypes(valueTypes, options) {
        options = options || {};
        this.valueTypes = valueTypes;
        if (Object.prototype.hasOwnProperty.call(options, 'timeformat')) {
            this.timeformat = options.timeformat;
        }
        if (Object.prototype.hasOwnProperty.call(options, 'tz')) {
            this.tz = options.tz;
        }
        if (Object.prototype.hasOwnProperty.call(options, 'nullValue')) {
            this.nullValue = options.nullValue;
        }
        if (Object.prototype.hasOwnProperty.call(options, 'convertAfterRows')) {
            this.convertAfterRows = options.convertAfterRows;
        }
        if (this._decoder) {
            this._decoder.configureValues({
                valueTypes: this.valueTypes,
                timeformat: this.timeformat,
                tz: this.tz,
                nullValue: this.nullValue,
                convertAfterRows: this.convertAfterRows,
            });
            this._convertCurrentBatch = true;
        }
    }

    _transform(chunk, encoding, callback) {
        try {
            const decoder = this._ensureDecoder();
            const err = this._consume(decoder, decoder.write(chunk));
            if (err) {
                return callback(err);
            }
            callback();
        } catch (err) {
            callback(err);
        }
    }

    _flush(callback) {
        try {
            const decoder = this._ensureDecoder();
            const err = this._consume(decoder, decoder.flush());
            if (err) {
                return callback(err);
            }
            callback();
        } catch (err) {
            callback(err);
        }
    }

    // Returns the sole 'data' listener when rows can be delivered without any
    // per-row transformation, so the emit() dispatch can be bypassed.
    _fastListener() {
        if (this.rowMode !== 'array' || this.mapValues || this.strict) {
            return null;
        }
        const handlers = this._events && this._events['data'];
        return handlers && handlers.length === 1 ? handlers[0] : null;
    }

    _consume(decoder, records) {
        this.bytesWritten = decoder.bytesWritten();
        this.bytesRead = decoder.bytesRead();
        if (!records) {
            this.lineNumber = decoder.lineNumber();
            return null;
        }
        const count = records.length;
        let i = 0;
        while (i < count && !this.headersParsed) {
            const err = this.processRecord(records[i]);
            if (err) {
                return err;
            }
            i++;
        }
        const fast = this._fastListener();
        if (fast) {
            for (; i < count; i++) {
                try {
                    fast.call(this, this._convertCurrentBatch ? decoder.convertRecord(records[i]) : records[i]);
                } catch (err) {
                    if (!this._events['error']) {
                        throw err;
                    }
                    this.emit('error', err);
                }
            }
            this._convertCurrentBatch = false;
            this.lineNumber = decoder.lineNumber();
            return null;
        }
        const recordLines = this.strict ? decoder.recordLines() : null;
        for (; i < count; i++) {
            if (recordLines) {
                this.lineNumber = recordLines[i];
            }
            const err = this.processRecord(records[i]);
            if (err) {
                return err;
            }
        }
        this.lineNumber = decoder.lineNumber();
        return null;
    }

    processRecord(fields) {
        // Handle headers
        if (!this.headersParsed) {
            if (Array.isArray(this.headers)) {
                // Use provided headers
                this.columnHeaders = this.headers;
                this.headersParsed = true;
                // This line is data, not headers
                return this.emitRow(fields);
            } else if (this.headers === false) {
                // No headers, use column indices
                this.columnHeaders = [];
                for (let i = 0; i < fields.length; i++) {
                    this.columnHeaders.push(String(i));
                }
                this.headersParsed = true;
                // This line is data, not headers
                return this.emitRow(fields);
            } else {
                // First line is headers (default behavior)
                this.columnHeaders = [];
                for (let i = 0; i < fields.length; i++) {
                    let header = fields[i];
                    if (this.mapHeaders) {
                        const mapped = this.mapHeaders({ header, index: i });
                        if (mapped === null || mapped === undefined) {
                            continue;
                        }
                        header = mapped;
                    }
                    this.columnHeaders.push(header);
                }

                this.headersParsed = true;
                this.emit('headers', this.columnHeaders);
                return null;
            }
        }

        // Emit data row
        return this.emitRow(fields);
    }

    emitRow(fields) {
        // Check strict mode
        if (this.strict && fields.length !== this.columnHeaders.length) {
            const err = new Error(
                `Column count mismatch at line ${this.lineNumber}: ` +
                `expected ${this.columnHeaders.length}, got ${fields.length}`
            );
            this.emit('error', err);
            return err;
        }

        if (this.rowMode === 'array' && !this.mapValues) {
            this.emit('data', fields);
            return null;
        }

        // Build row object
        const row = {};
        for (let i = 0; i < this.columnHeaders.length; i++) {
            const header = this.columnHeaders[i];
            let value = i < fields.length ? fields[i] : '';

            // Apply value mapping if provided
            if (this.mapValues) {
                value = this.mapValues({ header, index: i, value });
            }

            row[header] = value;
        }

        // Handle extra columns in non-strict mode
        if (!this.strict && fields.length > this.columnHeaders.length) {
            for (let i = this.columnHeaders.length; i < fields.length; i++) {
                row[`_${i}`] = fields[i];
            }
        }

        // Emit the row object as 'data' event only
        this.emit('data', row);

        return null;
    }
}

module.exports = CSVParser;
