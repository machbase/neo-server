'use strict';

const process = require('process');

const signalName = process.env.get('TEST_SIGNAL') || 'SIGTERM';
const timer = setInterval(() => { }, 1000);

process.addShutdownHook(() => {
    console.println('shutdown:', process.pid);
});

process.on(signalName, () => {
    console.println('caught:', signalName);
    console.println('cleanup:', process.pid);
    clearInterval(timer);
});

console.println('child-ready:', process.pid);