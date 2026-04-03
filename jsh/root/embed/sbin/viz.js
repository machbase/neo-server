(() => {
    'use strict';

    const fs = require('fs');
    const process = require('process');
    const pretty = require('pretty');
    const parseArgs = require('util/parseArgs');
    const advn = require('mathx/advn');

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
        description: 'Render an ADVN spec file or stdin as TUI blocks',
        options: {
            help: optionHelp,
            compact: { type: 'boolean', description: 'Hide series summary and raw data tables', default: false },
            rows: { type: 'integer', description: 'Limit detail rows per block', default: 8 },
            verboseMeta: { type: 'boolean', description: 'Show block metadata', default: false },
            width: { type: 'integer', description: 'Width for sparklines, bars, and timelines', default: 40 },
            ...pretty.TableArgOptions,
        },
        positionals: [
            { name: 'filename', description: 'ADVN JSON file path', optional: true },
        ],
    };

    const validateConfig = {
        command: 'validate',
        usage: 'viz validate [filename]',
        description: 'Validate an ADVN spec file or stdin',
        options: {
            help: optionHelp,
        },
        positionals: [
            { name: 'filename', description: 'ADVN JSON file path', optional: true },
        ],
    };

    const exportConfig = {
        command: 'export',
        usage: 'viz export [options] [filename]',
        description: 'Export an ADVN spec file or stdin to SVG',
        options: {
            help: optionHelp,
            format: { type: 'string', description: 'Export format', default: 'svg' },
            output: { type: 'string', short: 'o', description: 'Output file path', default: '' },
            width: { type: 'integer', description: 'SVG width in pixels', default: 0 },
            height: { type: 'integer', description: 'SVG height in pixels', default: 0 },
            padding: { type: 'integer', description: 'SVG padding in pixels', default: 0 },
            title: { type: 'string', description: 'Optional SVG title', default: '' },
            background: { type: 'string', description: 'SVG background color', default: '' },
            fontFamily: { type: 'string', description: 'SVG font family', default: '' },
            fontSize: { type: 'integer', description: 'SVG base font size', default: 0 },
            hideLegend: { type: 'boolean', description: 'Suppress legend rendering', default: false },
        },
        positionals: [
            { name: 'filename', description: 'ADVN JSON file path', optional: true },
        ],
    };

    let parsed;
    try {
        parsed = parseArgs(process.argv.slice(2), defaultConfig, viewConfig, validateConfig, exportConfig);
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
        if (command === 'export') {
            console.println(parseArgs.formatHelp(exportConfig));
            return;
        }
        console.println(parseArgs.formatHelp(defaultConfig, viewConfig, validateConfig, exportConfig));
    }

    function doView(config, filename) {
        try {
            validateViewOptions(config);
            const spec = readSpec(filename);
            const blocks = advn.toTUIBlocks(spec, {
                compact: config.compact,
                rows: config.rows,
                width: config.width,
            });
            renderBlocks(blocks, config);
        } catch (err) {
            console.println(`Error: ${err.message}`);
            process.exit(1);
        }
    }

    function doValidate(filename) {
        try {
            const spec = readSpec(filename);
            advn.validate(spec);
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
            const svg = advn.toSVG(spec, buildSVGOptions(config));
            if (config.output) {
                const outputPath = resolvePath(config.output);
                fs.writeFileSync(outputPath, `${svg}\n`, 'utf8');
                console.println(`WROTE ${outputPath}`);
                return;
            }
            console.println(svg);
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
        return advn.parse(content);
    }

    function readSpecStdin() {
        if (process.stdin.isTTY()) {
            throw new Error('filename is required unless ADVN JSON is provided on stdin');
        }
        const content = process.stdin.read();
        if (!content || !String(content).trim()) {
            throw new Error('stdin is empty');
        }
        return advn.parse(String(content));
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

    function validateExportOptions(config) {
        if (config.format !== 'svg') {
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
        return ret;
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