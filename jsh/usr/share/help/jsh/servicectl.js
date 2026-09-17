'use strict';
const { simple } = require('help/config');

module.exports = simple('servicectl', 'Manage services through a service controller', 'Usage: servicectl.js --controller=<host:port|tcp://host:port|unix://path> <command> [args...]', {
    options: {
        controller: { type: 'string', short: 'c', description: 'Controller address in host:port format' },
        name: { type: 'string', short: 'n', description: 'Service name for inline install', default: '' },
        enable: { type: 'boolean', description: 'Enable the service for inline install', default: false },
        workingDir: { type: 'string', short: 'w', description: 'Working directory for inline install', default: '' },
        executable: { type: 'string', short: 'x', description: 'Executable path for inline install', default: '' },
        arg: { type: 'string', short: 'a', description: 'Executable argument for inline install', multiple: true },
        env: { type: 'string', short: 'e', description: 'Environment variable KEY=VALUE for inline install', multiple: true },
        detailType: { type: 'string', description: 'Detail value type for details set: string, number, boolean/bool, object/json', default: '' },
        format: { type: 'string', description: 'Output format for details get: box or json', default: 'box' },
        stripPrefix: { type: 'string', description: 'Public path prefix to strip for proxy register', default: '' },
        healthPath: { type: 'string', description: 'Health check path metadata for proxy register', default: '' },
        timeout: { type: 'integer', short: 't', description: 'RPC timeout in milliseconds', default: 5000 },
    },
    positionals: [
        { name: 'command', description: 'Command to execute', optional: true },
        { name: 'args', description: 'Command arguments', optional: true, variadic: true },
    ],
});

module.exports.content = `Usage: servicectl.js --controller=<host:port|tcp://host:port|unix://path> <command> [args...]

Commands:
  read
  update
  reload
  install <config.json>
  install --name <name> --executable <path> [--arg <arg> ...] [--working-dir <dir>] [--enable] [--env KEY=VALUE ...]
  uninstall <service_name>
  status [service_name]
  start <service_name>
  stop <service_name>
  details get <service_name> [key]
  details set <service_name> <key> <value> [--detail-type <string|number|boolean|bool|object|json>]
  details delete <service_name> <key>
  proxy list [service_name]
  proxy get <service_name> <prefix>
  proxy register <service_name> <prefix> <target> [--strip-prefix <path>] [--health-path <path>]
  proxy unregister <service_name> [prefix]
    controller [metrics|get|reset]
Examples:
    servicectl details get alpha --format json
    servicectl details set alpha retries 3 --detail-type number
    servicectl details set alpha enabled true --detail-type boolean
    servicectl details set alpha labels '{"tier":"gold"}' --detail-type object
    servicectl proxy list github.com/acme/chart
    servicectl proxy register github.com/acme/chart /api/ http://127.0.0.1:18080 --health-path /healthz`;