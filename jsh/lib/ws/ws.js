'use strict';

const process = require('/lib/process');
const EventEmitter = require('/lib/events');
const _ws = require('@jsh/ws');

// events: "open", "close", "message", "error"
class WebSocket extends EventEmitter {
    constructor(url, protocols) {
        super();
        if (url === undefined || typeof url !== 'string') {
            throw new TypeError('URL must be a string, got ' + typeof url);
        }
        const normalizedProtocols = normalizeProtocols(protocols);
        this.raw = _ws.NewWebSocket(this, url, normalizedProtocols, process.dispatchEvent);
        this.readyState = WebSocket.CONNECTING;
        this.url = url;
        this.protocol = '';

        setImmediate(() => {
            try {
                const err = this.raw.connect();
                if (err instanceof Error) {
                    this.emit('error', err);
                } else {
                    this.protocol = typeof this.raw.protocol === 'function' ? this.raw.protocol() : '';
                    this.readyState = WebSocket.OPEN;
                    this.emit('open');
                }
            } catch (e) {
                this.emit('error', new Error(e.value || e.message || String(e)));
            }
        });
    }

    static _fromAccepted(raw, url = '') {
        const socket = Object.create(WebSocket.prototype);
        socket._events = {};
        socket._maxListeners = 10;
        socket.raw = raw;
        socket.readyState = WebSocket.OPEN;
        socket.url = url;
        socket.protocol = typeof socket.raw.protocol === 'function' ? socket.raw.protocol() : '';
        if (typeof socket.raw.onMessage !== 'function' || typeof socket.raw.onClose !== 'function' || typeof socket.raw.onError !== 'function') {
            throw new TypeError('accepted websocket native hooks are not available');
        }
        socket.raw.onMessage((event) => {
            socket.emit('message', event);
        });
        socket.raw.onError((error) => {
            socket.emit('error', error);
        });
        socket.raw.onClose((event) => {
            socket.readyState = WebSocket.CLOSED;
            socket.emit('close', event);
        });
        return socket;
    }

    send(data) {
        try {
            this.raw.send(WebSocket.TextMessage, data);
        } catch (err) {
            this.emit('error', err);
        }
    }
    close() {
        if (this.readyState === this.CLOSED) {
            return;
        }
        this.readyState = this.CLOSING;
        this.raw.close();
        this.readyState = this.CLOSED;
        this.emit('close');
    }

    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    static TextMessage = 1;
    static BinaryMessage = 2;
}

function normalizeProtocols(protocols) {
    if (protocols === undefined || protocols === null) {
        return [];
    }
    if (typeof protocols === 'string') {
        return [protocols];
    }
    if (Array.isArray(protocols)) {
        return protocols.filter((value) => typeof value === 'string');
    }
    throw new TypeError('Protocols must be a string or an array of strings');
}

function normalizeHeaders(headers) {
    const normalized = {};
    if (!headers || typeof headers !== 'object') {
        return normalized;
    }
    for (const key in headers) {
        normalized[String(key).toLowerCase()] = headers[key];
    }
    return normalized;
}

function normalizeRequest(rawReq) {
    if (!rawReq) {
        return rawReq;
    }
    const readProperty = (camelName, pascalName, fallback) => {
        if (rawReq[camelName] !== undefined) {
            return rawReq[camelName];
        }
        if (rawReq[pascalName] !== undefined) {
            return rawReq[pascalName];
        }
        return fallback;
    };
    const queryMethod = typeof rawReq.query === 'function'
        ? rawReq.query.bind(rawReq)
        : (typeof rawReq.Query === 'function' ? rawReq.Query.bind(rawReq) : null);
    const getHeaderMethod = typeof rawReq.getHeader === 'function'
        ? rawReq.getHeader.bind(rawReq)
        : (typeof rawReq.GetHeader === 'function' ? rawReq.GetHeader.bind(rawReq) : null);
    const hasHeaderMethod = typeof rawReq.hasHeader === 'function'
        ? rawReq.hasHeader.bind(rawReq)
        : (typeof rawReq.HasHeader === 'function' ? rawReq.HasHeader.bind(rawReq) : null);
    const headers = normalizeHeaders(readProperty('headers', 'Headers', {}));
    const rawUrl = readProperty('url', 'URL', '');
    const request = {
        url: rawUrl,
        method: readProperty('method', 'Method', 'GET'),
        headers,
        rawHeaders: readProperty('rawHeaders', 'RawHeaders', []),
        path: readProperty('path', 'Path', ''),
        host: readProperty('host', 'Host', headers.host || ''),
        requestUri: readProperty('requestURI', 'RequestURI', rawUrl),
        httpVersion: String(readProperty('proto', 'Proto', 'HTTP/1.1')).replace(/^HTTP\//, ''),
        complete: true,
        remoteAddress: readProperty('remoteAddress', 'RemoteAddress', ''),
        socket: {
            remoteAddress: readProperty('remoteAddress', 'RemoteAddress', ''),
        },
        getHeader(name) {
            if (getHeaderMethod) {
                return getHeaderMethod(name);
            }
            return headers[String(name).toLowerCase()];
        },
        hasHeader(name) {
            if (hasHeaderMethod) {
                return hasHeaderMethod(name);
            }
            return String(name).toLowerCase() in headers;
        },
        query(name) {
            return queryMethod ? queryMethod(name) : undefined;
        },
    };
    return {
        ...request,
    };
}

function createVerifyClient(options) {
    if (typeof options.verifyClient !== 'function') {
        return () => true;
    }
    return (rawReq) => {
        const req = normalizeRequest(rawReq);
        const result = options.verifyClient({
            origin: req.getHeader('origin'),
            req,
        });
        return result !== false;
    };
}

function createHandleProtocols(options) {
    if (typeof options.handleProtocols !== 'function') {
        return () => '';
    }
    return (protocols, rawReq) => {
        const req = normalizeRequest(rawReq);
        const selected = options.handleProtocols(protocols, req);
        return typeof selected === 'string' ? selected : '';
    };
}

class WebSocketServer extends EventEmitter {
    constructor(options = {}) {
        super();
        if (!options || typeof options !== 'object') {
            throw new TypeError('WebSocketServer options must be an object');
        }
        if (!options.server || typeof options.server._attachWebSocket !== 'function') {
            throw new TypeError('WebSocketServer requires an http.Server instance');
        }

        this.options = options;
        this.server = options.server;
        this.path = options.path || '/';
        this.clients = options.clientTracking === false ? null : new Set();
        this.closed = false;
        this._verifyClient = createVerifyClient(options);
        this._handleProtocols = createHandleProtocols(options);

        this.server._attachWebSocket(this.path, this._verifyClient, this._handleProtocols, (rawSocket, rawReq) => {
            try {
                if (this.closed) {
                    rawSocket.close();
                    return;
                }

                const request = normalizeRequest(rawReq);
                const socket = WebSocket._fromAccepted(rawSocket, request && request.url ? request.url : '');

                if (this.clients) {
                    this.clients.add(socket);
                    const cleanup = () => {
                        this.clients.delete(socket);
                        socket.removeListener('close', cleanup);
                    };
                    socket.on('close', cleanup);
                }

                this.emit('connection', socket, request);
                socket.emit('open');
            } catch (err) {
                this.emit('error', err);
            }
        });
    }

    close(callback) {
        this.closed = true;
        if (this.clients) {
            for (const client of this.clients) {
                client.close();
            }
            this.clients.clear();
        }
        this.emit('close');
        if (typeof callback === 'function') {
            callback();
        }
        return this;
    }
}

module.exports = {
    WebSocket: WebSocket,
    WebSocketServer: WebSocketServer,
}