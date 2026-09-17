'use strict';
const { command, root } = require('help/config');

const filename = [{ name: 'filename', description: 'VIZSPEC JSON file path', optional: true }];

module.exports = root('viz', 'Render and export VIZSPEC documents', 'Usage: viz <command> [options]', {
    view: command('viz view [options] [filename]', 'Render a VIZSPEC file or stdin as TUI blocks', {
        table: true,
        options: {
            compact: { type: 'boolean', description: 'Hide series summary and raw data tables', default: false },
            rows: { type: 'integer', description: 'Limit detail rows per block', default: 8 },
            verboseMeta: { type: 'boolean', description: 'Show block metadata', default: false },
            width: { type: 'integer', description: 'Width for sparkline, bars, and timelines', default: 40 },
        },
        positionals: filename,
    }),
    lines: command('viz lines [options] [filename]', 'Render a VIZSPEC file or stdin as TUI chart lines', {
        options: {
            height: { type: 'integer', description: 'Chart height for sparkline-style lines', default: 3 },
            width: { type: 'integer', description: 'Width for sparkline and band lines', default: 40 },
            series: { type: 'string', description: 'Series id to render. Defaults to the first compatible series.', default: '' },
            timeformat: { type: 'string', short: 't', description: 'Output time format [rfc3339|ns|us|ms|s]', default: 'rfc3339' },
            tz: { type: 'string', description: 'Output timezone for rendered time values', default: '' },
        },
        positionals: filename,
    }),
    validate: command('viz validate [filename]', 'Validate a VIZSPEC file or stdin', { positionals: filename }),
    export: command('viz export [options] [filename]', 'Export a VIZSPEC file or stdin to SVG or PNG', {
        options: {
            format: { type: 'string', description: 'Export format', default: 'svg' },
            output: { type: 'string', short: 'o', description: 'Output file path', default: '' },
            width: { type: 'integer', description: 'Export width in pixels', default: 0 },
            height: { type: 'integer', description: 'Export height in pixels', default: 0 },
            padding: { type: 'integer', description: 'Export padding in pixels', default: 0 },
            title: { type: 'string', description: 'Optional export title', default: '' },
            background: { type: 'string', description: 'Export background color', default: '' },
            fontFamily: { type: 'string', description: 'SVG font family', default: '' },
            fontSize: { type: 'integer', description: 'Export base font size', default: 0 },
            hideLegend: { type: 'boolean', description: 'Suppress legend rendering', default: false },
            timeformat: { type: 'string', short: 't', description: 'Output time format [rfc3339|ns|us|ms|s]', default: 'rfc3339' },
            tz: { type: 'string', description: 'Output timezone for rendered time values', default: '' },
        },
        positionals: filename,
    }),
});