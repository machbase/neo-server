((targetDir) => {
    try {
        const process = require("/lib/process");
        process.chdir(targetDir);
        return 0;
    } catch (e) {
        console.error(e.message)
        console.error(`cd: no such file or directory: ${targetDir || "~"}`);
        return 1;
    }
})
