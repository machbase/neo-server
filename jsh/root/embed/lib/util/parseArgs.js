'use strict';

const process = require('/lib/process');

function parseArgs(args, ...configs) {
    if (!Array.isArray(args)) {
        throw new TypeError('First argument must be an array of arguments');
    }

    // Select appropriate config based on sub-command
    let config = null;
    let commandName = null;
    let argsToProcess = args;

    if (configs.length > 1) {
        // Multiple configs provided - check for sub-command matching
        const potentialCommand = args.length > 0 ? args[0] : null;

        for (const cfg of configs) {
            if (cfg.command && cfg.command === potentialCommand) {
                config = cfg;
                commandName = potentialCommand;
                argsToProcess = args.slice(1); // Skip the command itself
                break;
            }
        }

        // If no command matched, use the first config without a command property
        if (!config) {
            config = configs.find(cfg => !cfg.command) || configs[0];
        }
    } else {
        // Single or no config provided
        config = configs[0] || {};
    }

    config = config || {};
    const options = config.options || {};
    const strict = config.strict !== undefined ? config.strict : true;
    const allowNegative = config.allowNegative || false;
    const tokens = config.tokens || false;
    const positionalsConfig = config.positionals;
    const allowPositionals = config.allowPositionals !== undefined
        ? config.allowPositionals
        : (config.positionals && config.positionals.length > 0);

    // Normalize positionals config
    let positionalDefs = null;
    if (positionalsConfig) {
        positionalDefs = positionalsConfig.map(def => {
            if (typeof def === 'string') {
                return { name: def, optional: false };
            }
            return {
                name: def.name,
                optional: def.optional || false,
                default: def.default,
                variadic: def.variadic || false
            };
        });

        // Validate: variadic must be last
        const variadicIndex = positionalDefs.findIndex(def => def.variadic);
        if (variadicIndex !== -1 && variadicIndex !== positionalDefs.length - 1) {
            throw new TypeError('Variadic positional argument must be the last argument');
        }
    }

    const result = {
        values: {},
        positionals: []
    };

    if (commandName !== null) {
        result.command = commandName;
    }

    if (tokens) {
        result.tokens = [];
    }

    if (positionalDefs) {
        result.namedPositionals = {};
    }

    // Apply default values from options
    for (const [name, optionConfig] of Object.entries(options)) {
        if ('default' in optionConfig) {
            result.values[name] = optionConfig.default;
        }
    }

    // Convert camelCase to kebab-case for CLI flags
    function toKebabCase(str) {
        return str.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`);
    }

    // Convert kebab-case to camelCase for positional argument names
    function toCamelCase(str) {
        return str.replace(/-([a-z])/g, (match, letter) => letter.toUpperCase());
    }

    // Build option maps for quick lookup
    const longOptions = new Map();
    const shortOptions = new Map();

    for (const [name, optionConfig] of Object.entries(options)) {
        const kebabName = toKebabCase(name);
        longOptions.set(kebabName, { name, ...optionConfig });
        if (optionConfig.short) {
            shortOptions.set(optionConfig.short, { name, ...optionConfig });
        }
    }

    let index = 0;
    let foundOptionTerminator = false;

    while (index < argsToProcess.length) {
        const arg = argsToProcess[index];

        // Option terminator '--'
        if (arg === '--') {
            if (tokens) {
                result.tokens.push({
                    kind: 'option-terminator',
                    index
                });
            }
            foundOptionTerminator = true;
            index++;

            // All remaining args are positionals
            while (index < argsToProcess.length) {
                if (!allowPositionals && strict) {
                    throw new TypeError(`Unexpected positional argument: ${argsToProcess[index]}`);
                }
                result.positionals.push(argsToProcess[index]);
                if (tokens) {
                    result.tokens.push({
                        kind: 'positional',
                        index,
                        value: argsToProcess[index]
                    });
                }
                index++;
            }
            break;
        }

        // Long option (--foo or --foo=bar)
        if (arg.startsWith('--')) {
            let optionName = arg.slice(2);
            let optionValue = undefined;
            let inlineValue = false;

            // Check for --foo=bar format
            const equalsIndex = optionName.indexOf('=');
            if (equalsIndex !== -1) {
                optionValue = optionName.slice(equalsIndex + 1);
                optionName = optionName.slice(0, equalsIndex);
                inlineValue = true;
            }

            // Handle negative options (--no-foo)
            let isNegative = false;
            let actualOptionName = optionName;

            if (allowNegative && optionName.startsWith('no-')) {
                const positiveForm = optionName.slice(3);
                const positiveOption = longOptions.get(positiveForm);

                if (positiveOption && positiveOption.type === 'boolean') {
                    isNegative = true;
                    actualOptionName = positiveForm;
                    optionName = positiveForm;
                }
            }

            const option = longOptions.get(actualOptionName);

            if (!option) {
                if (strict) {
                    throw new TypeError(`Unknown option: --${optionName}`);
                }
                index++;
                continue;
            }

            if (option.type === 'string') {
                if (isNegative && strict) {
                    throw new TypeError(`Option --no-${optionName} cannot be used with type 'string'`);
                }

                // Get value from inline or next arg
                if (optionValue === undefined) {
                    index++;
                    if (index >= argsToProcess.length) {
                        throw new TypeError(`Option --${optionName} requires a value`);
                    }
                    optionValue = argsToProcess[index];
                }

                if (option.multiple) {
                    if (!Array.isArray(result.values[option.name])) {
                        result.values[option.name] = [];
                    }
                    result.values[option.name].push(optionValue);
                } else {
                    result.values[option.name] = optionValue;
                }

                if (tokens) {
                    result.tokens.push({
                        kind: 'option',
                        name: option.name,
                        rawName: `--${actualOptionName}`,
                        index: inlineValue ? index : index - 1,
                        value: optionValue,
                        inlineValue
                    });
                }
            } else if (option.type === 'integer' || option.type === 'float') {
                if (isNegative && strict) {
                    throw new TypeError(`Option --no-${optionName} cannot be used with type '${option.type}'`);
                }

                // Get value from inline or next arg
                if (optionValue === undefined) {
                    index++;
                    if (index >= argsToProcess.length) {
                        throw new TypeError(`Option --${optionName} requires a value`);
                    }
                    optionValue = argsToProcess[index];
                }

                // Parse and validate number
                const numValue = option.type === 'integer' ? parseInt(optionValue, 10) : parseFloat(optionValue);

                if (isNaN(numValue)) {
                    throw new TypeError(`Option --${optionName} requires a valid ${option.type} value, got: ${optionValue}`);
                }

                // For integer, check if the string contains a decimal point
                if (option.type === 'integer' && optionValue.includes('.')) {
                    throw new TypeError(`Option --${optionName} requires an integer value, got: ${optionValue}`);
                }

                if (option.multiple) {
                    if (!Array.isArray(result.values[option.name])) {
                        result.values[option.name] = [];
                    }
                    result.values[option.name].push(numValue);
                } else {
                    result.values[option.name] = numValue;
                }

                if (tokens) {
                    result.tokens.push({
                        kind: 'option',
                        name: option.name,
                        rawName: `--${actualOptionName}`,
                        index: inlineValue ? index : index - 1,
                        value: numValue,
                        inlineValue
                    });
                }
            } else if (option.type === 'boolean') {
                const boolValue = !isNegative;

                if (inlineValue && strict) {
                    throw new TypeError(`Option --${optionName} does not take a value`);
                }

                if (option.multiple) {
                    if (!Array.isArray(result.values[option.name])) {
                        result.values[option.name] = [];
                    }
                    result.values[option.name].push(boolValue);
                } else {
                    result.values[option.name] = boolValue;
                }

                if (tokens) {
                    result.tokens.push({
                        kind: 'option',
                        name: option.name,
                        rawName: isNegative ? `--no-${actualOptionName}` : `--${actualOptionName}`,
                        index,
                        value: undefined,
                        inlineValue: undefined
                    });
                }
            }

            index++;
            continue;
        }

        // Short option (-f or -abc or -f=bar)
        if (arg.startsWith('-') && arg.length > 1 && arg !== '-') {
            let shortOpts = arg.slice(1);
            let inlineValue = false;
            let optionValue = undefined;

            // Check for -f=bar format
            const equalsIndex = shortOpts.indexOf('=');
            if (equalsIndex !== -1) {
                optionValue = shortOpts.slice(equalsIndex + 1);
                shortOpts = shortOpts.slice(0, equalsIndex);
                inlineValue = true;
            }

            // Process each short option character
            for (let i = 0; i < shortOpts.length; i++) {
                const shortOpt = shortOpts[i];
                const option = shortOptions.get(shortOpt);

                if (!option) {
                    if (strict) {
                        throw new TypeError(`Unknown option: -${shortOpt}`);
                    }
                    continue;
                }

                if (option.type === 'string') {
                    // Get value from inline, remainder, or next arg
                    if (optionValue !== undefined) {
                        // From -f=bar
                    } else if (i < shortOpts.length - 1) {
                        // Remaining chars are the value
                        optionValue = shortOpts.slice(i + 1);
                        inlineValue = true;
                    } else {
                        // Get from next arg
                        index++;
                        if (index >= argsToProcess.length) {
                            throw new TypeError(`Option -${shortOpt} requires a value`);
                        }
                        optionValue = argsToProcess[index];
                    }

                    if (option.multiple) {
                        if (!Array.isArray(result.values[option.name])) {
                            result.values[option.name] = [];
                        }
                        result.values[option.name].push(optionValue);
                    } else {
                        result.values[option.name] = optionValue;
                    }

                    if (tokens) {
                        result.tokens.push({
                            kind: 'option',
                            name: option.name,
                            rawName: `-${shortOpt}`,
                            index: inlineValue ? index : (optionValue !== shortOpts.slice(i + 1) ? index : index - 1),
                            value: optionValue,
                            inlineValue
                        });
                    }

                    // Value consumed, break out of char loop
                    break;
                } else if (option.type === 'integer' || option.type === 'float') {
                    // Get value from inline, remainder, or next arg
                    if (optionValue !== undefined) {
                        // From -p=123
                    } else if (i < shortOpts.length - 1) {
                        // Remaining chars are the value
                        optionValue = shortOpts.slice(i + 1);
                        inlineValue = true;
                    } else {
                        // Get from next arg
                        index++;
                        if (index >= argsToProcess.length) {
                            throw new TypeError(`Option -${shortOpt} requires a value`);
                        }
                        optionValue = argsToProcess[index];
                    }

                    // Parse and validate number
                    const numValue = option.type === 'integer' ? parseInt(optionValue, 10) : parseFloat(optionValue);

                    if (isNaN(numValue)) {
                        throw new TypeError(`Option -${shortOpt} requires a valid ${option.type} value, got: ${optionValue}`);
                    }

                    // For integer, check if the string contains a decimal point
                    if (option.type === 'integer' && optionValue.includes('.')) {
                        throw new TypeError(`Option -${shortOpt} requires an integer value, got: ${optionValue}`);
                    }

                    if (option.multiple) {
                        if (!Array.isArray(result.values[option.name])) {
                            result.values[option.name] = [];
                        }
                        result.values[option.name].push(numValue);
                    } else {
                        result.values[option.name] = numValue;
                    }

                    if (tokens) {
                        result.tokens.push({
                            kind: 'option',
                            name: option.name,
                            rawName: `-${shortOpt}`,
                            index: inlineValue ? index : (optionValue !== shortOpts.slice(i + 1) ? index : index - 1),
                            value: numValue,
                            inlineValue
                        });
                    }

                    // Value consumed, break out of char loop
                    break;
                } else if (option.type === 'boolean') {
                    if (option.multiple) {
                        if (!Array.isArray(result.values[option.name])) {
                            result.values[option.name] = [];
                        }
                        result.values[option.name].push(true);
                    } else {
                        result.values[option.name] = true;
                    }

                    if (tokens) {
                        result.tokens.push({
                            kind: 'option',
                            name: option.name,
                            rawName: `-${shortOpt}`,
                            index,
                            value: undefined,
                            inlineValue: undefined
                        });
                    }
                }
            }

            index++;
            continue;
        }

        // Positional argument
        if (!allowPositionals) {
            if (strict) {
                throw new TypeError(`Unexpected positional argument: ${arg}`);
            }
            // In non-strict mode, skip unknown positionals
            index++;
            continue;
        }

        result.positionals.push(arg);
        if (tokens) {
            result.tokens.push({
                kind: 'positional',
                index,
                value: arg
            });
        }

        index++;
    }

    // Process named positionals if configured
    if (positionalDefs && result.positionals.length > 0) {
        let posIndex = 0;

        for (let i = 0; i < positionalDefs.length; i++) {
            const def = positionalDefs[i];

            if (def.variadic) {
                // Collect all remaining positionals
                const variadicValues = [];
                while (posIndex < result.positionals.length) {
                    variadicValues.push(result.positionals[posIndex]);
                    posIndex++;
                }
                result.namedPositionals[toCamelCase(def.name)] = variadicValues;
            } else {
                if (posIndex < result.positionals.length) {
                    result.namedPositionals[toCamelCase(def.name)] = result.positionals[posIndex];
                    posIndex++;
                } else if (!def.optional) {
                    throw new TypeError(`Missing required argument: ${def.name}`);
                } else if ('default' in def) {
                    result.namedPositionals[toCamelCase(def.name)] = def.default;
                } else {
                    result.namedPositionals[toCamelCase(def.name)] = undefined;
                }
            }
        }
    } else if (positionalDefs) {
        // No positionals provided, apply defaults
        for (const def of positionalDefs) {
            if (!def.optional) {
                throw new TypeError(`Missing required positional argument: ${def.name}`);
            } else if (def.variadic) {
                result.namedPositionals[toCamelCase(def.name)] = [];
            } else if ('default' in def) {
                result.namedPositionals[toCamelCase(def.name)] = def.default;
            } else {
                result.namedPositionals[toCamelCase(def.name)] = undefined;
            }
        }
    }

    return result;
}

// Helper function to convert camelCase to kebab-case
function toKebabCase(str) {
    return str.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`);
}

// Format help message for options
function formatHelp(...configs) {
    // If multiple configs with commands, show all sub-commands
    const commandConfigs = configs.filter(cfg => cfg && cfg.command);
    const defaultConfig = configs.find(cfg => cfg && !cfg.command);

    if (commandConfigs.length > 0) {
        // Multi-command help
        const lines = [];
        if (commandConfigs.length > 1) {
            const usage = (defaultConfig && defaultConfig.usage) || 'Usage: <command> [options]';
            lines.push(usage);
            lines.push('');
            lines.push('Commands:');

            // Calculate max command name width for alignment
            let maxCommandWidth = 0;
            for (const cfg of commandConfigs) {
                maxCommandWidth = Math.max(maxCommandWidth, cfg.command.length);
            }

            // List all commands with descriptions
            for (const cfg of commandConfigs) {
                const padding = ' '.repeat(maxCommandWidth - cfg.command.length);
                const desc = cfg.description || '';
                lines.push(`  ${cfg.command}${padding}${desc ? '  ' + desc : ''}`);
            }

            // Show global options if default config exists
            if (defaultConfig && defaultConfig.options && Object.keys(defaultConfig.options).length > 0) {
                lines.push('');
                lines.push('Global options:');
                lines.push(formatOptionsHelp(defaultConfig.options, defaultConfig.allowNegative));
            }
        }
        // Show detailed help for each command
        for (const cfg of commandConfigs) {
            lines.push('');
            if (cfg.usage) {
                lines.push(`${cfg.usage}`);
            } else {
                lines.push(`${cfg.command} [options]`);
            }
            if (cfg.description) {
                lines.push(`  ${cfg.description}`);
            }

            // Positionals
            if (cfg.positionals && cfg.positionals.length > 0) {
                lines.push('');
                lines.push('  Arguments:');

                // Calculate max name width for alignment
                let maxNameWidth = 0;
                for (const pos of cfg.positionals) {
                    if (typeof pos === 'string') {
                        maxNameWidth = Math.max(maxNameWidth, pos.length);
                    } else {
                        const variadic = pos.variadic ? '...' : '';
                        const nameWidth = pos.name.length + variadic.length;
                        maxNameWidth = Math.max(maxNameWidth, nameWidth);
                    }
                }

                for (const pos of cfg.positionals) {
                    if (typeof pos === 'string') {
                        lines.push(`    ${pos}`);
                    } else {
                        const required = pos.optional ? ' (optional)' : '';
                        const variadic = pos.variadic ? '...' : '';
                        const nameText = `${pos.name}${variadic}`;
                        const padding = ' '.repeat(maxNameWidth - nameText.length);
                        const defaultVal = pos.default !== undefined ? ` (default: ${pos.default})` : '';
                        const desc = pos.description ? pos.description : '';
                        lines.push(`    ${nameText}${padding}${required}  ${desc}${defaultVal}`);
                    }
                }
            }

            // Options
            if (cfg.options && Object.keys(cfg.options).length > 0) {
                lines.push('');
                lines.push('  Options:');
                const optionsHelp = formatOptionsHelp(cfg.options, cfg.allowNegative, '    ');
                lines.push(optionsHelp);
            }

            // Long description
            if (cfg.longDescription) {
                lines.push(cfg.longDescription);
            }
        }

        return lines.join('\n');
    } else {
        // Single config or no commands - original behavior
        const config = configs[0] || {};
        return formatSingleConfigHelp(config);
    }
}

// Helper to format options help
function formatOptionsHelp(options, allowNegative, indent = '  ') {
    allowNegative = allowNegative === undefined ? true : allowNegative;
    const lines = [];

    // First pass: calculate max width of option keys
    let maxKeyWidth = 0;
    for (const [key, opt] of Object.entries(options)) {
        const short = opt.short ? `-${opt.short}, ` : '    ';
        const kebabKey = toKebabCase(key);
        const isBooleanWithNegative = opt.type === 'boolean' && allowNegative && key !== 'help';
        const longFlag = isBooleanWithNegative ? `--[no-]${kebabKey}` : `--${kebabKey}`;
        const keyText = `${short}${longFlag}`;
        maxKeyWidth = Math.max(maxKeyWidth, keyText.length);
    }

    // Second pass: format with padding
    for (const [key, opt] of Object.entries(options)) {
        const short = opt.short ? `-${opt.short}, ` : '    ';
        const kebabKey = toKebabCase(key);
        const isBooleanWithNegative = opt.type === 'boolean' && allowNegative && key !== 'help';
        const longFlag = isBooleanWithNegative ? `--[no-]${kebabKey}` : `--${kebabKey}`;
        const keyText = `${short}${longFlag}`;
        const padding = ' '.repeat(maxKeyWidth - keyText.length);
        const desc = opt.description || '';
        const defaultVal = opt.default !== undefined && key !== 'help' ? ` (default: ${opt.default})` : '';

        lines.push(`${indent}${keyText}${padding}${desc ? '  ' + desc : ''}${defaultVal}`);
    }

    return lines.join('\n');
}

// Helper to format single config help (original behavior)
function formatSingleConfigHelp(config) {
    const options = config.options || {};
    const usage = config.usage || 'Usage: [options]';
    const positionals = config.positionals || [];
    const allowNegative = config.allowNegative === undefined ? true : config.allowNegative;

    const lines = [];
    lines.push(usage);

    // Format positionals if present
    if (positionals.length > 0) {
        lines.push('');
        lines.push('Arguments:');

        // Calculate max name width for alignment
        let maxNameWidth = 0;
        for (const pos of positionals) {
            if (typeof pos === 'string') {
                maxNameWidth = Math.max(maxNameWidth, pos.length);
            } else {
                const variadic = pos.variadic ? '...' : '';
                const nameWidth = pos.name.length + variadic.length;
                maxNameWidth = Math.max(maxNameWidth, nameWidth);
            }
        }

        for (const pos of positionals) {
            if (typeof pos === 'string') {
                lines.push(`  ${pos}`);
            } else {
                const required = pos.optional ? ' (optional)' : '';
                const variadic = pos.variadic ? '...' : '';
                const nameText = `${pos.name}${variadic}`;
                const padding = ' '.repeat(maxNameWidth - nameText.length);
                const defaultVal = pos.default !== undefined ? ` (default: ${pos.default})` : '';
                const desc = pos.description ? ` - ${pos.description}` : '';
                lines.push(`  ${nameText}${padding}${required}${desc}${defaultVal}`);
            }
        }
    }

    // Format options
    if (Object.keys(options).length > 0) {
        lines.push('');
        lines.push('Options:');
        lines.push(formatOptionsHelp(options, allowNegative));
    }

    return lines.join('\n');
}

parseArgs.formatHelp = formatHelp;
parseArgs.toKebabCase = toKebabCase;

module.exports = parseArgs;