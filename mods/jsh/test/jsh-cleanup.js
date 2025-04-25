jsh = require('@jsh/process');

c1 = jsh.addCleanup(function () {
    console.log("Running cleanup code1...");
});

c2 = jsh.addCleanup(function () {
    console.log("Should not run this code2...");
});

c3 = jsh.addCleanup(function () {
    console.log("Running cleanup code3...");
});

jsh.removeCleanup(c2);
