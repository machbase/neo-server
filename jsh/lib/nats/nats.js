'use strict';

const EventEmitter = require('events');
const process = require('process');
const _nats = require('@jsh/nats');

class Client extends EventEmitter {
    constructor(options) {
        super();
        this.config = _nats.parseConfig(JSON.stringify(options));

        this.raw = _nats.NewClient(this, process.dispatchEvent);
        setImmediate(() => {
            if (this.raw.isClosed()) {
                return;
            }
            const err = this.raw.connect(this.config);
            if (err instanceof Error) {
                this.emit('error', err);
                return;
            }
            this.emit('open');

            const interval = setInterval(() => {
                if (this.raw.isClosed()) {
                    clearInterval(interval);
                }
            }, 200);
        });
    }

    publish(topic, message, options) {
        try {
            const reason = this.raw.publish(topic, message, options);
            this.emit('published', topic, reason);
        } catch (err) {
            this.emit('error', err);
        }
    }

    subscribe(topic, options) {
        try {
            const reason = this.raw.subscribe(topic, options);
            this.emit('subscribed', topic, reason);
        } catch (err) {
            this.emit('error', err);
        }
    }

    close() {
        this.raw.close();
        this.emit('close');
    }
}

module.exports = {
    Client,
};