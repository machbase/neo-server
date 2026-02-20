(() => {
    const process = require('process');
    const parseArgs = require('util/parseArgs');
    const pwd = process.env.get("PWD");
    const fs = process.env.filesystem();

    // Parse command line arguments
    const { values, positionals } = parseArgs(process.argv.slice(2), {
        options: {
            long: { type: 'boolean', short: 'l', default: false },
            all: { type: 'boolean', short: 'a', default: false },
            time: { type: 'boolean', short: 't', default: false },
            recursive: { type: 'boolean', short: 'R', default: false }
        },
        strict: false,
        allowPositionals: true
    });

    // ANSI color codes
    const colors = {
        reset: "\x1b[0m",
        blue: "\x1b[34m",      // directory
        cyan: "\x1b[36m",      // symlink
        green: "\x1b[32m",     // executable
        yellow: "\x1b[33m",    // device
        magenta: "\x1b[35m",   // pipe/socket
        red: "\x1b[31m",       // archive
        white: "\x1b[37m"      // regular file
    };

    // Extract flags and directories
    const longFormat = values.long;
    const showAll = values.all;
    const sortByTime = values.time;
    const recursive = values.recursive;

    let rawArgs = positionals.length > 0 ? positionals : [pwd];

    let showDir = false;

    // Get color for file based on mode
    let getColor = function (nfo) {
        const mode = nfo.mode();
        const modeStr = mode.string();
        const fileName = nfo.name();

        if (modeStr.startsWith("d")) {
            return colors.blue;  // directory
        } else if (modeStr.startsWith("l")) {
            return colors.cyan;  // symlink
        } else if (modeStr.startsWith("c") || modeStr.startsWith("b")) {
            return colors.yellow;  // character/block device
        } else if (modeStr.startsWith("p") || modeStr.startsWith("s")) {
            return colors.magenta;  // pipe or socket
        } else if (fileName.endsWith(".js")) {
            return colors.yellow;  // JavaScript files
        } else if (modeStr.includes("x")) {
            return colors.green;  // executable
        } else {
            return colors.white;  // regular file
        }
    };

    // Print function for detailed listing (-l)
    let printDetailed = function (nfo, idx) {
        const color = getColor(nfo);
        console.printf(`%-12s %10d %v %s%s%s\n`,
            nfo.mode().string(), nfo.size(), nfo.modTime(),
            color, nfo.name(), colors.reset);
    };

    // Print function for simple listing (no -l)
    let printSimple = function (nfo, idx) {
        const color = getColor(nfo);
        console.printf(`%s%s%s  `, color, nfo.name(), colors.reset);
    };

    let print = longFormat ? printDetailed : printSimple;

    let hasWildcard = function (value) {
        return /[*?]/.test(value);
    };

    let escapeRegex = function (value) {
        return value.replace(/[.+^${}()|[\]\\]/g, "\\$&");
    };

    let globToRegex = function (pattern) {
        const escaped = escapeRegex(pattern)
            .replace(/\\\*/g, ".*")
            .replace(/\\\?/g, ".");
        return new RegExp("^" + escaped + "$");
    };

    let splitPath = function (value) {
        const idx = value.lastIndexOf("/");
        if (idx === -1) {
            return { dir: "", base: value };
        }
        const dir = value.slice(0, idx) || "/";
        const base = value.slice(idx + 1);
        return { dir: dir, base: base };
    };

    let resolvePath = function (value) {
        if (value.startsWith("/")) {
            return value;
        }
        return pwd + "/" + value;
    };

    let joinPath = function (dir, name) {
        if (!dir || dir === ".") {
            return name;
        }
        if (dir.endsWith("/")) {
            return dir + name;
        }
        return dir + "/" + name;
    };

    let expandGlob = function (pattern) {
        if (!hasWildcard(pattern)) {
            return [pattern];
        }

        const parts = splitPath(pattern);
        const dirPart = parts.dir || ".";
        const basePart = parts.base || "";
        const dirPath = resolvePath(dirPart);
        let entries;

        try {
            entries = fs.readDir(dirPath).map((d) => d.info());
        } catch (e) {
            return [];
        }

        const regex = globToRegex(basePart);
        const matchDot = basePart.startsWith(".");

        return entries
            .map((entry) => entry.name())
            .filter((name) => matchDot || !name.startsWith("."))
            .filter((name) => regex.test(name))
            .map((name) => joinPath(parts.dir, name));
    };

    // Helper function to filter entries
    let filterEntries = function (entries) {
        if (!showAll) {
            // Filter out hidden files (starting with .)
            entries = entries.filter((d) => !d.name().startsWith('.'));
        }

        if (sortByTime) {
            // Sort by modification time (newest first)
            entries = entries.sort((a, b) => {
                const aTime = a.modTime();
                const bTime = b.modTime();
                // Direct comparison of time objects
                if (aTime > bTime) return -1;
                if (aTime < bTime) return 1;
                return 0;
            });
        }

        return entries;
    };

    // Helper function to list directory
    let listDirectory = function (dir, prefix) {
        if (!dir.startsWith("/")) {
            dir = pwd + "/" + dir;
        }

        if (prefix) {
            console.println(dir + ":");
        } else if (showDir) {
            console.println(dir + ":");
        }

        const entries = fs.readDir(dir).map((d) => d.info());
        const filtered = filterEntries(entries);

        filtered.forEach(print);

        if (showDir || !longFormat) {
            console.println();
        }

        // Recursively list subdirectories if -R flag is set
        if (recursive) {
            filtered.forEach((nfo) => {
                if (nfo.mode().string().startsWith("d") && nfo.name() !== "." && nfo.name() !== "..") {
                    const subdir = dir + (dir.endsWith("/") ? "" : "/") + nfo.name();
                    console.println();
                    listDirectory(subdir, prefix ? prefix + "  " : "  ");
                }
            });
        }
    };

    let expanded = [];
    let missing = [];

    rawArgs.forEach((arg) => {
        const matches = expandGlob(arg);
        if (matches.length === 0) {
            missing.push(arg);
        } else {
            expanded = expanded.concat(matches);
        }
    });

    showDir = expanded.length > 1;

    missing.forEach((arg) => {
        console.println("ls: cannot access '" + arg + "': No such file or directory");
    });

    let files = [];
    let dirs = [];

    expanded.forEach((path) => {
        const fullPath = resolvePath(path);
        try {
            const info = fs.stat(fullPath);
            const modeStr = info.mode().string();
            if (modeStr.startsWith("d")) {
                dirs.push(path);
            } else {
                files.push(info);
            }
        } catch (e) {
            console.println("ls: cannot access '" + path + "': No such file or directory");
        }
    });

    if (files.length > 0) {
        files.forEach((info) => {
            print(info);
        });
        if (!longFormat) {
            console.println();
        }
        if (dirs.length > 0) {
            console.println();
        }
    }

    dirs.forEach((dir, idx) => {
        listDirectory(dir, null);
        if (idx < dirs.length - 1) {
            console.println();
        }
    });
})()