'use strict';

const process = require('/lib/process');
const EventEmitter = require('/lib/events');
const _ws = require('@jsh/ws');

// events: "open", "close", "message", "error"
class WebSocket extends EventEmitter {
    constructor(url) {
        super();
        if (url === undefined || typeof url !== 'string') {
            throw new TypeError('URL must be a string, got ' + typeof url);
        }
        this.raw = _ws.NewWebSocket(this, url, process.dispatchEvent);
        this.readyState = WebSocket.CONNECTING;
        this.url = url;

        setImmediate(() => {
            try {
                const err = this.raw.connect();
                if (err instanceof Error) {
                    this.emit('error', err);
                } else {
                    this.readyState = WebSocket.OPEN;
                    this.emit('open');
                }
            } catch (e) {
                this.emit('error', new Error(e.value));
            }
        });
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

module.exports = {
    WebSocket: WebSocket,
}