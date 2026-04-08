(() => {
    'use strict';

    const fs = require('fs');
    const process = require('process');
    const pretty = require('pretty');
    const parseArgs = require('util/parseArgs');
    const vizspec = require('vizspec');

    const optionHelp = { type: 'boolean', short: 'h', description: 'Show help', default: false };

    const defaultConfig = {
        usage: 'Usage: viz <command> [options]',
        options: {
            help: optionHelp,
        },
    };

    const viewConfig = {
        command: 'view',
        usage: 'viz view [options] [filename]',
        description: 'Render a VIZSPEC file or stdin as TUI blocks',
        options: {
            help: optionHelp,
            compact: { type: 'boolean', description: 'Hide series summary and raw data tables', default: false },
            rows: { type: 'integer', description: 'Limit detail rows per block', default: 8 },
            verboseMeta: { type: 'boolean', description: 'Show block metadata', default: false },
            width: { type: 'integer', description: 'Width for sparkline, bars, and timelines', default: 40 },
            ...pretty.TableArgOptions,
        },
        positionals: [
            { name: 'filename', description: 'VIZSPEC JSON file path', optional: true },
        ],
    };

    const linesConfig = {
        command: 'lines',
        usage: 'viz lines [options] [filename]',
        description: 'Render a VIZSPEC file or stdin as TUI chart lines',
        options: {
            help: optionHelp,
            height: { type: 'integer', description: 'Chart height for sparkline-style lines', default: 3 },
            width: { type: 'integer', description: 'Width for sparkline and band lines', default: 40 },
            series: { type: 'string', description: 'Series id to render. Defaults to the first compatible series.', default: '' },
            timeformat: { type: 'string', short: 't', description: 'Output time format [rfc3339|ns|us|ms|s]', default: 'rfc3339' },
            tz: { type: 'string', description: 'Output timezone for rendered time values', default: '' },
        },
        positionals: [
            { name: 'filename', description: 'VIZSPEC JSON file path', optional: true },
        ],
    };

    const validateConfig = {
        command: 'validate',
        usage: 'viz validate [filename]',
        description: 'Validate a VIZSPEC file or stdin',
        options: {
            help: optionHelp,
        },
        positionals: [
            { name: 'filename', description: 'VIZSPEC JSON file path', optional: true },
        ],
    };

    const exportConfig = {
        command: 'export',
        usage: 'viz export [options] [filename]',
        description: 'Export a VIZSPEC file or stdin to SVG or PNG',
        options: {
            help: optionHelp,
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
        positionals: [
            { name: 'filename', description: 'VIZSPEC JSON file path', optional: true },
        ],
    };

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), defaultConfig, viewConfig, linesConfig, validateConfig, exportConfig);
    } catch (err) {
        console.println(err.message);
        printHelp();
        process.exit(1);
    }

    if (parsed.values.help) {
        printHelp(parsed.command);
        process.exit(0);
    }

    if (!parsed.command) {
        printHelp();
        process.exit(1);
    }

    if (parsed.command === 'view') {
        doView(parsed.values, parsed.namedPositionals.filename);
        return;
    }

    if (parsed.command === 'validate') {
        doValidate(parsed.namedPositionals.filename);
        return;
    }

    if (parsed.command === 'lines') {
        doLines(parsed.values, parsed.namedPositionals.filename);
        return;
    }

    if (parsed.command === 'export') {
        doExport(parsed.values, parsed.namedPositionals.filename);
        return;
    }

    console.println(`Unknown command: ${parsed.command}`);
    printHelp();
    process.exit(1);

    function printHelp(command) {
        if (command === 'view') {
            console.println(parseArgs.formatHelp(viewConfig));
            return;
        }
        if (command === 'validate') {
            console.println(parseArgs.formatHelp(validateConfig));
            return;
        }
        if (command === 'lines') {
            console.println(parseArgs.formatHelp(linesConfig));
            return;
        }
        if (command === 'export') {
            console.println(parseArgs.formatHelp(exportConfig));
            return;
        }
        console.println(parseArgs.formatHelp(defaultConfig, viewConfig, linesConfig, validateConfig, exportConfig));
    }

    function doView(config, filename) {
        try {
            validateViewOptions(config);
            const spec = readSpec(filename);
            const blocks = vizspec.toTUIBlocks(spec, {
                compact: config.compact,
                rows: config.rows,
                width: config.width,
                timeformat: resolveOutputTimeformat(config),
                tz: resolveOutputTimezone(config),
            });
            renderBlocks(blocks, config);
        } catch (err) {
            console.println(`Error: ${err.message}`);
            process.exit(1);
        }
    }

    function doLines(config, filename) {
        let spec;
        try {
            validateLinesOptions(config);
            spec = readSpec(filename);
            const lines = vizspec.toTUILines(spec, {
                height: config.height,
                width: config.width,
                seriesId: config.series,
                timeformat: resolveOutputTimeformat(config),
                tz: resolveOutputTimezone(config),
            });
            for (const line of lines) {
                console.println(line);
            }
        } catch (err) {
            console.println(`Error: ${err.message}`);
            const hint = spec ? buildLinesSeriesHint(spec) : '';
            if (hint) {
                console.println(hint);
            }
            process.exit(1);
        }
    }

    function doValidate(filename) {
        try {
            const spec = readSpec(filename);
            vizspec.validate(spec);
            console.println(`VALID version=${spec.version} series=${arrayLength(spec.series)} annotations=${arrayLength(spec.annotations)}`);
        } catch (err) {
            console.println(`INVALID ${err.message}`);
            process.exit(1);
        }
    }

    function doExport(config, filename) {
        try {
            validateExportOptions(config);
            const spec = readSpec(filename);
            const outputPath = config.output ? resolvePath(config.output) : '';
            if (config.format === 'svg') {
                const svg = vizspec.toSVG(spec, buildSVGOptions(config));
                if (outputPath) {
                    fs.writeFileSync(outputPath, `${svg}\n`, 'utf8');
                    console.println(`WROTE ${outputPath}`);
                    return;
                }
                console.println(svg);
                return;
            }
            const png = vizspec.toPNG(spec, buildSVGOptions(config));
            if (!outputPath) {
                throw new Error('png export requires --output because stdout is text-only');
            }
            writeBinaryFile(outputPath, png);
            console.println(`WROTE ${outputPath}`);
        } catch (err) {
            console.println(`Error: ${err.message}`);
            process.exit(1);
        }
    }

    function readSpec(filename) {
        if (!filename) {
            return readSpecStdin();
        }
        return readSpecFile(filename);
    }

    function readSpecFile(filename) {
        const resolved = resolvePath(filename);
        const content = fs.readFile(resolved, { encoding: 'utf8' });
        return vizspec.parse(content);
    }

    function readSpecStdin() {
        if (process.stdin.isTTY()) {
            throw new Error('filename is required unless VIZSPEC JSON is provided on stdin');
        }
        const content = process.stdin.read();
        if (!content || !String(content).trim()) {
            throw new Error('stdin is empty');
        }
        return vizspec.parse(String(content));
    }

    function resolvePath(filePath) {
        return filePath.startsWith('/') ? filePath : `${process.cwd()}/${filePath}`;
    }

    function renderBlocks(blocks, config) {
        for (let i = 0; i < blocks.length; i++) {
            if (i > 0) {
                console.println('');
            }
            renderBlock(blocks[i], config);
        }
    }

    function renderBlock(block, config) {
        if (block.title) {
            console.println(block.title);
            console.println('='.repeat(block.title.length));
        }
        if (Array.isArray(block.stats) && block.stats.length > 0) {
            console.println(renderStatsTable(block.stats, config));
        }
        if (Array.isArray(block.lines) && block.lines.length > 0) {
            for (const line of block.lines) {
                console.println(line);
            }
        }
        if (Array.isArray(block.rows) && block.rows.length > 0) {
            console.println(renderRowsTable(block, config));
        }
        if (config.verboseMeta && block.meta && Object.keys(block.meta).length > 0) {
            console.println(renderMetaTable(block.meta, config));
        }
    }

    function renderStatsTable(stats, config) {
        const box = pretty.Table({ ...tableConfig(config), rownum: false, footer: false });
        box.appendHeader(['NAME', 'VALUE']);
        for (const stat of stats) {
            box.appendRow(box.row(stat.label, stat.value));
        }
        return box.render();
    }

    function renderRowsTable(block, config) {
        const box = pretty.Table({ ...tableConfig(config), footer: false });
        const columns = Array.isArray(block.columns) && block.columns.length > 0 ? block.columns : ['VALUE'];
        box.appendHeader(columns);
        for (const row of block.rows) {
            if (Array.isArray(row)) {
                box.append(row);
            } else {
                box.append([String(row)]);
            }
        }
        return box.render();
    }

    function renderMetaTable(meta, config) {
        const box = pretty.Table({ ...tableConfig(config), rownum: false, footer: false });
        box.appendHeader(['META', 'VALUE']);
        for (const key of Object.keys(meta)) {
            box.appendRow(box.row(key, stringifyValue(meta[key])));
        }
        return box.render();
    }

    function stringifyValue(value) {
        if (value === null || value === undefined) {
            return '';
        }
        if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
            return String(value);
        }
        return JSON.stringify(value);
    }

    function arrayLength(value) {
        return Array.isArray(value) ? value.length : 0;
    }

    function validateViewOptions(config) {
        if (config.rows <= 0) {
            throw new Error('rows must be greater than 0');
        }
        if (config.width <= 0) {
            throw new Error('width must be greater than 0');
        }
    }

    function validateLinesOptions(config) {
        if (config.height <= 0) {
            throw new Error('height must be greater than 0');
        }
        if (config.width <= 0) {
            throw new Error('width must be greater than 0');
        }
    }

    function buildLinesSeriesHint(spec) {
        const listed = vizspec.listSeries(spec).filter(item => item.tuiLinesCompatible);
        if (listed.length === 0) {
            return 'Selectable line series: none';
        }
        return `Selectable line series: ${listed.map(item => item.id || `(index:${item.index})`).join(', ')}`;
    }

    function validateExportOptions(config) {
        if (config.format !== 'svg' && config.format !== 'png') {
            throw new Error(`unsupported export format: ${config.format}`);
        }
        if (config.width < 0) {
            throw new Error('width must be 0 or greater');
        }
        if (config.height < 0) {
            throw new Error('height must be 0 or greater');
        }
        if (config.padding < 0) {
            throw new Error('padding must be 0 or greater');
        }
        if (config.fontSize < 0) {
            throw new Error('font-size must be 0 or greater');
        }
    }

    function writeBinaryFile(filePath, data) {
        fs.writeFileSync(filePath, toByteArray(data), 'buffer');
    }

    function toByteArray(data) {
        if (data instanceof Uint8Array) {
            return Array.from(data);
        }
        if (data instanceof ArrayBuffer) {
            return Array.from(new Uint8Array(data));
        }
        if (Array.isArray(data)) {
            return data;
        }
        throw new Error('binary export produced unsupported data type');
    }

    function buildSVGOptions(config) {
        const ret = {};
        if (config.width > 0) {
            ret.width = config.width;
        }
        if (config.height > 0) {
            ret.height = config.height;
        }
        if (config.padding > 0) {
            ret.padding = config.padding;
        }
        if (config.title) {
            ret.title = config.title;
        }
        if (config.background) {
            ret.background = config.background;
        }
        if (config.fontFamily) {
            ret.fontFamily = config.fontFamily;
        }
        if (config.fontSize > 0) {
            ret.fontSize = config.fontSize;
        }
        if (config.hideLegend) {
            ret.showLegend = false;
        }
        ret.timeformat = resolveOutputTimeformat(config);
        const tz = resolveOutputTimezone(config);
        if (tz) {
            ret.tz = tz;
        }
        return ret;
    }

    function resolveOutputTimeformat(config) {
        if (config.timeformat && config.timeformat !== 'default') {
            return config.timeformat;
        }
        return 'rfc3339';
    }

    function resolveOutputTimezone(config) {
        if (config.tz && config.tz !== 'local') {
            return config.tz;
        }
        return '';
    }

    function tableConfig(config) {
        const ret = {};
        for (const key of Object.keys(pretty.TableArgOptions)) {
            if (config[key] !== undefined) {
                ret[key] = config[key];
            }
        }
        return ret;
    }
})();
