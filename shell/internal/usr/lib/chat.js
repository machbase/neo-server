'use strict';

const EventEmitter = require('/lib/events');
const { getHttpConfig, setHttpToken, getHttpAccessToken, getHttpRefreshToken } = require('@jsh/session');

// events: "answer-start", "answer-stop"
class Chat extends EventEmitter {
    constructor(options = {}) {
        super();

        this.options = getHttpConfig();
        this.options = { ...this.options, ...options }
    }
    login() {
        console.println(`Logging in to ${this.options.host}:${this.options.port} as ${this.options.user}...`);
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
                this.accessToken = result.accessToken;
                this.refreshToken = result.refreshToken;
                const wsUrl = `${this.options.protocol === 'https:' ? 'wss:' : 'ws:'}//${this.options.host}:${this.options.port}/web/api/console/1234/data?token=${this.accessToken}`;
                resolve(wsUrl);
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
    answering(reply) {
        // reply: {data:{"type":"msg","msg":{"body":null,"id":1234,"type":"answer-start","ver":"1.0"}}
        const obj = JSON.parse(reply.data);
        switch (obj.type) {
            case 'msg':
                switch (obj.msg.type) {
                    case 'answer-start':
                        this.endAnswer = false;
                    case 'stream-message-start':
                    case 'stream-block-start':
                    case 'stream-block-delta':
                    case 'stream-block-stop':
                    case 'stream-message-stop':
                        if (obj.msg.body && obj.msg.body.data && obj.msg.body.data.length > 0) {
                            let text = obj.msg.body.data;
                            console.print(text)
                            this.emit('answer-start', text);
                        }
                        break;
                    case 'answer-stop':
                        this.endAnswer = true;
                        this.emit('answer-stop', obj.msg.body);
                        break;
                    default:
                        console.println('Unknown msg type:', obj.msg.type);
                        return;
                }
                break;
            default:
                console.println('Unknown data type:', reply.data.trim());
                return;
        }
    }
}

module.exports = {
    Chat
}