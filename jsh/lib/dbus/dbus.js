'use strict';

/**
 * D-Bus module for JSH.
 * Provides a thin wrapper around the native D-Bus connection bindings.
 */

const _dbus = require('@jsh/dbus');
const EventEmitter = require('events');
const process = require('process');

class ObjectProxy {
    constructor(connection, destination, path) {
        this._connection = connection;
        this.destination = destination;
        this.path = path;
    }

    call(method) {
        return this._connection.call({
            destination: this.destination,
            path: this.path,
            method,
            args: Array.prototype.slice.call(arguments, 1),
        });
    }

    getProperty(name, interfaceName) {
        return this._connection.getProperty({
            destination: this.destination,
            path: this.path,
            interface: interfaceName,
            name,
        });
    }

    get(name, interfaceName) {
        const result = this.getProperty(name, interfaceName);
        return result ? result.value : undefined;
    }

    setProperty(name, value, interfaceName) {
        return this._connection.setProperty({
            destination: this.destination,
            path: this.path,
            interface: interfaceName,
            name,
            value,
        });
    }

    set(name, value, interfaceName) {
        return this.setProperty(name, value, interfaceName);
    }

    introspect() {
        return this._connection.introspect({
            destination: this.destination,
            path: this.path,
        });
    }

    subscribeSignal(member, interfaceName) {
        return this._connection.subscribeSignal({
            destination: this.destination,
            path: this.path,
            interface: interfaceName,
            member,
        });
    }

    unsubscribeSignal(member, interfaceName) {
        return this._connection.unsubscribeSignal({
            destination: this.destination,
            path: this.path,
            interface: interfaceName,
            member,
        });
    }
}

/**
 * D-Bus connection wrapper.
 *
 * Wraps the native connection returned by `_dbus.newConnection(opt)` and exposes
 * a JavaScript class interface.
 *
 * @param {Object} [opt] - Connection options.
 * @param {string} [opt.busType=BusType.Session] - D-Bus bus type to connect to.
 */
class Connection extends EventEmitter {
    constructor(opt) {
        super();
        if (!opt) {
            opt = {};
        }
        if (opt.busType === undefined) {
            opt.busType = BusType.Session;
        }
        this._connection = _dbus.newConnection(this, process.dispatchEvent, opt);
        if (!this._connection) {
            throw new Error('failed to create D-Bus connection');
        }
    }

    close() {
        if (!this._connection) {
            return;
        }
        const connection = this._connection;
        this._connection = null;
        return connection.close(...arguments);
    }

    call(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return this._connection.call(request);
    }

    getProperty(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return this._connection.getProperty(request);
    }

    setProperty(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return this._connection.setProperty(request);
    }

    introspect(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return this._connection.introspect(request);
    }

    subscribeSignal(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        this._connection.subscribeSignal(request);
        return this;
    }

    watchName(name) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        this._connection.watchName({ name });
        return this;
    }

    getNameOwner(name) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return this._connection.getNameOwner({ name });
    }

    unsubscribeSignal(request) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        this._connection.unsubscribeSignal(request);
        return this;
    }

    unwatchName(name) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        this._connection.unwatchName({ name });
        return this;
    }

    object(destination, path) {
        if (!this._connection) {
            throw new Error('connection not initialized');
        }
        return new ObjectProxy(this, destination, path);
    }
}

const BusType = _dbus.BusType;

module.exports = {
    Connection,
    BusType,
};
