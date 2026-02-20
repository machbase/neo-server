'use strict';

const CSVParser = require('./csv');
const NDJSONParser = require('./ndjson');

/**
 * Create a CSV parser stream
 * @param {Object} options - Parser options
 * @returns {CSVParser} CSV parser stream
 */
function csv(options) {
    return new CSVParser(options);
}

/**
 * Create an NDJSON parser stream
 * @param {Object} options - Parser options
 * @returns {NDJSONParser} NDJSON parser stream
 */
function ndjson(options) {
    return new NDJSONParser(options);
}

module.exports = {
    csv,
    ndjson,
    CSVParser,
    NDJSONParser,
};
