'use strict';

const process = require('process');
const env = process.env;

const user = env.get('NEOSHELL_USER');
const password = env.get('NEOSHELL_PASSWORD');

env.set('NEOSHELL_USER', null);
env.set('NEOSHELL_PASSWORD', null);

process.exec('neo-shell')

console.println("disconnected from neo-shell");
env.set('NEOSHELL_USER', user);
env.set('NEOSHELL_PASSWORD', password);