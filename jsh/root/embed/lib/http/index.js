'use strict';

const _http = require('@jsh/http');
const EventEmitter = require('/lib/events');

class Agent {
    constructor(opts = {}) {
        this.options = opts;
        this.raw = _http.NewClient();
    }
    destroy() {
        this.raw.close();
    }
}

// http.request(options[, callback])
// http.request(url[, options][, callback])
//  - url: string | URL
//  - options: RequestOptions
//  - callback: function(response)
function request() {
    let options = {};
    let url = undefined;
    if (arguments.length == 0) {
        throw new TypeError("At least one argument is required");
    }
    if (typeof arguments[0] === "string") {
        url = new URL(arguments[0]);
        if (typeof arguments[1] === "object") {
            options = arguments[1] || {};
        }
    } else if (arguments[0] instanceof URL) {
        url = arguments[0];
        if (typeof arguments[1] === "object") {
            options = arguments[1] || {};
        }
    } else if (arguments[0] instanceof Object) {
        options = arguments[0] || {};
        if (typeof options.url === "string") {
            url = new URL(options.url);
        } else if (options.url instanceof URL) {
            url = options.url;
        }
    } else {
        throw new TypeError("The first argument must be of type string, URL, or object, but got " + typeof arguments[0]);
    }

    if (url) {
        options = {
            ...options,
            protocol: url.protocol,
            host: url.host,
            hostname: url.hostname.trimRight('/'),
            port: url.port,
            path: url.pathname + url.search,
        };
    }
    return new ClientRequest(options);
}

// IncomingMessage - represents HTTP response
class IncomingMessage extends EventEmitter {
    constructor(rawResponse) {
        super();
        this.raw = rawResponse;
        this.statusCode = rawResponse.statusCode || 0;
        this.statusMessage = rawResponse.statusMessage || '';
        this.headers = rawResponse.headers || {};
        this.rawHeaders = this._getRawHeaders(rawResponse.headers);
        this.httpVersion = '1.1';
        this.complete = true;
        this.ok = rawResponse.ok || false;
    }

    _getRawHeaders(headersObj) {
        const rawHeaders = [];
        if (!headersObj || typeof headersObj !== 'object') return rawHeaders;
        
        for (const key in headersObj) {
            const value = headersObj[key];
            if (Array.isArray(value)) {
                for (const v of value) {
                    rawHeaders.push(key, v);
                }
            } else {
                rawHeaders.push(key, value);
            }
        }
        return rawHeaders;
    }

    setTimeout(msecs, callback) {
        if (callback) {
            this.once('timeout', callback);
        }
        return this;
    }

    // Read response body as string
    readBody(encoding = 'utf-8') {
        if (this.raw && this.raw.string) {
            return this.raw.string();
        }
        return '';
    }

    // Read response body as buffer
    readBodyBuffer() {
        if (this.raw && this.raw.read) {
            return this.raw.read();
        }
        return new Uint8Array(0);
    }

    // Parse response body as JSON
    json() {
        if (this.raw && this.raw.json) {
            return this.raw.json();
        }
        const body = this.readBody('utf-8');
        return JSON.parse(body);
    }

    // Parse response body as text
    text(encoding = 'utf-8') {
        return this.readBody(encoding);
    }

    close() {
        if (this.raw && this.raw.close) {
            this.raw.close();
        }
    }
}

// events: "response", "error", "end"
class OutgoingMessage extends EventEmitter {
    constructor() {
        super();
    }
}

// events: "response", "error", "end"
class ClientRequest extends OutgoingMessage {
    constructor(options) {
        super();
        if (!options.method || typeof options.method !== "string" || options.method.length === 0) {
            options.method = "GET";
        }
        const url = new URL(
            (options.protocol || "http:") + "//" +
            (options.hostname || options.host || "localhost") +
            (options.port ? (":" + options.port) : "") +
            (options.path || "/"));
        const req = _http.NewRequest(options.method.toUpperCase(), url.toString());
        this.raw = req;
        this.options = options;
        this._headers = {};
        
        for (const key in options) {
            if (key === "headers") {
                for (const hkey in options.headers) {
                    this.setHeader(hkey, options.headers[hkey]);
                }
            } else if (key === "auth") {
                this.setHeader("Authorization", "Basic " + _http.base64Encode(options.auth));
            }
        }
    }

    setHeader(name, value) {
        if (typeof name !== 'string') {
            throw new TypeError('Header name must be a string');
        }
        if (value === undefined) {
            throw new TypeError('Header value cannot be undefined');
        }
        this._headers[name.toLowerCase()] = { name, value: String(value) };
        this.raw.header.set(name, String(value));
        return this;
    }

    getHeader(name) {
        if (typeof name !== 'string') {
            throw new TypeError('Header name must be a string');
        }
        const header = this._headers[name.toLowerCase()];
        return header ? header.value : undefined;
    }

    removeHeader(name) {
        if (typeof name !== 'string') {
            throw new TypeError('Header name must be a string');
        }
        delete this._headers[name.toLowerCase()];
        this.raw.header.del(name);
        return this;
    }

    hasHeader(name) {
        if (typeof name !== 'string') {
            throw new TypeError('Header name must be a string');
        }
        return name.toLowerCase() in this._headers;
    }

    getHeaders() {
        const headers = {};
        for (const key in this._headers) {
            headers[this._headers[key].name] = this._headers[key].value;
        }
        return headers;
    }

    getHeaderNames() {
        return Object.keys(this._headers).map(key => this._headers[key].name);
    }

    destroy(err) {
        this.raw = null;
        if (err) {
            this.emit('error', err);
        }
    }
    // request.write(chunk[, encoding][, callback])
    // chunk - string | Buffer | Uint8Array
    // encoding - string
    // callback - function
    // Returns: true | false
    write() {
        const chunk = arguments[0];
        const encoding = (typeof arguments[1] === 'string') ? arguments[1] : 'utf-8';
        const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;

        let err = null;
        if (typeof chunk === 'string') {
            err = this.raw.writeString(chunk, encoding);
        } else if (chunk instanceof Uint8Array) {
            err = this.raw.write(chunk);
        } else {
            this.emit('error', new TypeError("The chunk argument must be of type string, Buffer, or Uint8Array, but got " + typeof chunk));
            return false;
        }
        if (err instanceof Error) {
            this.emit('error', new Error("Failed to write chunk to request stream"));
            return false;
        }
        if (callback) {
            callback();
        }
        return true;
    }
    // request.end([data[, encoding]][, callback])
    // data - string | Buffer | Uint8Array
    // encoding - string
    // callback - function
    // Returns: this
    end() {
        const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;

        let argLen = callback ? arguments.length - 1 : arguments.length;
        const data = (argLen > 0) ? arguments[0] : undefined;
        const encoding = (argLen > 1) ? arguments[1] : undefined;

        if (data) {
            this.write(data, encoding);
        }
        if (callback) {
            this.on('response', (response) => {
                callback(response);
            });
        }
        setImmediate(() => {
            const agent = this.options.agent ? this.options.agent : new Agent();
            let rawRsp = undefined;
            let incomingMsg = undefined;
            try {
                rawRsp = agent.raw.do(this.raw);
                incomingMsg = new IncomingMessage(rawRsp);
                this.emit('response', incomingMsg);
            } catch (err) {
                this.emit('error', err);
            } finally {
                if (incomingMsg) {
                    incomingMsg.close();
                }
                this.emit("end")
            }
        })

        // const p = new Promise((resolve, reject) => {
        //     setImmediate(() => {
        //         const agent = this.options.agent ? this.options.agent : new Agent();
        //         try {
        //             const rsp = agent.raw.do(this.raw);
        //             this.emit('response', rsp);
        //             resolve(rsp);
        //         } catch (err) {
        //             this.emit('error', err);
        //             reject(err);
        //         }
        //     })
        // });
        return this;
    }
}

// http.get(options[, callback])
// http.get(url[, options][, callback])
//  - url: string | URL
//  - options: RequestOptions
//  - callback: function(response)
function get() {
    const callback = (typeof arguments[arguments.length - 1] === 'function') ? arguments[arguments.length - 1] : null;
    const args = callback ? Array.prototype.slice.call(arguments, 0, -1) : arguments;
    const req = request(...args);
    if (callback) {
        req.end(callback);
    } else {
        req.end();
    }
    return req;
}

module.exports = {
    Agent,
    ClientRequest,
    IncomingMessage,
    get,
    request,
}
