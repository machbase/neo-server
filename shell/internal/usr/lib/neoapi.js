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
            return this._rpcRequest('markdownRender', [mdText, darkMode]);
        });
    }
    getServicePorts(service = '') {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getServicePorts', [service]);
        });
    }
    getServerInfo() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getServerInfo', []);
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
    getShellList() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getShellList', []);
        });
    }
    addShell(label, command) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('addShell', [label, command]);
        });
    }
    deleteShell(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('deleteShell', [id]);
        });
    }
    getBridgeList() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getBridgeList', []);
        });
    }
    addBridge(name, type, conn) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('addBridge', [name, type, conn]);
        });
    }
    deleteBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('deleteBridge', [name]);
        });
    }
    testBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('testBridge', [name]);
        });
    }
    statsBridge(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('statsBridge', [name]);
        });
    }
    execBridge(name, command) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('execBridge', [name, command]);
        });
    }
    queryBridge(name, query) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('queryBridge', [name, query]);
        });
    }
    fetchResultBridge(handle) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('fetchResultBridge', [handle]);
        });
    }
    closeResultBridge(handle) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('closeResultBridge', [handle]);
        });
    }
    listSSHKeys() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('listSSHKeys', []);
        });
    }
    addSSHKey(type, key, comment) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('addSSHKey', [type, key, comment]);
        });
    }
    deleteSSHKey(key) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('deleteSSHKey', [key]);
        });
    }
    listKeys() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('listKeys', []);
        });
    }
    genKey(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('genKey', [id]);
        });
    }
    deleteKey(id) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('deleteKey', [id]);
        });
    }
    getServerCertificate() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getServerCertificate', []);
        });
    }
    listSchedules() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('listSchedules', []);
        });
    }
    addSchedule(sch) {
        const name = sch.name;
        const type = sch.type.toUpperCase();
        const spec = sch.spec;
        const task = sch.task;
        const bridge = sch.bridge;
        const topic = sch.topic;
        const autoStart = sch.autoStart || false;

        return this._executeWithAuth(() => {
            if (type === 'SUBSCRIBER') {
                return this._rpcRequest('addSubscriberSchedule',
                    [name, bridge, topic, task, autoStart]);
            } else if (type === 'TIMER') {
                return this._rpcRequest('addTimerSchedule',
                    [name, spec, task, autoStart]);
            } else {
                throw new Error(`Unsupported schedule type: ${type}`);
            }
        });
    }
    deleteSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('deleteSchedule', [name]);
        });
    }
    startSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('startSchedule', [name]);
        });
    }
    stopSchedule(name) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('stopSchedule', [name]);
        });
    }
    listSessions() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('listSessions', []);
        });
    }
    killSession(id, force = false) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('killSession', [id, force]);
        });
    }
    statSession(reset = false) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('statSession', [reset]);
        });
    }
    getSessionLimit() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('getSessionLimit', []);
        });
    }
    setSessionLimit(limit = {}) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('setSessionLimit', [limit]);
        });
    }
    shutdownServer() {
        return this._executeWithAuth(() => {
            return this._rpcRequest('shutdownServer', []);
        });
    }
    splitSqlStatements(sqlText) {
        return this._executeWithAuth(() => {
            return this._rpcRequest('splitSqlStatements', [sqlText]);
        });
    }
}

module.exports = {
    Client
}