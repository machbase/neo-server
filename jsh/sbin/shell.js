(() => {
    const process = require('@jsh/process');
    const m = require("@jsh/shell");
    const r = new m.Shell();
    r.run(process.env);
})()