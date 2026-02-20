'use strict';

/**
 * Split a string into fields by whitespace, respecting quoted substrings.
 * Quoted parts (with " or ') are treated as single fields even if they contain spaces.
 * 
 * @param {string} str - The input string to split
 * @param {object} options - Optional configuration
 * @returns {string[]} Array of field strings
 * 
 * @example
 * splitFields('hello world') // ['hello', 'world']
 * splitFields('hello "world foo" bar') // ['hello', 'world foo', 'bar']
 * splitFields("hello 'world foo' bar") // ['hello', 'world foo', 'bar']
 * splitFields('a "b c" d "e f"') // ['a', 'b c', 'd', 'e f']
 */
function splitFields(str, options) {
    if (typeof str !== 'string') {
        throw new TypeError('Input must be a string');
    }

    const fields = [];
    let current = '';
    let inQuote = false;
    let quoteChar = null;

    for (let i = 0; i < str.length; i++) {
        const char = str[i];

        if (inQuote) {
            // Inside a quoted string
            if (char === quoteChar) {
                // End of quoted string
                inQuote = false;
                quoteChar = null;
            } else {
                current += char;
            }
        } else {
            // Outside quoted string
            if (char === '"' || char === "'") {
                // Start of quoted string
                inQuote = true;
                quoteChar = char;
            } else if (char === ' ' || char === '\t' || char === '\n' || char === '\r') {
                // Whitespace - push current field if not empty
                if (current.length > 0) {
                    fields.push(current);
                    current = '';
                }
            } else {
                current += char;
            }
        }
    }

    // Push last field if not empty
    if (current.length > 0) {
        fields.push(current);
    }

    return fields;
}

module.exports = splitFields;