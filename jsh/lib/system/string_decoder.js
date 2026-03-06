// string_decoder.js - Node.js compatible string decoder implementation
// Provides an API for decoding Buffer objects into strings while preserving
// encoded multi-byte UTF-8 and UTF-16 characters

'use strict';

// Supported encodings
const ENCODINGS = {
  'utf8': true,
  'utf-8': true,
  'utf16le': true,
  'utf-16le': true,
  'ucs2': true,
  'ucs-2': true,
  'base64': true,
  'base64url': true,
  'latin1': true,
  'binary': true,
  'hex': true,
  'ascii': true
};

// Normalize encoding name
function normalizeEncoding(enc) {
  if (!enc) return 'utf8';
  const encoding = String(enc).toLowerCase();
  
  // Map aliases
  switch (encoding) {
    case 'utf8':
    case 'utf-8':
      return 'utf8';
    case 'utf16le':
    case 'utf-16le':
    case 'ucs2':
    case 'ucs-2':
      return 'utf16le';
    case 'latin1':
    case 'binary':
      return 'latin1';
    case 'base64':
    case 'base64url':
    case 'hex':
    case 'ascii':
      return encoding;
    default:
      // Node.js throws for unsupported encodings
      if (ENCODINGS[encoding]) return encoding;
      throw new TypeError('Unknown encoding: ' + enc);
  }
}

class StringDecoder {
  constructor(encoding) {
    this.encoding = normalizeEncoding(encoding);
    this.lastNeed = 0;  // Number of bytes needed to complete character
    this.lastTotal = 0; // Total bytes needed for the character
    this.lastChar = Buffer.allocUnsafe(6); // Buffer to store incomplete characters
  }

  write(buf) {
    if (!Buffer.isBuffer(buf)) {
      throw new TypeError('Argument must be a Buffer');
    }

    if (buf.length === 0) return '';

    let r;
    let i;

    // If we have incomplete characters from last write, prepend them
    if (this.lastNeed) {
      r = this._fillLast(buf);
      if (r === undefined) return '';
      i = this.lastNeed;
      this.lastNeed = 0;
    } else {
      r = '';
      i = 0;
    }

    // Decode the main buffer
    if (i < buf.length) {
      return r + this._decode(buf, i);
    }

    return r;
  }

  end(buf) {
    let r = '';
    
    if (buf && buf.length) {
      r = this.write(buf);
    }

    // If there are incomplete bytes, return them as replacement characters
    if (this.lastNeed) {
      r += this._flushIncomplete();
    }

    return r;
  }

  _decode(buf, start) {
    switch (this.encoding) {
      case 'utf8':
        return this._utf8Decode(buf, start);
      case 'utf16le':
        return this._utf16Decode(buf, start);
      case 'base64':
        return this._base64Decode(buf, start);
      case 'base64url':
        return this._base64urlDecode(buf, start);
      case 'latin1':
        return this._latin1Decode(buf, start);
      case 'hex':
        return this._hexDecode(buf, start);
      case 'ascii':
        return this._asciiDecode(buf, start);
      default:
        return buf.toString(this.encoding, start);
    }
  }

  _utf8Decode(buf, start) {
    let end = buf.length;
    let str = '';

    // Find where to cut off to avoid incomplete characters
    while (start < end) {
      const firstByte = buf[start];
      let charBytes = 1;
      let missingBytes = 0;

      if (firstByte < 0x80) {
        // Single byte character (0xxxxxxx)
        charBytes = 1;
      } else if ((firstByte & 0xE0) === 0xC0) {
        // Two byte character (110xxxxx)
        charBytes = 2;
      } else if ((firstByte & 0xF0) === 0xE0) {
        // Three byte character (1110xxxx)
        charBytes = 3;
      } else if ((firstByte & 0xF8) === 0xF0) {
        // Four byte character (11110xxx)
        charBytes = 4;
      } else {
        // Invalid UTF-8 start byte, skip it
        start++;
        str += '\ufffd'; // Replacement character
        continue;
      }

      missingBytes = charBytes - (end - start);

      if (missingBytes > 0) {
        // Incomplete character at the end
        this.lastTotal = charBytes;
        this.lastNeed = missingBytes;
        buf.copy(this.lastChar, 0, start, end);
        return str + buf.toString('utf8', start, start);
      }

      // Complete character, move forward
      start += charBytes;
    }

    return str + buf.toString('utf8', start - (end - start), end);
  }

  _utf16Decode(buf, start) {
    const end = buf.length;
    
    // UTF-16 characters are 2 or 4 bytes
    // Check if we have incomplete character at the end
    const remaining = (end - start) % 2;
    
    if (remaining !== 0) {
      this.lastNeed = 2 - remaining;
      this.lastTotal = 2;
      buf.copy(this.lastChar, 0, end - remaining, end);
      return buf.toString('utf16le', start, end - remaining);
    }

    return buf.toString('utf16le', start, end);
  }

  _base64Decode(buf, start) {
    const remaining = (buf.length - start) % 3;
    
    if (remaining !== 0) {
      this.lastNeed = 3 - remaining;
      this.lastTotal = 3;
      buf.copy(this.lastChar, 0, buf.length - remaining, buf.length);
      return buf.toString('base64', start, buf.length - remaining);
    }

    return buf.toString('base64', start);
  }

  _base64urlDecode(buf, start) {
    // Similar to base64 but with URL-safe characters
    const str = this._base64Decode(buf, start);
    return str.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }

  _latin1Decode(buf, start) {
    return buf.toString('latin1', start);
  }

  _hexDecode(buf, start) {
    return buf.toString('hex', start);
  }

  _asciiDecode(buf, start) {
    return buf.toString('ascii', start);
  }

  _fillLast(buf) {
    if (this.lastNeed <= buf.length) {
      // We have enough bytes to complete the character
      buf.copy(this.lastChar, this.lastTotal - this.lastNeed, 0, this.lastNeed);
      return this.lastChar.toString(this.encoding, 0, this.lastTotal);
    }

    // Still not enough bytes
    buf.copy(this.lastChar, this.lastTotal - this.lastNeed, 0, buf.length);
    this.lastNeed -= buf.length;
    return undefined;
  }

  _flushIncomplete() {
    // Return replacement characters for incomplete bytes
    let str = '';
    
    if (this.encoding === 'utf8' || this.encoding === 'utf-8') {
      const bytesWritten = this.lastTotal - this.lastNeed;
      for (let i = 0; i < bytesWritten; i++) {
        str += '\ufffd'; // UTF-8 replacement character
      }
    } else {
      // For other encodings, try to decode what we have
      str = this.lastChar.toString(this.encoding, 0, this.lastTotal - this.lastNeed);
    }

    this.lastNeed = 0;
    this.lastTotal = 0;
    return str;
  }

  // Node.js 8+ compatibility
  text(buf, offset) {
    return this.write(buf);
  }
}

// Export for CommonJS
module.exports = {
  StringDecoder
};

// Also export the class directly as default
module.exports.StringDecoder = StringDecoder;
