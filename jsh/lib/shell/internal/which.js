((obj) => {
    try {
        const process = require("/lib/process");
        const where = process.which(obj);
        console.println(where);
        return 0;
    } catch (e) {
        console.error(e.message)
        console.error(`which: command not found: ${obj}`);
        return 1;
    }
})
