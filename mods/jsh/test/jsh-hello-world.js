console.log("Hello,", "World!");

const process = require("@jsh/process");
process.cd("/etc_services/");
console.log("Current directory:", process.cwd());
