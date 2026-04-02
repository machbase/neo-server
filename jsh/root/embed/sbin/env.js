(() => {
    const process = require('/lib/process');
    const entries = [];
    const names = process.argv.slice(2);

    if (names.length > 0) {
        names.forEach((name) => {
            const value = process.env.get(name);
            if (value !== null && value !== undefined) {
                entries.push(`${name}=${value}`);
            }
        });
        entries.forEach((entry) => console.println(entry));
        return;
    }

    process.env.forEach((key, value) => {
        entries.push(`${key}=${value}`);
    });

    entries.sort().forEach((entry) => console.println(entry));
})()