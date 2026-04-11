'use strict';

const fs = require('fs');
const process = require('process');
const path = require('path');
const parseArgs = require('util/parseArgs');
const { ai } = require('@jsh/shell');
const { buildSystemPrompt } = require('ai/prompt');
const { extractCodeBlocks, executeBlock, collectEditStats } = require('ai/executor');

(() => {
    const options = {
        scenarios: {
            type: 'string',
            short: 's',
            description: 'Scenario file path (JSONL or plain text lines)',
        },
        out: {
            type: 'string',
            short: 'o',
            description: 'Output report path (default: ai-kpi-report.json)',
            default: 'ai-kpi-report.json',
        },
        outNdjson: {
            type: 'string',
            description: 'Optional NDJSON output path for per-scenario entries',
        },
        outCsv: {
            type: 'string',
            description: 'Optional CSV output path for per-scenario entries',
        },
        provider: {
            type: 'string',
            short: 'p',
            description: 'LLM provider override',
        },
        model: {
            type: 'string',
            short: 'm',
            description: 'LLM model override',
        },
        timeout: {
            type: 'string',
            description: 'Execution timeout in ms (default: 30000)',
        },
        maxRows: {
            type: 'string',
            description: 'Max query rows (default: 1000)',
        },
        maxOutputBytes: {
            type: 'string',
            description: 'Max execution output bytes (default: 65536)',
        },
        noExec: {
            type: 'boolean',
            description: 'Disable runnable block execution',
            default: false,
        },
        dryRun: {
            type: 'boolean',
            description: 'Do not call provider; only parse and report scenario metadata',
            default: false,
        },
        help: {
            type: 'boolean',
            short: 'h',
            description: 'Show this help message',
            default: false,
        },
    };

    let values = {};
    let positionals = [];
    let parseError = null;
    try {
        const parsed = parseArgs(process.argv.slice(2), { options, allowPositionals: true });
        values = parsed.values;
        positionals = parsed.positionals || [];
    } catch (err) {
        parseError = err;
    }

    if (parseError || values.help) {
        if (parseError) {
            console.println('Error:', parseError.message);
        }
        console.println(parseArgs.formatHelp({
            usage: 'Usage: ai_kpi [options]',
            description: 'Run scenario batches and emit KPI report for AI harness evaluation.',
            options: options,
        }));
        console.println('');
        console.println('Scenario file format:');
        console.println('  - JSON line: {"id":"s1","prompt":"..."}');
        console.println('  - Plain line: prompt text');
        process.exit(parseError ? 1 : 0);
    }

    if (values.provider) {
        ai.setProvider(values.provider);
    }
    if (values.model) {
        ai.setModel(values.model);
    }

    const scenariosPathInput = values.scenarios || positionals[0];
    if (!scenariosPathInput) {
        console.println('Error: scenario file path is required. Use --scenarios <path>.');
        process.exit(1);
    }
    const scenariosPath = resolveUserPath(scenariosPathInput);
    const outPath = resolveUserPath(values.out || 'ai-kpi-report.json');
    const outNdjsonPath = values.outNdjson ? resolveUserPath(values.outNdjson) : '';
    const outCsvPath = values.outCsv ? resolveUserPath(values.outCsv) : '';

    const cfg = ai.config.load();
    const activeSegments = (cfg.prompt && cfg.prompt.segments)
        ? cfg.prompt.segments.slice()
        : ['jsh-runtime', 'jsh-modules', 'agent-api', 'machbase-sql'];
    const timeoutMs = toInt(values.timeout, 30000);
    const maxRows = toInt(values.maxRows, 1000);
    const maxOutputBytes = toInt(values.maxOutputBytes, 65536);

    const scenarios = loadScenarios(scenariosPath);
    if (scenarios.length === 0) {
        console.println('Error: no scenarios loaded from ' + scenariosPath);
        process.exit(1);
    }

    if (scenarios.length < 20) {
        console.println('[warn] loaded scenarios: ' + scenarios.length + ' (recommended baseline: 20)');
    }

    const providerInfo = ai.providerInfo();
    const startedAt = Date.now();
    const entries = [];

    for (let i = 0; i < scenarios.length; i++) {
        const scenario = scenarios[i];
        const entry = runScenario(i, scenario, {
            provider: providerInfo,
            systemPrompt: buildSystemPrompt(activeSegments),
            noExec: !!values.noExec,
            dryRun: !!values.dryRun,
            timeoutMs: timeoutMs,
            maxRows: maxRows,
            maxOutputBytes: maxOutputBytes,
        });
        entries.push(entry);
    }

    const report = buildReport({
        generatedAt: new Date().toISOString(),
        elapsedMs: Date.now() - startedAt,
        provider: providerInfo.name || 'unknown',
        model: providerInfo.model || 'unknown',
        scenariosPath: scenariosPath,
        outPath: outPath,
        outNdjsonPath: outNdjsonPath,
        outCsvPath: outCsvPath,
        options: {
            noExec: !!values.noExec,
            dryRun: !!values.dryRun,
            timeoutMs: timeoutMs,
            maxRows: maxRows,
            maxOutputBytes: maxOutputBytes,
        },
        entries: entries,
    });

    fs.writeFileSync(outPath, JSON.stringify(report, null, 2), 'utf8');
    if (outNdjsonPath) {
        writeNdjson(outNdjsonPath, report.scenarios || []);
    }
    if (outCsvPath) {
        writeCsv(outCsvPath, report.scenarios || []);
    }

    console.println('AI KPI Report');
    console.println('  Scenarios: ' + report.totals.scenarioCount);
    console.println('  Success:   ' + report.totals.successCount);
    console.println('  Failures:  ' + report.totals.failureCount);
    console.println('  Tokens:    ' + report.totals.inputTokens + ' in / ' + report.totals.outputTokens + ' out');
    console.println('  ExecOps:   ' + report.totals.execOps + ' (run=' + report.totals.runOps + ' create=' + report.totals.createOps + ' patch=' + report.totals.patchOps + ')');
    console.println('  Denied:    ' + report.totals.policyDeniedCount);
    console.println('  Avg(ms):   latency=' + report.totals.avgLatencyMs + ', ttfb=' + report.totals.avgTTFBMs + ', exec=' + report.totals.avgExecElapsedMs);
    console.println('  Report:    ' + outPath);
    if (outNdjsonPath) {
        console.println('  NDJSON:    ' + outNdjsonPath);
    }
    if (outCsvPath) {
        console.println('  CSV:       ' + outCsvPath);
    }
})();

function resolveUserPath(inputPath) {
    const raw = String(inputPath || '').trim();
    if (!raw) {
        return '/work/ai-kpi-report.json';
    }
    if (raw.charAt(0) === '/') {
        return raw;
    }
    const envPwd = process && process.env && typeof process.env.get === 'function'
        ? String(process.env.get('PWD') || '')
        : '';
    const base = envPwd || process.cwd();
    return path.resolve(base, raw);
}

function toInt(raw, defaultValue) {
    if (raw === undefined || raw === null || raw === '') {
        return defaultValue;
    }
    const n = Number(raw);
    if (!Number.isFinite(n) || n <= 0) {
        return defaultValue;
    }
    return Math.floor(n);
}

function loadScenarios(path) {
    const raw = fs.readFileSync(path, 'utf8');
    const lines = String(raw || '').split(/\r?\n/);
    const out = [];
    let lineNo = 0;
    for (let i = 0; i < lines.length; i++) {
        lineNo = i + 1;
        const line = String(lines[i] || '').trim();
        if (!line || line.charAt(0) === '#') {
            continue;
        }
        if (line.charAt(0) === '{') {
            const obj = JSON.parse(line);
            const prompt = String(obj.prompt || '').trim();
            if (!prompt) {
                throw new Error('invalid scenario JSON at line ' + lineNo + ': prompt is required');
            }
            out.push({ id: String(obj.id || ('scenario-' + out.length)), prompt: prompt });
            continue;
        }
        out.push({ id: 'scenario-' + out.length, prompt: line });
    }
    return out;
}

function runScenario(index, scenario, ctx) {
    if (ctx.dryRun) {
        return {
            index: index,
            id: scenario.id,
            ok: true,
            dryRun: true,
            promptChars: scenario.prompt.length,
            latencyMs: 0,
            ttfbMs: 0,
            inputTokens: 0,
            outputTokens: 0,
            retryCount: 0,
            exec: {
                executedBlocks: 0,
                elapsedMs: 0,
                policyDenied: 0,
                editStats: {
                    totalOps: 0,
                    runOps: 0,
                    createOps: 0,
                    patchOps: 0,
                    byLang: {},
                },
            },
        };
    }

    let responseContent = '';
    let streamErr = null;
    let finalResp = null;
    let firstTokenAt = 0;
    const startedAt = Date.now();

    try {
        ai.stream([{ role: 'user', content: scenario.prompt }], ctx.systemPrompt, {
            data: function (token) {
                if (!firstTokenAt) {
                    firstTokenAt = Date.now();
                }
                responseContent += token;
            },
            end: function (resp) {
                finalResp = resp;
            },
            error: function (err) {
                streamErr = err;
            },
        });
    } catch (err) {
        streamErr = err;
    }

    const latencyMs = Date.now() - startedAt;
    const ttfbMs = firstTokenAt ? (firstTokenAt - startedAt) : 0;

    if (streamErr) {
        return {
            index: index,
            id: scenario.id,
            ok: false,
            error: String(streamErr),
            latencyMs: latencyMs,
            ttfbMs: ttfbMs,
            inputTokens: Number(finalResp && finalResp.inputTokens || 0),
            outputTokens: Number(finalResp && finalResp.outputTokens || 0),
            retryCount: 0,
            exec: {
                executedBlocks: 0,
                elapsedMs: 0,
                policyDenied: countDeniedFromText(String(streamErr)),
                editStats: {
                    totalOps: 0,
                    runOps: 0,
                    createOps: 0,
                    patchOps: 0,
                    byLang: {},
                },
            },
        };
    }

    const execStartedAt = Date.now();
    const blocks = ctx.noExec ? [] : extractCodeBlocks(responseContent);
    let executedBlocks = 0;
    let policyDenied = 0;
    let allResults = [];

    for (let i = 0; i < blocks.length; i++) {
        const block = blocks[i];
        try {
            const results = executeBlock(block, {
                readOnly: true,
                maxRows: ctx.maxRows,
                timeoutMs: ctx.timeoutMs,
                maxOutputBytes: ctx.maxOutputBytes,
            }) || [];
            executedBlocks += 1;
            allResults = allResults.concat(results);
            policyDenied += countDeniedFromResults(results);
        } catch (err) {
            policyDenied += countDeniedFromText(String(err));
            allResults.push({ ok: false, error: String(err) });
        }
    }

    const execElapsedMs = Date.now() - execStartedAt;
    const editStats = collectEditStats(allResults);

    return {
        index: index,
        id: scenario.id,
        ok: true,
        latencyMs: latencyMs,
        ttfbMs: ttfbMs,
        inputTokens: Number(finalResp && finalResp.inputTokens || 0),
        outputTokens: Number(finalResp && finalResp.outputTokens || 0),
        retryCount: 0,
        exec: {
            executedBlocks: executedBlocks,
            elapsedMs: execElapsedMs,
            policyDenied: policyDenied,
            editStats: editStats,
        },
    };
}

function countDeniedFromResults(results) {
    if (!Array.isArray(results) || results.length === 0) {
        return 0;
    }
    let denied = 0;
    for (let i = 0; i < results.length; i++) {
        const one = results[i] || {};
        if (one.ok === false && countDeniedFromText(String(one.error || '')) > 0) {
            denied++;
        }
    }
    return denied;
}

function countDeniedFromText(text) {
    const lower = String(text || '').toLowerCase();
    if (!lower) {
        return 0;
    }
    return (lower.indexOf('capability denied') >= 0 || lower.indexOf('not allowed in read-only mode') >= 0 || lower.indexOf('workspace boundary violation') >= 0) ? 1 : 0;
}

function buildReport(input) {
    const entries = input.entries || [];
    let successCount = 0;
    let failureCount = 0;
    let inputTokens = 0;
    let outputTokens = 0;
    let execOps = 0;
    let runOps = 0;
    let createOps = 0;
    let patchOps = 0;
    let policyDeniedCount = 0;
    let totalLatencyMs = 0;
    let totalTTFBMs = 0;
    let totalExecElapsedMs = 0;

    for (let i = 0; i < entries.length; i++) {
        const one = entries[i] || {};
        if (one.ok) {
            successCount++;
        } else {
            failureCount++;
        }
        inputTokens += Number(one.inputTokens || 0);
        outputTokens += Number(one.outputTokens || 0);
        totalLatencyMs += Number(one.latencyMs || 0);
        totalTTFBMs += Number(one.ttfbMs || 0);

        const exec = one.exec || {};
        const editStats = exec.editStats || {};
        execOps += Number(editStats.totalOps || 0);
        runOps += Number(editStats.runOps || 0);
        createOps += Number(editStats.createOps || 0);
        patchOps += Number(editStats.patchOps || 0);
        policyDeniedCount += Number(exec.policyDenied || 0);
        totalExecElapsedMs += Number(exec.elapsedMs || 0);
    }

    const n = entries.length || 1;
    const patchBase = createOps + patchOps;
    const partialPatchRatio = patchBase > 0 ? (patchOps / patchBase) : 0;

    return {
        generatedAt: input.generatedAt,
        elapsedMs: input.elapsedMs,
        provider: input.provider,
        model: input.model,
        scenariosPath: input.scenariosPath,
        outPath: input.outPath,
        outNdjsonPath: input.outNdjsonPath,
        outCsvPath: input.outCsvPath,
        options: input.options,
        totals: {
            scenarioCount: entries.length,
            successCount: successCount,
            failureCount: failureCount,
            inputTokens: inputTokens,
            outputTokens: outputTokens,
            execOps: execOps,
            runOps: runOps,
            createOps: createOps,
            patchOps: patchOps,
            partialPatchRatio: partialPatchRatio,
            policyDeniedCount: policyDeniedCount,
            totalLatencyMs: totalLatencyMs,
            avgLatencyMs: Math.round(totalLatencyMs / n),
            totalTTFBMs: totalTTFBMs,
            avgTTFBMs: Math.round(totalTTFBMs / n),
            totalExecElapsedMs: totalExecElapsedMs,
            avgExecElapsedMs: Math.round(totalExecElapsedMs / n),
        },
        scenarios: entries,
    };
}

function writeNdjson(path, entries) {
    const lines = [];
    for (let i = 0; i < entries.length; i++) {
        lines.push(JSON.stringify(entries[i] || {}));
    }
    fs.writeFileSync(path, lines.join('\n') + (lines.length > 0 ? '\n' : ''), 'utf8');
}

function csvEscape(value) {
    const raw = String(value === undefined || value === null ? '' : value);
    if (raw.indexOf(',') < 0 && raw.indexOf('"') < 0 && raw.indexOf('\n') < 0 && raw.indexOf('\r') < 0) {
        return raw;
    }
    return '"' + raw.replace(/"/g, '""') + '"';
}

function writeCsv(path, entries) {
    const header = [
        'index',
        'id',
        'ok',
        'dryRun',
        'inputTokens',
        'outputTokens',
        'latencyMs',
        'ttfbMs',
        'retryCount',
        'executedBlocks',
        'execElapsedMs',
        'policyDenied',
        'totalOps',
        'runOps',
        'createOps',
        'patchOps',
        'error',
    ];
    const lines = [header.join(',')];
    for (let i = 0; i < entries.length; i++) {
        const one = entries[i] || {};
        const exec = one.exec || {};
        const stats = exec.editStats || {};
        const row = [
            one.index,
            one.id,
            one.ok,
            one.dryRun,
            one.inputTokens,
            one.outputTokens,
            one.latencyMs,
            one.ttfbMs,
            one.retryCount,
            exec.executedBlocks,
            exec.elapsedMs,
            exec.policyDenied,
            stats.totalOps,
            stats.runOps,
            stats.createOps,
            stats.patchOps,
            one.error,
        ].map(csvEscape);
        lines.push(row.join(','));
    }
    fs.writeFileSync(path, lines.join('\n') + '\n', 'utf8');
}
