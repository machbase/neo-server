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

        // Internal state
        this.bufferChunks = [];  // Array of string chunks for O(n) concatenation
        this.bufferLength = 0;   // Track total buffer length
        this.lineNumber = 0;
        this.headersParsed = false;
        this.columnHeaders = null;
        this.skippedLines = 0;
    }

    _transform(chunk, encoding, callback) {
        try {
            // Convert chunk to string
            const str = chunk.toString('utf-8');

            // Use array buffering to avoid O(n²) string concatenation
            this.bufferChunks.push(str);
            this.bufferLength += str.length;

            // Process complete lines
            this.processBuffer(callback);
        } catch (err) {
            callback(err);
        }
    }

    _flush(callback) {
        try {
            // Join any remaining buffered chunks and process
            const remaining = this.bufferChunks.join('');
            if (remaining.trim().length > 0) {
                this.processLine(remaining, callback);
            }
            this.bufferChunks = [];
            this.bufferLength = 0;
            callback();
        } catch (err) {
            callback(err);
        }
    }

    processBuffer(callback) {
        // Join all buffered chunks into a single string
        const buffer = this.bufferChunks.join('');
        const lines = buffer.split('\n');

        // Keep the last incomplete line in buffer as a single chunk
        const incompleteLine = lines.pop() || '';
        this.bufferChunks = incompleteLine ? [incompleteLine] : [];
        this.bufferLength = incompleteLine.length;

        // Process each complete line
        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            const err = this.processLine(line);
            if (err) {
                return callback(err);
            }
        }

        callback();
    }

    processLine(line) {
        this.lineNumber++;

        // Remove trailing \r (for \r\n line endings)
        line = line.replace(/\r$/, '');

        // Skip initial lines if configured
        if (this.skippedLines < this.skipLines) {
            this.skippedLines++;
            return null;
        }

        // Skip empty lines
        if (line.trim().length === 0) {
            return null;
        }

        // Skip comments
        if (this.skipComments && line.trim().startsWith(this.commentChar)) {
            return null;
        }

        // Parse the CSV line
        const fields = this.parseCSVLine(line);

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
                this.columnHeaders = fields.map((_, i) => String(i));
                this.headersParsed = true;
                // This line is data, not headers
                return this.emitRow(fields);
            } else {
                // First line is headers (default behavior)
                this.columnHeaders = fields;

                // Apply header mapping if provided
                if (this.mapHeaders) {
                    this.columnHeaders = this.columnHeaders.map((header, index) => {
                        const mapped = this.mapHeaders({ header, index });
                        return mapped !== null && mapped !== undefined ? mapped : header;
                    }).filter(h => h !== null && h !== undefined);
                }

                this.headersParsed = true;
                this.emit('headers', this.columnHeaders);
                return null;
            }
        }

        // Emit data row
        return this.emitRow(fields);
    }

    parseCSVLine(line) {
        const fields = [];
        let fieldChars = [];  // Use array building instead of string concatenation
        let inQuotes = false;
        let i = 0;

        while (i < line.length) {
            const char = line[i];
            const nextChar = i + 1 < line.length ? line[i + 1] : '';

            if (inQuotes) {
                if (char === this.escape && nextChar === this.quote) {
                    // Escaped quote
                    fieldChars.push(this.quote);
                    i += 2;
                } else if (char === this.quote) {
                    // End of quoted field
                    inQuotes = false;
                    i++;
                } else {
                    fieldChars.push(char);
                    i++;
                }
            } else {
                if (char === this.quote) {
                    // Start of quoted field
                    inQuotes = true;
                    i++;
                } else if (char === this.separator) {
                    // Field separator
                    const field = fieldChars.join('');
                    fields.push(this.trimLeadingSpace ? field.trimStart() : field);
                    fieldChars = [];
                    i++;
                } else {
                    fieldChars.push(char);
                    i++;
                }
            }
        }

        // Add the last field
        const field = fieldChars.join('');
        fields.push(this.trimLeadingSpace ? field.trimStart() : field);

        return fields;
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
