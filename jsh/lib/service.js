'use strict';

const process = require('process');
const net = require('net');

const defaultTimeout = 5000;

class Client {
    constructor(options) {
        const normalized = normalizeClientOptions(options);
        this.controller = normalized.controller;
        this.timeout = normalized.timeout;
        this.endpoint = parseController(this.controller);
        this._nextID = 1;
        this.runtime = {
            get: (name, callback) => this.runtimeGet(name, callback),
        };
        this.details = {
            get: (name, key, callback) => this.detailsGet(name, key, callback),
            add: (name, key, value, callback) => this.detailWrite('service.runtime.detail.add', name, key, value, callback),
            update: (name, key, value, callback) => this.detailWrite('service.runtime.detail.update', name, key, value, callback),
            set: (name, key, value, callback) => this.detailWrite('service.runtime.detail.set', name, key, value, callback),
            delete: (name, key, callback) => this.detailsDelete(name, key, callback),
        };
        this.commands = {
            status: (name, callback) => this.status(name, callback),
            read: (callback) => this.read(callback),
            update: (callback) => this.update(callback),
            reload: (callback) => this.reload(callback),
            install: (config, callback) => this.install(config, callback),
            uninstall: (name, callback) => this.uninstall(name, callback),
            start: (name, callback) => this.start(name, callback),
            stop: (name, callback) => this.stop(name, callback),
        };
    }

    call(method, params, callback) {
        if (typeof callback !== 'function') {
            throw new Error('service callback is required');
        }
        if (!method) {
            throw new Error('service RPC method is required');
        }

        const request = {
            jsonrpc: '2.0',
            id: this._nextID++,
            method,
        };
        if (params !== null && params !== undefined) {
            request.params = params;
        }

        sendRPCRequest(this.endpoint, request, this.timeout, callback);
        return this;
    }

    list(callback) {
        return this.call('service.list', null, callback);
    }

    get(name, callback) {
        return this.call('service.get', { name: requireName(name) }, callback);
    }

    status(name, callback) {
        if (typeof name === 'function') {
            callback = name;
            name = '';
        }
        if (name === undefined || name === null || name === '') {
            return this.list(callback);
        }
        return this.get(name, callback);
    }

    read(callback) {
        return this.call('service.read', null, callback);
    }

    update(callback) {
        return this.call('service.update', null, callback);
    }

    reload(callback) {
        return this.call('service.reload', null, callback);
    }

    install(config, callback) {
        if (!config || typeof config !== 'object') {
            throw new Error('service config is required');
        }
        return this.call('service.install', config, callback);
    }

    uninstall(name, callback) {
        return this.call('service.uninstall', { name: requireName(name) }, callback);
    }

    start(name, callback) {
        return this.call('service.start', { name: requireName(name) }, callback);
    }

    stop(name, callback) {
        return this.call('service.stop', { name: requireName(name) }, callback);
    }

    runtimeGet(name, callback) {
        return this.call('service.runtime.get', { name: requireName(name) }, callback);
    }

    detailsGet(name, key, callback) {
        if (typeof key === 'function') {
            callback = key;
            key = '';
        }

        name = requireName(name);
        const detailKey = normalizeDetailKey(key, false);
        return this.runtimeGet(name, (err, runtime) => {
            if (err) {
                callback(err);
                return;
            }
            if (detailKey && !hasDetailKey(runtime, detailKey)) {
                callback(new Error(`Detail '${detailKey}' not found for service '${name}'.`));
                return;
            }
            callback(null, runtime);
        });
    }

    detailWrite(method, name, key, value, callback) {
        return this.call(method, {
            name: requireName(name),
            key: normalizeDetailKey(key, true),
            value,
        }, callback);
    }

    detailsDelete(name, key, callback) {
        return this.call('service.runtime.detail.delete', {
            name: requireName(name),
            key: normalizeDetailKey(key, true),
        }, callback);
    }
}

function call(method, params, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).call(method, params, callback);
}

function list(options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).list(callback);
}

function get(name, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).get(name, callback);
}

function status(name, options, callback) {
    if (typeof name === 'function') {
        callback = name;
        name = '';
        options = undefined;
    } else if (typeof name === 'object' && name !== null) {
        callback = options;
        options = name;
        name = '';
    } else if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).status(name, callback);
}

function read(options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).read(callback);
}

function update(options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).update(callback);
}

function reload(options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).reload(callback);
}

function install(config, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).install(config, callback);
}

function uninstall(name, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).uninstall(name, callback);
}

function start(name, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).start(name, callback);
}

function stop(name, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).stop(name, callback);
}

function runtimeGet(name, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).runtime.get(name, callback);
}

function detailsGet(name, key, options, callback) {
    if (typeof key === 'function') {
        callback = key;
        key = '';
        options = undefined;
    } else if (typeof key === 'object' && key !== null) {
        callback = options;
        options = key;
        key = '';
    } else if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).details.get(name, key, callback);
}

function detailsWrite(method, name, key, value, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).call(method, {
        name: requireName(name),
        key: normalizeDetailKey(key, true),
        value,
    }, callback);
}

function detailsDelete(name, key, options, callback) {
    if (typeof options === 'function') {
        callback = options;
        options = undefined;
    }
    return new Client(options).details.delete(name, key, callback);
}

function normalizeClientOptions(options) {
    if (typeof options === 'string') {
        options = { controller: options };
    } else if (!options) {
        options = {};
    }

    const controller = resolveController(options.controller);
    if (!controller) {
        throw new Error('Option --controller=<host:port|unix://path> is required.');
    }

    const timeout = options.timeout === undefined ? defaultTimeout : options.timeout;
    if (!Number.isInteger(timeout) || timeout <= 0) {
        throw new Error(`Invalid timeout '${timeout}'. Use a positive integer.`);
    }

    return { controller, timeout };
}

function resolveController(value) {
    if (value) {
        return String(value);
    }
    const envValue = process.env.get('SERVICE_CONTROLLER');
    return envValue ? String(envValue) : '';
}

function parseController(value) {
    if (typeof value !== 'string' || value === '') {
        throw new Error('Option --controller=<host:port|unix://path> is required.');
    }
    if (value.startsWith('unix://')) {
        const socketPath = value.slice(7);
        if (!socketPath) {
            throw new Error(`Invalid controller socket path in '${value}'.`);
        }
        return { network: 'unix', path: socketPath };
    }

    if (value.startsWith('tcp://')) {
        value = value.slice(6);
    }

    const idx = value.lastIndexOf(':');
    if (idx <= 0 || idx === value.length - 1) {
        throw new Error(`Invalid controller address '${value}'. Expected host:port.`);
    }
    const host = value.slice(0, idx);
    const portText = value.slice(idx + 1);
    const port = parseInt(portText, 10);
    if (!host) {
        throw new Error(`Invalid controller host in '${value}'.`);
    }
    if (!Number.isInteger(port) || port <= 0 || port > 65535) {
        throw new Error(`Invalid controller port '${portText}'.`);
    }
    return { network: 'tcp', host, port };
}

function sendRPCRequest(endpoint, request, timeoutMsec, callback) {
    const socket = endpoint.network === 'unix'
        ? net.createConnection({ path: endpoint.path })
        : net.createConnection({ host: endpoint.host, port: endpoint.port });
    let buffer = '';
    let settled = false;
    const hold = createRequestHold(timeoutMsec);

    function settle(err, result) {
        if (settled) {
            return;
        }
        settled = true;
        releaseRequestHold(hold);
        callback(err, result);
        try {
            socket.end();
        } catch (endErr) {
            try {
                socket.destroy();
            } catch (destroyErr) {
            }
        }
    }

    socket.setTimeout(timeoutMsec);

    socket.on('connect', () => {
        socket.write(JSON.stringify(request) + '\n');
    });

    socket.on('data', (chunk) => {
        buffer += chunk.toString();
        let response;
        try {
            response = JSON.parse(buffer);
        } catch (err) {
            return;
        }
        if (response.error) {
            settle(new Error(response.error.message || JSON.stringify(response.error)));
            return;
        }
        settle(null, response.result);
    });

    socket.on('timeout', () => {
        settle(new Error(`RPC timeout after ${timeoutMsec}ms`));
    });

    socket.on('error', (err) => {
        settle(err);
    });

    socket.on('end', () => {
        if (!settled) {
            settle(new Error('Controller closed the connection before sending a complete response.'));
        }
    });
}

function createRequestHold(timeoutMsec) {
    if (!Number.isInteger(timeoutMsec) || timeoutMsec <= 0) {
        return null;
    }
    return setTimeout(() => { }, timeoutMsec + 1000);
}

function releaseRequestHold(handle) {
    if (handle) {
        clearTimeout(handle);
    }
}

function hasDetailKey(runtime, key) {
    const details = runtime && runtime.details && typeof runtime.details === 'object' ? runtime.details : null;
    return !!details && Object.prototype.hasOwnProperty.call(details, key);
}

function requireName(name) {
    if (typeof name !== 'string' || name === '') {
        throw new Error('service name is required');
    }
    return name;
}

function normalizeDetailKey(key, required) {
    if (key === undefined || key === null || key === '') {
        if (required) {
            throw new Error('service detail key is required');
        }
        return '';
    }
    return String(key);
}

module.exports = {
    Client,
    resolveController,
    parseController,
    call,
    list,
    get,
    status,
    read,
    update,
    reload,
    install,
    uninstall,
    start,
    stop,
    commands: {
        status,
        read,
        update,
        reload,
        install,
        uninstall,
        start,
        stop,
    },
    runtime: {
        get: runtimeGet,
    },
    details: {
        get: detailsGet,
        add: (name, key, value, options, callback) => detailsWrite('service.runtime.detail.add', name, key, value, options, callback),
        update: (name, key, value, options, callback) => detailsWrite('service.runtime.detail.update', name, key, value, options, callback),
        set: (name, key, value, options, callback) => detailsWrite('service.runtime.detail.set', name, key, value, options, callback),
        delete: detailsDelete,
    },
};