'use strict';

const EventEmitter = require('events');
const process = require('process');
const _mqtt = require('@jsh/mqtt');

if (typeof Buffer !== 'undefined' && !Buffer.isBuffer) {
    Buffer.isBuffer = function (obj) {
        return obj instanceof Uint8Array || (obj && obj.constructor && obj.constructor.name === 'Buffer');
    };
}

class Client extends EventEmitter {
    constructor(options) {
        super();
        this.config = _mqtt.parseConfig(JSON.stringify(options));

        this.raw = _mqtt.NewClient(this, process.dispatchEvent);
        setImmediate(() => {
            let err = this.raw.connect(this.config);
            if (err instanceof Error) {
                this.emit('error', err);
                return;
            }
            this.emit('open');
            // keep event loop alive until connection is closed
            const interval = setInterval(() => {
                if (this.raw.isClosed()) {
                    clearInterval(interval);
                }
            }, 200);
        });
    }

    emit(event, ...args) {
        if (event === 'message' && args.length > 0 && args[0] && typeof args[0] === 'object') {
            const msg = args[0];
            if (typeof Buffer !== 'undefined') {
                if (msg.payload && typeof msg.payload.byteLength === 'number') {
                    msg.payload = Buffer.from(new Uint8Array(msg.payload));
                } else if (msg.payload instanceof Uint8Array && !Buffer.isBuffer(msg.payload)) {
                    msg.payload = Buffer.from(msg.payload);
                }
                if (Buffer.isBuffer(msg.payload) && typeof msg.payloadText === 'undefined') {
                    msg.payloadText = msg.payload.toString();
                }
                if (msg.properties && msg.properties.correlationData && typeof msg.properties.correlationData.byteLength === 'number') {
                    msg.properties.correlationData = Buffer.from(new Uint8Array(msg.properties.correlationData));
                }
            }
        }
        return super.emit(event, ...args);
    }

    publish(topic, message, options) {
        try {
            let reason = this.raw.publish(topic, message, options);
            this.emit('published', topic, reason);
        } catch (err) {
            this.emit('error', err);
        }
    }

    subscribe(topic, options) {
        try {
            let reason = this.raw.subscribe(topic, options);
            this.emit('subscribed', topic, reason);
        } catch (err) {
            this.emit('error', err);
        }
    }

    unsubscribe(topic, options) {
        try {
            let reason = this.raw.unsubscribe(topic, options);
            this.emit('unsubscribed', topic, reason);
        } catch (err) {
            this.emit('error', err);
        }
    }

    close() {
        this.raw.disconnect();
        this.emit('close');
    }
}

module.exports = {
    Client,
}