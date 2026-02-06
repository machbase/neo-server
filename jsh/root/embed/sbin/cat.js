(() => {
    const process = require('process');
    const { parseArgs } = require('util');
    const pwd = process.env.get("PWD");
    const fs = process.env.filesystem();

    // Parse command line arguments
    const { values, positionals } = parseArgs(process.argv.slice(2), {
        options: {
            number: { type: 'boolean', short: 'n', default: false },
            showEnds: { type: 'boolean', short: 'E', default: false },
            showTabs: { type: 'boolean', short: 'T', default: false },
            squeeze: { type: 'boolean', short: 's', default: false },
            color: { type: 'boolean', short: 'c', default: false },
            help: { type: 'boolean', short: 'h', default: false }
        },
        strict: false,
        allowPositionals: true
    });

    // Show help if requested
    if (values.help) {
        console.println("Usage: cat [OPTION]... [FILE]...");
        console.println("Concatenate FILE(s) to standard output.\n");
        console.println("Options:");
        console.println("  -n, --number          number all output lines");
        console.println("  -E, --showEnds        display $ at end of each line");
        console.println("  -T, --showTabs        display TAB characters as ^I");
        console.println("  -s, --squeeze         suppress repeated empty output lines");
        console.println("  -c, --color           enable syntax highlighting");
        console.println("  -h, --help            display this help and exit\n");
        console.println("Syntax highlighting (with -c) is supported for:");
        console.println("  .js, .json, .ndjson, .sql, .csv, .yaml, .yml, .toml\n");
        console.println("Examples:");
        console.println("  cat -c file.js        Display file.js with syntax highlighting");
        console.println("  cat -n data.json      Display data.json with line numbers");
        console.println("  cat -cs file1.txt     Squeeze blank lines with colors");
        process.exit(0);
    }

    // ANSI color codes for syntax highlighting
    const colors = {
        reset: "\x1b[0m",
        keyword: "\x1b[35m",     // magenta
        string: "\x1b[32m",      // green
        number: "\x1b[33m",      // yellow
        comment: "\x1b[90m",     // gray
        operator: "\x1b[36m",    // cyan
        property: "\x1b[94m",    // bright blue
        function: "\x1b[93m",    // bright yellow
        bracket: "\x1b[37m",     // white
        error: "\x1b[31m"        // red
    };

    // JavaScript/JSON keywords
    const jsKeywords = new Set([
        'const', 'let', 'var', 'function', 'return', 'if', 'else', 'for', 'while',
        'do', 'switch', 'case', 'break', 'continue', 'new', 'this', 'typeof',
        'instanceof', 'try', 'catch', 'finally', 'throw', 'async', 'await',
        'class', 'extends', 'import', 'export', 'default', 'from', 'as',
        'true', 'false', 'null', 'undefined', 'void', 'delete', 'in', 'of'
    ]);

    // SQL keywords
    const sqlKeywords = new Set([
        'SELECT', 'FROM', 'WHERE', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'DROP',
        'TABLE', 'DATABASE', 'INDEX', 'VIEW', 'JOIN', 'LEFT', 'RIGHT', 'INNER',
        'OUTER', 'ON', 'AND', 'OR', 'NOT', 'NULL', 'IS', 'AS', 'ORDER', 'BY',
        'GROUP', 'HAVING', 'LIMIT', 'OFFSET', 'UNION', 'ALL', 'DISTINCT',
        'INTO', 'VALUES', 'SET', 'ALTER', 'ADD', 'COLUMN'
    ]);

    // Get file extension
    function getExtension(filename) {
        const parts = filename.split('.');
        return parts.length > 1 ? '.' + parts[parts.length - 1].toLowerCase() : '';
    }

    // Syntax highlighter for JavaScript/JSON
    function highlightJS(line) {
        let result = '';
        let i = 0;

        while (i < line.length) {
            // Skip whitespace
            if (/\s/.test(line[i])) {
                result += line[i];
                i++;
                continue;
            }

            // Comments
            if (line.substring(i, i + 2) === '//') {
                result += colors.comment + line.substring(i) + colors.reset;
                break;
            }
            if (line.substring(i, i + 2) === '/*') {
                const end = line.indexOf('*/', i + 2);
                if (end !== -1) {
                    result += colors.comment + line.substring(i, end + 2) + colors.reset;
                    i = end + 2;
                } else {
                    result += colors.comment + line.substring(i) + colors.reset;
                    break;
                }
                continue;
            }

            // Strings
            if (line[i] === '"' || line[i] === "'" || line[i] === '`') {
                const quote = line[i];
                let j = i + 1;
                while (j < line.length && (line[j] !== quote || line[j - 1] === '\\')) {
                    j++;
                }
                result += colors.string + line.substring(i, j + 1) + colors.reset;
                i = j + 1;
                continue;
            }

            // Numbers
            if (/\d/.test(line[i])) {
                let j = i;
                while (j < line.length && /[\d.xeE+-]/.test(line[j])) {
                    j++;
                }
                result += colors.number + line.substring(i, j) + colors.reset;
                i = j;
                continue;
            }

            // Keywords and identifiers
            if (/[a-zA-Z_$]/.test(line[i])) {
                let j = i;
                while (j < line.length && /[a-zA-Z0-9_$]/.test(line[j])) {
                    j++;
                }
                const word = line.substring(i, j);
                if (jsKeywords.has(word)) {
                    result += colors.keyword + word + colors.reset;
                } else if (j < line.length && line[j] === '(') {
                    result += colors.function + word + colors.reset;
                } else {
                    result += word;
                }
                i = j;
                continue;
            }

            // Operators and brackets
            if ('(){}[]'.includes(line[i])) {
                result += colors.bracket + line[i] + colors.reset;
                i++;
                continue;
            }
            if ('+-*/%=<>!&|^~?:;,.'.includes(line[i])) {
                result += colors.operator + line[i] + colors.reset;
                i++;
                continue;
            }

            // Default
            result += line[i];
            i++;
        }

        return result;
    }

    // Syntax highlighter for JSON
    function highlightJSON(line) {
        let result = '';
        let i = 0;

        while (i < line.length) {
            if (/\s/.test(line[i])) {
                result += line[i];
                i++;
                continue;
            }

            // Strings (property names or values)
            if (line[i] === '"') {
                let j = i + 1;
                while (j < line.length && (line[j] !== '"' || line[j - 1] === '\\')) {
                    j++;
                }
                const str = line.substring(i, j + 1);
                // Check if it's a property name (followed by :)
                let k = j + 1;
                while (k < line.length && /\s/.test(line[k])) k++;
                if (k < line.length && line[k] === ':') {
                    result += colors.property + str + colors.reset;
                } else {
                    result += colors.string + str + colors.reset;
                }
                i = j + 1;
                continue;
            }

            // Numbers
            if (/[\d-]/.test(line[i])) {
                let j = i;
                if (line[i] === '-') j++;
                while (j < line.length && /[\d.eE+-]/.test(line[j])) {
                    j++;
                }
                result += colors.number + line.substring(i, j) + colors.reset;
                i = j;
                continue;
            }

            // Keywords (true, false, null)
            if (line.substring(i, i + 4) === 'true' || line.substring(i, i + 4) === 'null') {
                result += colors.keyword + line.substring(i, i + 4) + colors.reset;
                i += 4;
                continue;
            }
            if (line.substring(i, i + 5) === 'false') {
                result += colors.keyword + line.substring(i, i + 5) + colors.reset;
                i += 5;
                continue;
            }

            // Brackets and operators
            if ('{}[],:'.includes(line[i])) {
                result += colors.bracket + line[i] + colors.reset;
                i++;
                continue;
            }

            result += line[i];
            i++;
        }

        return result;
    }

    // Syntax highlighter for SQL
    function highlightSQL(line) {
        let result = '';
        let i = 0;

        while (i < line.length) {
            if (/\s/.test(line[i])) {
                result += line[i];
                i++;
                continue;
            }

            // Comments
            if (line.substring(i, i + 2) === '--') {
                result += colors.comment + line.substring(i) + colors.reset;
                break;
            }

            // Strings
            if (line[i] === "'" || line[i] === '"') {
                const quote = line[i];
                let j = i + 1;
                while (j < line.length && (line[j] !== quote || line[j - 1] === '\\')) {
                    j++;
                }
                result += colors.string + line.substring(i, j + 1) + colors.reset;
                i = j + 1;
                continue;
            }

            // Numbers
            if (/\d/.test(line[i])) {
                let j = i;
                while (j < line.length && /[\d.]/.test(line[j])) {
                    j++;
                }
                result += colors.number + line.substring(i, j) + colors.reset;
                i = j;
                continue;
            }

            // Keywords
            if (/[a-zA-Z_]/.test(line[i])) {
                let j = i;
                while (j < line.length && /[a-zA-Z0-9_]/.test(line[j])) {
                    j++;
                }
                const word = line.substring(i, j);
                if (sqlKeywords.has(word.toUpperCase())) {
                    result += colors.keyword + word + colors.reset;
                } else {
                    result += word;
                }
                i = j;
                continue;
            }

            // Operators
            if ('()=<>!,;.*+-/'.includes(line[i])) {
                result += colors.operator + line[i] + colors.reset;
                i++;
                continue;
            }

            result += line[i];
            i++;
        }

        return result;
    }

    // Syntax highlighter for CSV
    function highlightCSV(line) {
        const fields = line.split(',');
        return fields.map((field, idx) => {
            const trimmed = field.trim();
            if (trimmed === '') {
                return field;
            } else if (/^".*"$/.test(trimmed) || /^'.*'$/.test(trimmed)) {
                return colors.string + field + colors.reset;
            } else if (/^-?\d+\.?\d*$/.test(trimmed)) {
                return colors.number + field + colors.reset;
            } else {
                return field;
            }
        }).join(colors.operator + ',' + colors.reset);
    }

    // Syntax highlighter for YAML
    function highlightYAML(line) {
        let result = '';
        
        // Comments
        if (line.trimStart().startsWith('#')) {
            return colors.comment + line + colors.reset;
        }

        // Key: value pattern
        const match = line.match(/^(\s*)([^:]+):(.*)/);
        if (match) {
            const [, indent, key, value] = match;
            result = indent + colors.property + key + colors.reset + colors.operator + ':' + colors.reset;
            
            const trimmedValue = value.trim();
            if (trimmedValue.startsWith('"') || trimmedValue.startsWith("'")) {
                result += value.replace(trimmedValue, colors.string + trimmedValue + colors.reset);
            } else if (/^-?\d+\.?\d*$/.test(trimmedValue)) {
                result += value.replace(trimmedValue, colors.number + trimmedValue + colors.reset);
            } else if (['true', 'false', 'null', 'yes', 'no'].includes(trimmedValue.toLowerCase())) {
                result += value.replace(trimmedValue, colors.keyword + trimmedValue + colors.reset);
            } else {
                result += value;
            }
            return result;
        }

        // List items
        if (line.trimStart().startsWith('- ')) {
            return line.replace('-', colors.operator + '-' + colors.reset);
        }

        return line;
    }

    // Syntax highlighter for TOML
    function highlightTOML(line) {
        // Comments
        if (line.trimStart().startsWith('#')) {
            return colors.comment + line + colors.reset;
        }

        // Section headers [section]
        if (line.trim().match(/^\[.*\]$/)) {
            return colors.keyword + line + colors.reset;
        }

        // Key = value pattern
        const match = line.match(/^(\s*)([^=]+)=(.*)/);
        if (match) {
            const [, indent, key, value] = match;
            let result = indent + colors.property + key.trim() + colors.reset + colors.operator + ' = ' + colors.reset;
            
            const trimmedValue = value.trim();
            if (trimmedValue.startsWith('"') || trimmedValue.startsWith("'")) {
                result += value.replace(trimmedValue, colors.string + trimmedValue + colors.reset);
            } else if (/^-?\d+\.?\d*$/.test(trimmedValue)) {
                result += value.replace(trimmedValue, colors.number + trimmedValue + colors.reset);
            } else if (['true', 'false'].includes(trimmedValue.toLowerCase())) {
                result += value.replace(trimmedValue, colors.keyword + trimmedValue + colors.reset);
            } else {
                result += value;
            }
            return result;
        }

        return line;
    }

    // Select highlighter based on file extension
    function getHighlighter(filename) {
        const ext = getExtension(filename);
        
        switch (ext) {
            case '.js':
                return highlightJS;
            case '.json':
            case '.ndjson':
                return highlightJSON;
            case '.sql':
                return highlightSQL;
            case '.csv':
                return highlightCSV;
            case '.yaml':
            case '.yml':
                return highlightYAML;
            case '.toml':
                return highlightTOML;
            default:
                return null; // No highlighting
        }
    }

    // Process a single file
    function catFile(filepath, lineNumber, highlighter) {
        let fullPath = filepath;
        if (!fullPath.startsWith("/")) {
            fullPath = pwd + "/" + filepath;
        }

        try {
            const raw = fs.readFile(fullPath);
            const content = String.fromCharCode(...raw);
            const lines = content.split('\n');
            let prevEmpty = false;
            
            lines.forEach((line, idx) => {
                // Squeeze blank lines if -s flag
                if (values.squeeze) {
                    const isEmpty = line.trim() === '';
                    if (isEmpty && prevEmpty) {
                        return;
                    }
                    prevEmpty = isEmpty;
                }

                let output = line;

                // Apply syntax highlighting
                if (highlighter) {
                    output = highlighter(output);
                }

                // Show tabs as ^I if -T flag
                if (values.showTabs) {
                    output = output.replace(/\t/g, '^I');
                }

                // Show line endings as $ if -E flag
                if (values.showEnds) {
                    output += '$';
                }

                // Add line numbers if -n flag
                if (lineNumber) {
                    console.printf('%6d  %s\n', idx + 1, output);
                } else {
                    console.println(output);
                }
            });
        } catch (e) {
            console.println(colors.error + `cat: ${filepath}: ${e}` + colors.reset);
            process.exit(1);
        }
    }

    // Main execution
    if (positionals.length === 0) {
        console.println(colors.error + "cat: missing file operand" + colors.reset);
        process.exit(1);
    }

    positionals.forEach((file) => {
        const highlighter = values.color ? getHighlighter(file) : null;
        catFile(file, values.number, highlighter);
    });
})()
