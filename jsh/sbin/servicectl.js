(() => {
    'use strict';

    const process = require('process');
    const fs = require('fs');
    const path = require('path');
    const pretty = require('pretty');
    const serviceModule = require('service');
    const parseArgs = require('util/parseArgs');
    const statusOutputMaxLines = 20;

    const options = {
        controller: { type: 'string', short: 'c', description: 'Controller address in host:port format', default: serviceModule.resolveController() },
        help: { type: 'boolean', short: 'h', description: 'Show help', default: false },
        name: { type: 'string', short: 'n', description: 'Service name for inline install', default: '' },
        enable: { type: 'boolean', description: 'Enable the service for inline install', default: false },
        workingDir: { type: 'string', short: 'w', description: 'Working directory for inline install', default: '' },
        executable: { type: 'string', short: 'x', description: 'Executable path for inline install', default: '' },
        arg: { type: 'string', short: 'a', description: 'Executable argument for inline install', multiple: true },
        env: { type: 'string', short: 'e', description: 'Environment variable KEY=VALUE for inline install', multiple: true },
        detailType: { type: 'string', description: 'Detail value type for details set: string, number, boolean/bool, object/json', default: '' },
        format: { type: 'string', description: 'Output format for details get: box or json', default: 'box' },
        timeout: { type: 'integer', short: 't', description: 'RPC timeout in milliseconds', default: 5000 },
    };

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), {
            options,
            allowPositionals: true,
            strict: true,
            positionals: [
                { name: 'command', optional: true },
                { name: 'args', variadic: true, optional: true },
            ],
        });
    } catch (err) {
        console.println(err.message);
        printHelp();
        process.exit(1);
    }

    if (parsed.values.help) {
        printHelp();
        process.exit(0);
    }

    const command = parsed.namedPositionals.command || '';
    const args = parsed.namedPositionals.args || [];
    const controller = serviceModule.resolveController(parsed.values.controller);
    const timeout = parsed.values.timeout;

    if (!controller) {
        fail('Option --controller=<host:port|unix://path> is required.');
    }
    if (!command) {
        printHelp();
        process.exit(1);
    }
    if (timeout <= 0) {
        fail(`Invalid timeout '${timeout}'. Use a positive integer.`);
    }

    try {
        serviceModule.parseController(controller);
    } catch (err) {
        fail(err.message || String(err));
    }
    const commandSpec = buildCommandSpec(command, args);
    const rpcOptions = { controller, timeout };

    executeCommand(commandSpec, (err, result) => {
        if (err) {
            fail(commandErrorMessage(commandSpec, err));
            return;
        }
        renderResult(commandSpec, result);
    });

    function printHelp() {
        console.println(parseArgs.formatHelp({
            usage: 'Usage: servicectl.js --controller=<host:port|tcp://host:port|unix://path> <command> [args...]',
            options,
            positionals: [
                { name: 'command', description: 'Command to execute' },
                { name: 'args', description: 'Command arguments', optional: true, variadic: true },
            ],
        }));
        console.println('Commands:');
        console.println('  read');
        console.println('  update');
        console.println('  reload');
        console.println('  install <config.json>');
        console.println('  install --name <name> --executable <path> [--arg <arg> ...] [--working-dir <dir>] [--enable] [--env KEY=VALUE ...]');
        console.println('  uninstall <service_name>');
        console.println('  status [service_name]');
        console.println('  start <service_name>');
        console.println('  stop <service_name>');
        console.println('  details get <service_name> [key]');
        console.println('  details set <service_name> <key> <value> [--detail-type <string|number|boolean|bool|object|json>]');
        console.println('  details delete <service_name> <key>');
        console.println('  controller [metrics|get|reset]');
        console.println('Examples:');
        console.println('  servicectl --controller=127.0.0.1:1234 details get alpha --format json');
        console.println('  servicectl --controller=127.0.0.1:1234 details set alpha retries 3 --detail-type number');
        console.println('  servicectl --controller=127.0.0.1:1234 details set alpha enabled true --detail-type boolean');
        console.println("  servicectl --controller=127.0.0.1:1234 details set alpha labels '{\"tier\":\"gold\"}' --detail-type object");
    }

    function fail(message) {
        console.println(message);
        process.exit(1);
    }

    function buildCommandSpec(cmd, positionalArgs) {
        switch (cmd) {
            case 'read':
                expectArgs(cmd, positionalArgs, 0);
                return clientCommandSpec(cmd, (callback) => serviceModule.read(rpcOptions, callback));
            case 'update':
                expectArgs(cmd, positionalArgs, 0);
                return clientCommandSpec(cmd, (callback) => serviceModule.update(rpcOptions, callback));
            case 'reload':
                expectArgs(cmd, positionalArgs, 0);
                return clientCommandSpec(cmd, (callback) => serviceModule.reload(rpcOptions, callback));
            case 'install':
                return clientCommandSpec(cmd, (callback) => serviceModule.install(buildInstallConfig(positionalArgs), rpcOptions, callback));
            case 'uninstall':
                expectArgs(cmd, positionalArgs, 1);
                return clientCommandSpec(cmd, (callback) => serviceModule.uninstall(positionalArgs[0], rpcOptions, callback), { name: positionalArgs[0] });
            case 'status':
                if (positionalArgs.length === 0) {
                    return clientCommandSpec(cmd, (callback) => serviceModule.status(rpcOptions, callback));
                }
                if (positionalArgs.length === 1) {
                    return clientCommandSpec(cmd, (callback) => serviceModule.status(positionalArgs[0], rpcOptions, callback));
                }
                fail("Command 'status' accepts zero or one argument.");
                return null;
            case 'start':
                expectArgs(cmd, positionalArgs, 1);
                return clientCommandSpec(cmd, (callback) => serviceModule.start(positionalArgs[0], rpcOptions, callback));
            case 'stop':
                expectArgs(cmd, positionalArgs, 1);
                return clientCommandSpec(cmd, (callback) => serviceModule.stop(positionalArgs[0], rpcOptions, callback));
            case 'details':
                return buildDetailsCommandSpec(positionalArgs);
            case 'controller':
                return buildControllerCommandSpec(positionalArgs);
            default:
                fail(`Unknown command '${cmd}'.`);
                return null;
        }
    }

    function clientCommandSpec(commandName, execute, params) {
        return { kind: 'client', command: commandName, execute, params: params || null };
    }

    function buildDetailsCommandSpec(positionalArgs) {
        if (positionalArgs.length === 0) {
            fail("Command 'details' requires a subcommand: get, set, delete.");
        }
        const action = positionalArgs[0];
        switch (action) {
            case 'get':
                if (positionalArgs.length !== 2 && positionalArgs.length !== 3) {
                    fail("Command 'details get' requires <service_name> and optional [key].");
                }
                return {
                    kind: 'client',
                    command: 'details',
                    action,
                    serviceName: positionalArgs[1],
                    key: positionalArgs[2] || '',
                    format: normalizedFormat(parsed.values.format || 'box'),
                    execute: (callback) => {
                        if (positionalArgs[2]) {
                            serviceModule.details.get(positionalArgs[1], positionalArgs[2], rpcOptions, callback);
                            return;
                        }
                        serviceModule.details.get(positionalArgs[1], rpcOptions, callback);
                    },
                };
            case 'set':
                if (positionalArgs.length !== 4) {
                    fail("Command 'details set' requires <service_name> <key> <value>.");
                }
                if (parsed.values.format && parsed.values.format !== 'box') {
                    fail("Option --format is only supported with 'details get'.");
                }
                return {
                    kind: 'client',
                    command: 'details',
                    action,
                    serviceName: positionalArgs[1],
                    key: positionalArgs[2],
                    value: parseDetailValue(positionalArgs[3], parsed.values.detailType || ''),
                    detailType: normalizedDetailType(parsed.values.detailType || ''),
                    execute: (callback) => serviceModule.details.set(positionalArgs[1], positionalArgs[2], parseDetailValue(positionalArgs[3], parsed.values.detailType || ''), rpcOptions, callback),
                };
            case 'delete':
                if (positionalArgs.length !== 3) {
                    fail("Command 'details delete' requires <service_name> <key>.");
                }
                if (parsed.values.format && parsed.values.format !== 'box') {
                    fail("Option --format is only supported with 'details get'.");
                }
                return {
                    kind: 'client',
                    command: 'details',
                    action,
                    serviceName: positionalArgs[1],
                    key: positionalArgs[2],
                    execute: (callback) => serviceModule.details.delete(positionalArgs[1], positionalArgs[2], rpcOptions, callback),
                };
            default:
                fail(`Unknown details command '${action}'.`);
                return null;
        }
    }

    function buildControllerCommandSpec(positionalArgs) {
        const action = positionalArgs[0] || 'metrics';
        if (action === 'metrics' || action === 'get') {
            if (positionalArgs.length > 1) {
                fail("Command 'controller metrics' does not accept additional arguments.");
            }
            return {
                kind: 'client',
                command: 'controller',
                action: 'metrics',
                execute: (callback) => serviceModule.controller.metrics(rpcOptions, callback),
            };
        }
        if (action === 'reset') {
            if (positionalArgs.length > 1) {
                fail("Command 'controller reset' does not accept additional arguments.");
            }
            return {
                kind: 'client',
                command: 'controller',
                action: 'reset',
                execute: (callback) => serviceModule.controller.reset(rpcOptions, callback),
            };
        }
        fail(`Unknown controller command '${action}'. Use metrics|get|reset.`);
        return null;
    }

    function expectArgs(cmd, positionalArgs, expectedCount) {
        if (positionalArgs.length !== expectedCount) {
            if (expectedCount === 0) {
                fail(`Command '${cmd}' does not accept positional arguments.`);
            }
            fail(`Command '${cmd}' requires ${expectedCount} argument(s).`);
        }
    }

    function normalizedDetailType(value) {
        const lowered = String(value || '').toLowerCase();
        if (lowered === '') {
            return 'string';
        }
        if (lowered === 'bool') {
            return 'boolean';
        }
        if (lowered === 'json') {
            return 'object';
        }
        if (lowered === 'string' || lowered === 'number' || lowered === 'boolean' || lowered === 'object') {
            return lowered;
        }
        fail(`Invalid --detail-type '${value}'. Use string, number, boolean, bool, object, or json.`);
        return '';
    }

    function normalizedFormat(value) {
        const lowered = String(value || 'box').toLowerCase();
        if (lowered === 'box' || lowered === 'json') {
            return lowered;
        }
        fail(`Invalid --format '${value}'. Use box or json.`);
        return 'box';
    }

    function parseDetailValue(rawValue, detailType) {
        const normalizedType = normalizedDetailType(detailType);
        switch (normalizedType) {
            case 'string':
                return rawValue;
            case 'number': {
                const value = parseJSONValue(rawValue, 'number');
                if (typeof value !== 'number' || !Number.isFinite(value)) {
                    fail(`Detail value '${rawValue}' is not a valid JSON number.`);
                }
                return value;
            }
            case 'boolean': {
                const value = parseJSONValue(rawValue, 'boolean');
                if (typeof value !== 'boolean') {
                    fail(`Detail value '${rawValue}' is not a valid JSON boolean.`);
                }
                return value;
            }
            case 'object': {
                const value = parseJSONValue(rawValue, 'object');
                if (!value || typeof value !== 'object' || Array.isArray(value)) {
                    fail(`Detail value '${rawValue}' is not a valid JSON object.`);
                }
                return value;
            }
            default:
                fail(`Invalid --detail-type '${detailType}'. Use string, number, boolean, bool, object, or json.`);
                return null;
        }
    }

    function parseJSONValue(rawValue, expectedType) {
        try {
            return JSON.parse(rawValue);
        } catch (err) {
            fail(`Failed to parse ${expectedType} detail value '${rawValue}': ${err.message}`);
            return null;
        }
    }

    function buildInstallConfig(positionalArgs) {
        if (positionalArgs.length > 1) {
            fail("Command 'install' accepts either one config path or inline install options.");
        }
        if (positionalArgs.length === 1) {
            if (parsed.values.name || parsed.values.executable || parsed.values.workingDir || parsed.values.enable || parsed.values.arg || parsed.values.env) {
                fail("Command 'install' cannot mix a config file with inline install options.");
            }
            return readConfigFile(positionalArgs[0]);
        }

        if (!parsed.values.name) {
            fail("Inline install requires --name <service_name>.");
        }
        if (!parsed.values.executable) {
            fail("Inline install requires --executable <path>.");
        }

        return {
            name: parsed.values.name,
            enable: parsed.values.enable,
            working_dir: parsed.values.workingDir,
            environment: parseEnvList(parsed.values.env || []),
            executable: parsed.values.executable,
            args: normalizeList(parsed.values.arg || []),
        };
    }

    function parseEnvList(entries) {
        const env = {};
        for (const entry of entries) {
            const idx = entry.indexOf('=');
            if (idx <= 0) {
                fail(`Invalid --env value '${entry}'. Expected KEY=VALUE.`);
            }
            const key = entry.slice(0, idx);
            const value = entry.slice(idx + 1);
            env[key] = value;
        }
        return env;
    }

    function normalizeList(value) {
        if (Array.isArray(value)) {
            return value;
        }
        if (value === undefined || value === null || value === '') {
            return [];
        }
        return [value];
    }

    function readConfigFile(filePath) {
        const resolved = resolvePath(filePath);
        let raw;
        try {
            raw = fs.readFileSync(resolved, 'utf8');
        } catch (err) {
            fail(`Failed to read config file '${resolved}': ${err.message}`);
        }
        try {
            return JSON.parse(raw);
        } catch (err) {
            fail(`Failed to parse config JSON '${resolved}': ${err.message}`);
        }
    }

    function resolvePath(filePath) {
        if (filePath.startsWith('/')) {
            return filePath;
        }
        return path.resolve(process.cwd(), filePath);
    }

    function executeCommand(commandSpec, callback) {
        if (commandSpec && typeof commandSpec.execute === 'function') {
            commandSpec.execute(callback);
            return;
        }
        callback(new Error(`Unsupported command kind '${commandSpec ? commandSpec.kind : ''}'.`));
    }

    function commandErrorMessage(commandSpec, err) {
        if (isMissingStatusService(commandSpec, err)) {
            return `Service '${commandSpec.params.name}' does not exist.`;
        }
        return err && err.message ? err.message : String(err);
    }

    function isMissingStatusService(commandSpec, err) {
        return Boolean(
            commandSpec &&
            commandSpec.command === 'status' &&
            commandSpec.params &&
            commandSpec.params.name &&
            err &&
            err.rpcCode === -32004
        );
    }

    function renderResult(commandSpec, result) {
        switch (commandSpec.command) {
            case 'read':
                renderReadResult(result);
                return;
            case 'update':
            case 'reload':
                renderUpdateResult(result);
                return;
            case 'status':
                if (Array.isArray(result)) {
                    console.println(`SERVICES (${result.length})`);
                    renderServiceList(result);
                    return;
                }
                renderService(result);
                return;
            case 'start':
            case 'stop':
            case 'install':
                renderOperationResult(commandSpec.command, result);
                return;
            case 'uninstall':
                renderBooleanOperationResult(commandSpec.command, commandSpec.params, result);
                return;
            case 'details':
                renderDetailsResult(commandSpec, result);
                return;
            case 'controller':
                renderControllerResult(commandSpec, result);
                return;
            default:
                printJson(result);
        }
    }

    function renderControllerResult(commandSpec, metrics) {
        if (!metrics || typeof metrics !== 'object') {
            printJson(metrics);
            return;
        }
        const actionLabel = commandSpec.action === 'reset' ? 'controller reset' : 'controller metrics';
        console.println(`RPC METRICS (${actionLabel})`);
        renderTable([
            { key: 'key', title: 'KEY' },
            { key: 'value', title: 'VALUE' },
        ], [
            { key: 'started_at', value: String(metrics.started_at || '-') },
            { key: 'reset_at', value: String(metrics.reset_at || '-') },
            { key: 'connection_limit', value: valueOrDash(metrics.connection_limit) },
            { key: 'active_connections', value: valueOrDash(metrics.active_connections) },
            { key: 'high_water_mark_connections', value: valueOrDash(metrics.high_water_mark_connections) },
            { key: 'accepted_connections', value: valueOrDash(metrics.accepted_connections) },
            { key: 'rejected_connections', value: valueOrDash(metrics.rejected_connections) },
            { key: 'closed_connections', value: valueOrDash(metrics.closed_connections) },
            { key: 'request_count', value: valueOrDash(metrics.request_count) },
            { key: 'notification_count', value: valueOrDash(metrics.notification_count) },
            { key: 'response_count', value: valueOrDash(metrics.response_count) },
            { key: 'response_encode_error_count', value: valueOrDash(metrics.response_encode_error_count) },
        ]);
    }

    function valueOrDash(value) {
        if (value === undefined || value === null) {
            return '-';
        }
        return String(value);
    }

    function renderDetailsResult(commandSpec, runtime) {
        if (!runtime || typeof runtime !== 'object') {
            printJson(runtime);
            return;
        }

        if (commandSpec.action === 'get' && commandSpec.format === 'json') {
            if (commandSpec.key) {
                printJson({ [commandSpec.key]: (runtime.details || {})[commandSpec.key] });
                return;
            }
            printJson(runtime.details || {});
            return;
        }

        const rows = detailRows(runtime.details, commandSpec.key);
        if (commandSpec.action === 'get') {
            console.println(`DETAILS (${rows.length})`);
            renderDetailTable(rows);
            return;
        }

        const currentValue = commandSpec.action === 'delete'
            ? '-'
            : formatDetailValue((runtime.details || {})[commandSpec.key]);
        const currentType = commandSpec.action === 'delete'
            ? '-'
            : detailValueType((runtime.details || {})[commandSpec.key]);
        console.println('RESULT');
        renderTable([
            { key: 'operation', title: 'OPERATION' },
            { key: 'name', title: 'NAME' },
            { key: 'key', title: 'KEY' },
            { key: 'type', title: 'TYPE' },
            { key: 'value', title: 'VALUE' },
            { key: 'success', title: 'SUCCESS' },
        ], [{
            operation: `details ${commandSpec.action}`,
            name: commandSpec.serviceName,
            key: commandSpec.key,
            type: currentType,
            value: currentValue,
            success: 'yes',
        }]);
        console.println('');
        console.println(`DETAILS (${rows.length})`);
        renderDetailTable(rows);
    }

    function detailRows(details, onlyKey) {
        const source = details && typeof details === 'object' ? details : {};
        const keys = Object.keys(source).sort();
        const rows = [];
        for (const key of keys) {
            if (onlyKey && key !== onlyKey) {
                continue;
            }
            rows.push({
                key,
                type: detailValueType(source[key]),
                value: formatDetailValue(source[key]),
            });
        }
        return rows;
    }

    function renderDetailTable(rows) {
        if (!rows || rows.length === 0) {
            console.println('  (none)');
            return;
        }
        renderTable([
            { key: 'key', title: 'KEY' },
            { key: 'type', title: 'TYPE' },
            { key: 'value', title: 'VALUE' },
        ], rows);
    }

    function detailValueType(value) {
        if (value === null) {
            return 'null';
        }
        if (Array.isArray(value)) {
            return 'array';
        }
        return typeof value;
    }

    function formatDetailValue(value) {
        if (value === undefined) {
            return '-';
        }
        if (typeof value === 'string') {
            return value;
        }
        if (typeof value === 'number' || typeof value === 'boolean') {
            return String(value);
        }
        return JSON.stringify(value);
    }

    function renderReadResult(result) {
        if (!result) {
            console.println('No read result');
            return;
        }
        const sections = [
            { title: 'UNCHANGED', configs: result.unchanged || [] },
            { title: 'ADDED', configs: result.added || [] },
            { title: 'UPDATED', configs: result.updated || [] },
            { title: 'REMOVED', configs: result.removed || [] },
            { title: 'ERRORED', configs: result.errored || [] },
        ];

        const rows = [];
        for (const section of sections) {
            const configs = Array.isArray(section.configs) ? section.configs : [];
            for (const cfg of configs) {
                rows.push({
                    status: section.title,
                    name: cfg.name || '',
                    executable: cfg.executable || '',
                    readError: cfg.read_error || '',
                    startError: cfg.start_error || '',
                    stopError: cfg.stop_error || '',
                });
            }
        }

        renderTable([
            { key: 'name', title: 'NAME' },
            { key: 'status', title: 'STATUS' },
            { key: 'executable', title: 'EXECUTABLE' },
            { key: 'readError', title: 'READ_ERROR' },
            { key: 'startError', title: 'START_ERROR' },
            { key: 'stopError', title: 'STOP_ERROR' },
        ], rows.length > 0 ? rows : [{ status: '(none)', name: '', executable: '', readError: '', startError: '', stopError: '' }]);
    }

    function renderUpdateResult(result) {
        if (!result) {
            console.println('No update result');
            return;
        }
        const actions = Array.isArray(result.actions) ? result.actions : [];
        console.println(`ACTIONS (${actions.length})`);
        const rows = actions.length > 0
            ? actions.map((action) => ({
                name: action.name || '',
                action: action.action || '',
                error: action.error || '',
            }))
            : [{ name: '(none)', action: '', error: '' }];
        renderTable([
            { key: 'name', title: 'NAME' },
            { key: 'action', title: 'ACTION' },
            { key: 'error', title: 'ERROR' },
        ], rows);
        console.println('');
        const services = result.services || [];
        console.println(`SERVICES (${Array.isArray(services) ? services.length : 0})`);
        renderServiceList(services);
    }

    function renderServiceList(services) {
        const rows = Array.isArray(services) && services.length > 0
            ? services.map((service) => {
                const cfg = service.config || {};
                return {
                    name: cfg.name || '',
                    enabled: yesNo(cfg.enable),
                    status: service.status || '',
                    pid: service.pid ? String(service.pid) : '-',
                    executable: cfg.executable || '',
                };
            })
            : [{ name: '(none)', enabled: '', status: '', pid: '', executable: '' }];
        const columns = [
            { key: 'name', title: 'NAME' },
            { key: 'enabled', title: 'ENABLED' },
            { key: 'status', title: 'STATUS' },
            { key: 'pid', title: 'PID' },
            { key: 'executable', title: 'EXECUTABLE' },
        ];
        renderTable(columns, rows);
    }

    function renderService(service) {
        if (!service || !service.config) {
            printJson(service);
            return;
        }
        const cfg = service.config;

        const detailRows = [
            { key: 'name', value: cfg.name || '' },
            { key: 'enabled', value: yesNo(cfg.enable) },
            { key: 'status', value: service.status || '' },
            { key: 'exit_code', value: service.exit_code === undefined || service.exit_code === null ? '-' : String(service.exit_code) },
            { key: 'pid', value: service.pid ? String(service.pid) : '-' },
            { key: 'start', value: `${cfg.executable || ''} [ ${Array.isArray(cfg.args) ? cfg.args.join(', ') : ''} ]` },
        ];

        if (cfg.working_dir) {
            detailRows.push({ key: 'cwd', value: cfg.working_dir });
        }
        if (service.error) {
            detailRows.push({ key: 'error', value: service.error });
        }
        if (cfg.read_error) {
            detailRows.push({ key: 'read_error', value: cfg.read_error });
        }
        if (cfg.start_error) {
            detailRows.push({ key: 'start_error', value: cfg.start_error });
        }
        if (cfg.stop_error) {
            detailRows.push({ key: 'stop_error', value: cfg.stop_error });
        }

        console.println('SERVICE');
        renderTable([
            { key: 'key', title: 'KEY' },
            { key: 'value', title: 'VALUE' },
        ], detailRows);

        if (cfg.environment && Object.keys(cfg.environment).length > 0) {
            const envRows = [];
            const keys = Object.keys(cfg.environment).sort();
            for (const key of keys) {
                envRows.push({ key, value: String(cfg.environment[key]) });
            }
            console.println('');
            console.println('ENVIRONMENT');
            renderTable([
                { key: 'key', title: 'KEY' },
                { key: 'value', title: 'VALUE' },
            ], envRows);
        }

        console.println('');
        console.println('OUTPUT');
        const lines = Array.isArray(service.output) ? service.output : [];
        if (lines.length === 0) {
            console.println('  (none)');
            return;
        }

        const startIdx = lines.length > statusOutputMaxLines ? lines.length - statusOutputMaxLines : 0;
        for (let i = startIdx; i < lines.length; i++) {
            console.println(`  ${String(lines[i])}`);
        }
    }

    function renderOperationResult(operation, service) {
        if (!service || !service.config) {
            printJson(service);
            return;
        }
        const cfg = service.config;
        const error = operationError(operation, service);
        const rows = [{
            operation: operation,
            name: cfg.name || '',
            success: yesNo(!error),
            enabled: yesNo(cfg.enable),
            status: service.status || '',
            pid: service.pid ? String(service.pid) : '-',
            exitCode: service.exit_code === undefined || service.exit_code === null ? '-' : String(service.exit_code),
            error: error || '-',
        }];
        console.println('RESULT');
        renderTable([
            { key: 'operation', title: 'OPERATION' },
            { key: 'name', title: 'NAME' },
            { key: 'success', title: 'SUCCESS' },
            { key: 'enabled', title: 'ENABLED' },
            { key: 'status', title: 'STATUS' },
            { key: 'pid', title: 'PID' },
            { key: 'exitCode', title: 'EXIT_CODE' },
            { key: 'error', title: 'ERROR' },
        ], rows);
        console.println('');
        renderService(service);
    }

    function renderBooleanOperationResult(operation, params, result) {
        const name = params && params.name ? params.name : '-';
        const rows = [{
            operation: operation,
            name: name,
            success: yesNo(result),
            enabled: '-',
            status: result ? 'removed' : 'failed',
            pid: '-',
            exitCode: '-',
            error: result ? '-' : 'operation failed',
        }];
        console.println('RESULT');
        renderTable([
            { key: 'operation', title: 'OPERATION' },
            { key: 'name', title: 'NAME' },
            { key: 'success', title: 'SUCCESS' },
            { key: 'enabled', title: 'ENABLED' },
            { key: 'status', title: 'STATUS' },
            { key: 'pid', title: 'PID' },
            { key: 'exitCode', title: 'EXIT_CODE' },
            { key: 'error', title: 'ERROR' },
        ], rows);
    }

    function operationError(operation, service) {
        const cfg = service.config || {};
        if (operation === 'start' && cfg.start_error) {
            return cfg.start_error;
        }
        if (operation === 'stop' && cfg.stop_error) {
            return cfg.stop_error;
        }
        if (service.error) {
            return service.error;
        }
        if (cfg.read_error) {
            return cfg.read_error;
        }
        return '';
    }

    function renderTable(columns, rows) {
        const box = pretty.Table({ rownum: false });
        box.appendHeader(columns.map((column) => column.title));
        for (const row of rows) {
            box.append(columns.map((column) => {
                const value = row[column.key];
                if (value === undefined || value === null) {
                    return '';
                }
                return String(value);
            }));
        }
        console.println(box.render());
    }

    function yesNo(value) {
        return value ? 'yes' : 'no';
    }

    function printJson(value) {
        console.println(JSON.stringify(value, null, 2));
    }
})();
