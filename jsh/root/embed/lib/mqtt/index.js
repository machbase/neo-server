'use strict';

const EventEmitter = require('/lib/events');
const process = require('/lib/process');
const _mqtt = require('@jsh/mqtt');

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

    close() {
        this.raw.disconnect();
        this.emit('close');
    }
}

module.exports = {
    Client,
}