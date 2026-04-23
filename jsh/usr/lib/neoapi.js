'use strict';

const http = require('http');
const { getHttpConfig, setHttpToken, getHttpAccessToken, getHttpRefreshToken } = require('@jsh/session');

class _Client {
    constructor(options = {}) {
        this.options = { ...getHttpConfig(), ...options }
    }
    login() {
        return new Promise((resolve, reject) => {
            const req = http.request({
                method: 'POST',
                protocol: this.options.protocol,
                host: this.options.host,
                port: this.options.port,
                path: '/web/api/login',
                headers: {
                    'Content-Type': 'application/json'
                }
            });
            req.on('response', (res) => {
                const result = res.json();
                if (!result.success) {
                    reject(new Error('Login failed: ' + result.reason));
                    return;
                }
                setHttpToken(result.accessToken, result.refreshToken);
                resolve(result.reason);
            });
            req.on('error', (err) => {
                reject(err);
            });
            const body = JSON.stringify({
                loginName: this.options.user,
                password: this.options.password
            });
            req.write(body);
            req.end();
        });
    }
    relogin() {
        return new Promise((resolve, reject) => {
            if (!getHttpRefreshToken()) {
                reject(new Error('No refresh token available'));
                return;
            }
            const req = http.request({
                method: 'POST',
                protocol: this.options.protocol,
                host: this.options.host,
                port: this.options.port,
                path: '/web/api/relogin',
                headers: {
                    'Content-Type': 'application/json'
                }
            });
            req.on('response', (res) => {
                const result = res.json();
                if (!result.success) {
                    reject(new Error('Relogin failed: ' + result.reason));
                    return;
                }
                setHttpToken(result.accessToken, result.refreshToken);
                resolve(result.reason);
            });
            req.on('error', (err) => {
                reject(err);
            });
            const body = JSON.stringify({
                refreshToken: getHttpRefreshToken()
            });
            req.write(body);
            req.end();
        });
    }

    /**
     * Executes an authenticated request.
     * Automatically handles login and relogin on 401 errors.
     * @param {Function} requestFn - Function that executes the request (must return a Promise)
     * @returns {Promise} Request result
     */
    _executeWithAuth(requestFn) {
        const executeRequest = () => {
            return requestFn().catch((err) => {
                // If 401 error, attempt relogin and retry request
                if (err.unauthorized) {
                    return this.relogin().then(() => {
                        return requestFn();
                    }).catch((err) => {
                        throw err;
                    });
                }
                throw err;
            });
        };

        // Attempt login if accessToken is not available
        if (!getHttpAccessToken()) {
            return this.login().then(() => {
                return executeRequest();
            }).catch((err) => {
                return Promise.reject(err);
            });
        }

        return executeRequest();
    }

    /**
     * Makes a JSON-RPC request to the server.
     * @param {string} method - The RPC method name
     * @param {Array} params - The method parameters
     * @returns {Promise} The RPC result
     */
    _rpcRequest(method, params = []) {
        return new Promise((resolve, reject) => {
            const req = http.request({
                method: 'POST',
                protocol: this.options.protocol,
                host: this.options.host,
                port: this.options.port,
                path: '/web/api/rpc',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${getHttpAccessToken()}`
                }
            });
            req.on('response', (res) => {
                // Mark rejection with special flag if 401 error
                if (res.statusCode === 401) {
                    reject({ unauthorized: true });
                    return;
                }
                if (res.statusCode < 200 || res.statusCode >= 300) {
                    reject(new Error(res.statusMessage));
                    return;
                }
                const result = res.json();
                if (result.error) {
                    reject(new Error('JSON-RPC error: ' + (result.error.message || JSON.stringify(result.error))));
                    return;
                }
                resolve(result.result);
            });
            req.on('error', (err) => {
                reject(err);
            });
            const body = JSON.stringify({
                jsonrpc: '2.0',
                method: method,
                params: params,
                id: Date.now()
            });
            req.write(body);
            req.end();
        });
    }
}

let _instance = null;

class Client extends _Client {
    constructor(options = {}) {
        // Return existing instance if already created
        if (_instance) {
            return _instance;
        }

        super(options);
        _instance = this;
    }

    markdownRender(mdText, darkMode = false) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('markdown.render', [mdText, darkMode]);
        });
    }
    getServicePorts(service = '') {
        return this._executeWithAuth(() => {
            return this._rpcRequest('service.port.list', [service]);
        });
    }
    getServerInfo() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('server.info.get', []);
        });
    }
    getMachbasePort(callback) {
        this.getServicePorts('mach')
            .then((data) => {
                let ports = {};
                for (const s of data) {
                    if (ports[s.Service]) {
                        ports[s.Service].push(s.Address);
                    } else {
                        ports[s.Service] = [s.Address];
                    }
                }
                let host = ports['mach'][0];
                host = host.replace('tcp://', '');
                callback(host, null);
            })
            .catch((err) => {
                callback(null, err);
            });
    }
    listShells() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('shell.list', []);
        });
    }
    addShell(label, command) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('shell.add', [label, command]);
        });
    }
    deleteShell(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('shell.delete', [id]);
        });
    }
    listBridges() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.list', []);
        });
    }
    addBridge(name, type, conn) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.add', [name, type, conn]);
        });
    }
    deleteBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.delete', [name]);
        });
    }
    testBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.test', [name]);
        });
    }
    statsBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.stats', [name]);
        });
    }
    execBridge(name, command) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.exec', [name, command]);
        });
    }
    queryBridge(name, query) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.query', [name, query]);
        });
    }
    fetchResultBridge(handle) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.result.fetch', [handle]);
        });
    }
    closeResultBridge(handle) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('bridge.result.close', [handle]);
        });
    }
    listSSHKeys() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('sshkey.list', []);
        });
    }
    addSSHKey(type, key, comment) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('sshkey.add', [type, key, comment]);
        });
    }
    deleteSSHKey(key) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('sshkey.delete', [key]);
        });
    }
    listKeys() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('key.list', []);
        });
    }
    genKey(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('key.generate', [id]);
        });
    }
    deleteKey(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('key.delete', [id]);
        });
    }
    getServerCertificate() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('server.certificate.get', []);
        });
    }
    listSchedules() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('schedule.list', []);
        });
    }
    addSchedule(sch) {
        const name = sch.name;
        const type = sch.type.toUpperCase();
        const spec = sch.spec;
        const task = sch.task;
        const bridge = sch.bridge;
        const topic = sch.topic;
        const autostart = sch.autostart;
        const qos = sch.qos || 0; // TODO: handle qos for subscriber type in the future

        return this._executeWithAuth(() => {
            if (type === 'SUBSCRIBER') {
                return this._rpcRequest('schedule.subscriber.add',
                    [name, bridge, task, true, topic, qos]);
            } else if (type === 'TIMER') {
                return this._rpcRequest('schedule.timer.add',
                    [name, spec, task, autostart]);
            } else {
                throw new Error(`Unsupported schedule type: ${type}`);
            }
        });
    }
    deleteSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('schedule.delete', [name]);
        });
    }
    startSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('schedule.start', [name]);
        });
    }
    stopSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('schedule.stop', [name]);
        });
    }
    listSessions() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('session.list', []);
        });
    }
    killSession(id, force = false) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('session.kill', [id, force]);
        });
    }
    statSession(reset = false) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('session.stat', [reset]);
        });
    }
    getSessionLimit() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('session.limit.get', []);
        });
    }
    setSessionLimit(limit = {}) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('session.limit.set', [limit]);
        });
    }
    shutdownServer() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('server.shutdown', []);
        });
    }
    splitSqlStatements(sqlText) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('sql.split', [sqlText]);
        });
    }
    setHttpDebug(conf = {}) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('http.debug.set', [conf]);
        });
    }
}

module.exports = {
    Client
}
