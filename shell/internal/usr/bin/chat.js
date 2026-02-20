'use strict';

const http = require('http');
const { WebSocket } = require('ws');
const { ReadLine } = require('readline');
const { Chat } = require('/usr/lib/chat');

function question(chat, ws, msg) {
    chat.endAnswer = false;
    const quest = JSON.stringify({
        type: 'msg',
        msg: {
            ver: '1.0',
            id: 1234,
            type: 'question',
            body: {
                provider: 'claude',
                model: 'claude-haiku-4-5-20251001',
                text: msg
            }
        },
    });
    try {
        ws.send(quest);
    } catch (e) {
        console.error("Failed to send question:", e.message);
        return;
    }

    const handle = (ws) => {
        if (!chat.endAnswer) {
            setImmediate(() => {
                handle(ws);
            });
            return;
        }
        console.println("\n"); // Answer complete.
        setImmediate(() => {
            loop(chat, ws);
        });
    }
    setImmediate(() => {
        handle(ws);
    });
}

const r = new ReadLine({
    history: 'neo-chat-history',
    prompt: (lineno) => {
        return lineno == 0 ? `chat > ` : `.... > `;
    },
});

function loop(chat, ws) {
    if (!chat.connected) {
        console.println('WebSocket is not connected.');
        return;
    }
    const line = r.readLine();
    if (line instanceof Error) {
        throw line;
    }
    if (line === null || line.toLowerCase() === 'exit') {
        console.println('Exiting...');
        ws.close();
        return;
    }

    if (line.trim() === '') {
        setImmediate(() => {
            loop(chat, ws);
        });
    } else {
        question(chat, ws, line);
    }
}

const chat = new Chat({ host: '192.168.1.165', port: 5654 });
chat.login()
    .then((wsUrl) => {
        const ws = new WebSocket(wsUrl);
        ws.on('open', () => {
            setImmediate(() => {
                chat.connected = true;
                console.println('You can type your messages now. Type "exit" to quit.');
                loop(chat, ws);
            });
        });
        ws.on('error', (err) => {
            console.println(err.message);
        });
        ws.on('close', () => {
            chat.connected = false;
        });
        ws.on('message', (data) => {
            chat.answering(data);
        });
    })
    .catch((err) => {
        console.error('Login failed:', err.message);
    });