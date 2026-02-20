'use strict";'

const {sayHello} = require("demo");
const {argv} = require("process");
const args = argv.slice(2);

sayHello(args.join(" "));
