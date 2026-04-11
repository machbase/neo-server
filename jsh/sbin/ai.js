'use strict';

// sbin/ai.js — CLI entrypoint for the ai command.
//
// Provides an interactive chat loop with an LLM (Claude / OpenAI).
// Phase 1: text-only chat, no jsh code execution.
// Supports explicit jsh-run execution candidates returned by the LLM.

const process = require('process');
const parseArgs = require('util/parseArgs');
const { ReadLine } = require('readline');
const pretty = require('pretty');
const { ai } = require('@jsh/shell');
const { buildSystemPrompt, listSegments } = require('ai/prompt');
const { extractRunnableCandidates, hasRunnableFence, detectAnalysisIntent, buildEvidenceGatePrompt, buildGroundedReportPrompt, detectUngroundedReport, executeBlock, formatResults, isSqlEvidence, isRenderEnvelope, collectExecutionEvidence, formatEvidencePrompt, collectEditStats, extractErrorLocation, collectErrorDiagnostics, formatDiagnosticsPrompt, buildPatchGuardrailPrompt, buildAutoPatchSuggestionPrompt, detectPatchFirstViolation } = require('ai/executor');
const { saveTranscript } = require('ai/transcript');

// ─── CLI options ──────────────────────────────────────────────────────────────

const options = {
    eval: {
        type: 'string',
        short: 'e',
        description: 'One-shot prompt (non-interactive, prints response and exits)',
    },
    provider: {
        type: 'string',
        short: 'p',
        description: 'LLM provider name (default: from config, e.g. "claude")',
    },
    model: {
        type: 'string',
        short: 'm',
        description: 'Model name override',
    },
    maxTokens: {
        type: 'string',
        description: 'Maximum response tokens (default: from config)',
    },
    noExec: {
        type: 'boolean',
        description: 'Disable jsh-run code execution prompts (safe mode)',
        default: false,
    },
    timeout: {
        type: 'string',
        description: 'jsh code execution timeout in ms (default: 30000)',
    },
    maxRows: {
        type: 'string',
        description: 'Query max rows (default: 1000)',
    },
    out: {
        type: 'string',
        description: 'Output format: text|json (default: text)',
        default: 'text',
    },
    help: {
        type: 'boolean',
        short: 'h',
        description: 'Show this help message',
        default: false,
    },
};

var values = {};
var positionals = [];
var parseError = null;
try {
    var parsed = parseArgs(process.argv.slice(2), { options, allowPositionals: true });
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
        usage: 'Usage: ai [options] [prompt]',
        description: 'Interactive LLM chat with machbase-neo context.\n' +
            'LLM can query your database via the agent API.',
        options: options,
    }));
    console.println('');
    console.println('Slash commands (during interactive session, prefix with "\\" or "/"):');
    console.println('  /provider [name]       Show or switch active LLM provider');
    console.println('  /model <name>          Change model for current provider');
    console.println('  /prompt                List active system prompt segments');
    console.println('  /prompt show           Print assembled system prompt');
    console.println('  /prompt add <segment>  Add a prompt segment');
    console.println('  /prompt rm <segment>   Remove a prompt segment');
    console.println('  /prompt list           List all available segments');
    console.println('  /config show           Print config file contents');
    console.println('  /config set <k> <v>    Set a config value (dot-notation)');
    console.println('  /config edit           Edit config file in host editor');
    console.println('  /config path           Print config file path');
    console.println('  /metrics [reset]       Show or reset session KPI metrics');
    console.println('  /clear                 Clear conversation history');
    console.println('  /save <file_path>      Save the current session as Markdown (.md recommended)');
    console.println('  /help                  Show this help');
    console.println('  /bye /exit /quit       Exit');
    process.exit(parseError ? 1 : 0);
}

// ─── Config apply ─────────────────────────────────────────────────────────────

if (values.provider) {
    ai.setProvider(values.provider);
}
if (values.model) {
    ai.setModel(values.model);
}
if (values.provider || values.model) {
    var cliInfo = ai.providerInfo();
    ai.lastConfig.save({ provider: cliInfo.name, model: cliInfo.model });
}

// ─── Conversation state ───────────────────────────────────────────────────────

var history = [];   // [{role, content}, ...]
var cfg = ai.config.load();
var sessionMetrics = {
    turns: 0,
    assistantReplies: 0,
    inputTokens: 0,
    outputTokens: 0,
    execOps: 0,
    runOps: 0,
    createOps: 0,
    patchOps: 0,
    executedBlocks: 0,
    execElapsedMs: 0,
    analysisIntentTurns: 0,
    evidenceGateRetryCount: 0,
    groundedReportRetryCount: 0,
    groundedCitationPassCount: 0,
    autoRepairCount: 0,
};
var activeSegments = (cfg.prompt && cfg.prompt.segments)
    ? cfg.prompt.segments.slice()
    : ['jsh-runtime', 'jsh-modules', 'agent-api', 'machbase-sql'];

function resetSessionMetrics() {
    sessionMetrics.turns = 0;
    sessionMetrics.assistantReplies = 0;
    sessionMetrics.inputTokens = 0;
    sessionMetrics.outputTokens = 0;
    sessionMetrics.execOps = 0;
    sessionMetrics.runOps = 0;
    sessionMetrics.createOps = 0;
    sessionMetrics.patchOps = 0;
    sessionMetrics.executedBlocks = 0;
    sessionMetrics.execElapsedMs = 0;
    sessionMetrics.analysisIntentTurns = 0;
    sessionMetrics.evidenceGateRetryCount = 0;
    sessionMetrics.groundedReportRetryCount = 0;
    sessionMetrics.groundedCitationPassCount = 0;
    sessionMetrics.autoRepairCount = 0;
}

function recordStreamMetrics(streamResult) {
    if (!streamResult || !streamResult.response) {
        return;
    }
    var resp = streamResult.response;
    sessionMetrics.assistantReplies += 1;
    sessionMetrics.inputTokens += Number(resp.inputTokens || 0);
    sessionMetrics.outputTokens += Number(resp.outputTokens || 0);
}

function recordExecMetrics(execResult) {
    if (!execResult || !execResult.editStats) {
        return;
    }
    var s = execResult.editStats;
    sessionMetrics.execOps += Number(s.totalOps || 0);
    sessionMetrics.runOps += Number(s.runOps || 0);
    sessionMetrics.createOps += Number(s.createOps || 0);
    sessionMetrics.patchOps += Number(s.patchOps || 0);
    sessionMetrics.executedBlocks += Number(execResult.executedCount || 0);
    sessionMetrics.execElapsedMs += Number(execResult.elapsedMs || 0);
}

function printSessionMetrics() {
    console.println('Session metrics');
    console.println('  turns: ' + sessionMetrics.turns);
    console.println('  assistant replies: ' + sessionMetrics.assistantReplies);
    console.println('  tokens: ' + sessionMetrics.inputTokens + ' in / ' + sessionMetrics.outputTokens + ' out');
    console.println('  exec ops: ' + sessionMetrics.execOps + ' (run=' + sessionMetrics.runOps + ' create=' + sessionMetrics.createOps + ' patch=' + sessionMetrics.patchOps + ')');
    console.println('  executed blocks: ' + sessionMetrics.executedBlocks);
    console.println('  exec elapsed: ' + sessionMetrics.execElapsedMs + 'ms');
    console.println('  analysis intent turns: ' + sessionMetrics.analysisIntentTurns);
    console.println('  evidence gate retries: ' + sessionMetrics.evidenceGateRetryCount);
    console.println('  grounded report retries: ' + sessionMetrics.groundedReportRetryCount + ' / pass: ' + sessionMetrics.groundedCitationPassCount);
    console.println('  auto repairs: ' + sessionMetrics.autoRepairCount);
}


function systemPrompt() {
    return buildSystemPrompt(activeSegments);
}

// ─── Output helpers ───────────────────────────────────────────────────────────

var BOLD = '\x1B[1m';
var DIM = '\x1B[2m';
var CYAN = '\x1B[36m';
var YELLOW = '\x1B[33m';
var GREEN = '\x1B[32m';
var RED = '\x1B[31m';
var RESET = '\x1B[0m';
var PROVIDER_MODELS = {
    claude: ['claude-opus-4-5', 'claude-sonnet-4', 'claude-haiku-3-5'],
    openai: ['gpt-5.4', 'gpt-5.4-mini', 'gpt-5.3-codex', 'gpt-4o-mini'],
    ollama: ['llama3.1', 'qwen2.5', 'mistral'],
};

function printUser(text) {
    console.println(BOLD + CYAN + 'You> ' + RESET + text);
}

function printAI(label) {
    process.stdout.write(BOLD + GREEN + label + '> ' + RESET);
}

function printInfo(text) {
    console.println(DIM + text + RESET);
}

function printError(text) {
    console.println(RED + 'Error: ' + text + RESET);
}

function renderStatsTable(stats) {
    var box = pretty.Table({ rownum: false, footer: false });
    box.appendHeader(['NAME', 'VALUE']);
    for (var i = 0; i < stats.length; i++) {
        var one = stats[i] || {};
        box.appendRow(box.row(String(one.label || ''), String(one.value || '')));
    }
    return box.render();
}

function renderRowsTable(block) {
    var box = pretty.Table({ footer: false });
    var cols = (Array.isArray(block.columns) && block.columns.length > 0) ? block.columns : ['VALUE'];
    box.appendHeader(cols);
    var rows = Array.isArray(block.rows) ? block.rows : [];
    for (var i = 0; i < rows.length; i++) {
        var row = rows[i];
        if (Array.isArray(row)) {
            box.append(row);
        } else {
            box.append([String(row)]);
        }
    }
    return box.render();
}

function renderAdvnBlocks(blocks) {
    for (var i = 0; i < blocks.length; i++) {
        var block = blocks[i] || {};
        if (i > 0) {
            console.println('');
        }
        if (block.title) {
            console.println(String(block.title));
            console.println('='.repeat(String(block.title).length));
        }
        if (Array.isArray(block.stats) && block.stats.length > 0) {
            console.println(renderStatsTable(block.stats));
        }
        if (Array.isArray(block.lines) && block.lines.length > 0) {
            for (var j = 0; j < block.lines.length; j++) {
                console.println(String(block.lines[j]));
            }
        }
        if (Array.isArray(block.rows) && block.rows.length > 0) {
            console.println(renderRowsTable(block));
        }
    }
}

function renderAdvnLines(lines) {
    for (var i = 0; i < lines.length; i++) {
        console.println(String(lines[i]));
    }
}

function renderAgentEnvelope(env) {
    if (!isRenderEnvelope(env)) {
        return false;
    }
    console.println(DIM + '[VIZSPEC TUI render: ' + env.mode + ']' + RESET);
    try {
        if (env.mode === 'blocks') {
            renderAdvnBlocks(Array.isArray(env.blocks) ? env.blocks : []);
            return true;
        }
        if (env.mode === 'lines') {
            renderAdvnLines(Array.isArray(env.lines) ? env.lines : []);
            return true;
        }
    } catch (e) {
        printError('VIZSPEC render failed: ' + (e.message || String(e)));
        return false;
    }
    return false;
}

function normalizeSlashCommand(line) {
    if (!line) {
        return line;
    }
    if (line.charAt(0) === '/') {
        return '\\' + line.slice(1);
    }
    return line;
}

function saveLastSelection() {
    var info = ai.providerInfo();
    ai.lastConfig.save({ provider: info.name, model: info.model });
}

function modelExamples(providerName) {
    return PROVIDER_MODELS[providerName] || [];
}

function printProviderExamples() {
    var names = Object.keys(PROVIDER_MODELS);
    for (var i = 0; i < names.length; i++) {
        var name = names[i];
        console.println('  ' + name + '  ' + DIM + '(' + modelExamples(name).join(', ') + ')' + RESET);
    }
}

function printModelExamples(providerName) {
    var examples = modelExamples(providerName);
    if (examples.length === 0) {
        printInfo('No model examples available for provider: ' + providerName);
        return;
    }
    printInfo('Model examples for ' + providerName + ': ' + examples.join(', '));
}

function formatWaitDuration(elapsedMs) {
    if (elapsedMs < 1000) {
        return elapsedMs + 'ms';
    }
    return (elapsedMs / 1000).toFixed(1) + 's';
}

function streamAssistantReply(options) {
    var startedAt = Date.now();
    var firstTokenSeen = false;
    var responseContent = '';
    var streamErr = null;
    var finalResp = null;
    var info = ai.providerInfo();

    try {
        ai.stream(history, systemPrompt(), {
            data: function (token) {
                if (!firstTokenSeen) {
                    firstTokenSeen = true;
                    process.stdout.write(BOLD + GREEN + info.name + '> ' + RESET);
                    process.stdout.write(DIM + '[waited ' + formatWaitDuration(Date.now() - startedAt) + '] ' + RESET);
                }
                process.stdout.write(token);
                responseContent += token;
            },
            end: function (resp) {
                finalResp = resp;
                if (!firstTokenSeen) {
                    process.stdout.write(BOLD + GREEN + info.name + '> ' + RESET);
                    process.stdout.write(DIM + '[waited ' + formatWaitDuration(Date.now() - startedAt) + '] ' + RESET);
                }
                console.println('');
                console.println(DIM + '[tokens: ' + (resp.inputTokens || 0) + ' in / ' + (resp.outputTokens || 0) + ' out]' + RESET);
            },
            error: function (err) {
                streamErr = err;
            }
        }, {
            waitLabel: BOLD + GREEN + info.name + '> ' + RESET,
            waitIntervalMs: 250,
        });
    } catch (e) {
        streamErr = e.message || String(e);
    }

    if (streamErr && !firstTokenSeen) {
        console.println('');
    }

    return {
        content: responseContent,
        error: streamErr,
        response: finalResp,
    };
}

// ─── Provider setup recovery ──────────────────────────────────────────────────

function isAuthError(errMsg) {
    return errMsg.indexOf('401') >= 0 ||
        errMsg.indexOf('authentication_error') >= 0 ||
        errMsg.indexOf('api-key') >= 0 ||
        errMsg.indexOf('x-api-key') >= 0 ||
        errMsg.indexOf('API key') >= 0;
}

// Prompt the user to enter provider-specific configuration interactively.
// Returns true if a value was entered/saved (caller should retry), false to cancel.
function promptForProviderSetup(reason) {
    var info = ai.providerInfo();
    var providerName = info.name;
    var configKey = 'apiKey';
    var valueLabel = 'API key';
    var promptLabel = 'API Key';
    var currentValue = '';

    var envVarHint = '';
    if (providerName === 'claude') {
        envVarHint = 'ANTHROPIC_API_KEY';
    } else if (providerName === 'openai') {
        envVarHint = 'OPENAI_API_KEY';
    } else if (providerName === 'ollama') {
        envVarHint = 'OLLAMA_HOST';
        configKey = 'baseUrl';
        valueLabel = 'base URL';
        promptLabel = 'Base URL';
        currentValue = info.baseUrl || 'http://127.0.0.1:11434';
    }

    console.println('');
    if (reason === 'missing') {
        console.println(YELLOW + 'No ' + valueLabel + ' configured for provider: ' + BOLD + providerName + RESET);
    } else {
        console.println(YELLOW + 'Provider setup failed for provider: ' + BOLD + providerName + RESET);
    }
    if (providerName === 'ollama') {
        console.println(DIM + '  Enter the Ollama base URL, "." to open config editor, or press Enter to use the default.' + RESET);
        console.println(DIM + '  Default: ' + currentValue + RESET);
    } else {
        console.println(DIM + '  Enter your API key, "." to open config editor, or press Enter to cancel.' + RESET);
    }
    if (envVarHint) {
        console.println(DIM + '  Tip: you can also set the ' + envVarHint + ' environment variable.' + RESET);
    }
    console.println('');

    var keyRL = new ReadLine({ historyName: '' });
    var value;
    try {
        value = keyRL.readLine({ prompt: function () { return YELLOW + promptLabel + '> ' + RESET; } });
    } catch (e) {
        return false;
    } finally {
        keyRL.close();
    }

    if (value === null || value === undefined) {
        return false;
    }
    value = value.trim();

    if (!value) {
        if (providerName === 'ollama') {
            value = currentValue;
        } else {
            // Empty Enter → cancel
            printInfo('Cancelled.');
            return false;
        }
    }

    if (value === '.') {
        // "." → open editor
        var result = ai.editConfig();
        if (result === 'saved') {
            cfg = ai.config.load();
            ai.setProvider(providerName);
            printInfo('Config saved and reloaded.');
            return true;
        } else if (result === 'no-editor') {
            if (providerName === 'ollama') {
                printInfo('No host editor found. Use \\config set providers.' + providerName + '.baseUrl <url>');
            } else {
                printInfo('No host editor found. Use \\config set providers.' + providerName + '.apiKey <key>');
            }
        } else if (result === 'invalid-json') {
            printInfo('Invalid JSON in config — changes discarded.');
        } else {
            printInfo('Edit cancelled.');
        }
        return false;
    }

    // Save the entered value into config
    try {
        ai.config.set('providers.' + providerName + '.' + configKey, value);
        cfg = ai.config.load();
        ai.setProvider(providerName);
        if (providerName === 'ollama') {
            printInfo('Base URL saved. Retrying...');
        } else {
            printInfo('API key saved. Retrying...');
        }
        return true;
    } catch (e) {
        printError('Failed to save ' + valueLabel + ': ' + (e.message || String(e)));
        return false;
    }
}

// Check provider configuration before sending — returns true if ready, false if user cancelled.
function ensureProviderReady() {
    var info = ai.providerInfo();
    if ((info.name === 'claude' || info.name === 'openai') && !info.hasApiKey) {
        return promptForProviderSetup('missing');
    }
    if (info.name === 'ollama' && !info.hasBaseUrl) {
        return promptForProviderSetup('missing');
    }
    return true;
}

// ─── Code execution (Phase 2) ─────────────────────────────────────────────────

// Execution options derived from CLI flags and config.
var execOpts = (function () {
    var cfg2 = ai.config.load();
    var execCfg = cfg2.exec || {};
    return {
        readOnly: execCfg.readOnly !== false,   // default true
        maxRows: parseInt(values.maxRows, 10) || execCfg.maxRows || 1000,
        timeoutMs: parseInt(values.timeout, 10) || execCfg.timeoutMs || 30000,
    };
}());

var CODE_BOLD = '\x1B[1m';
var CODE_BG = '\x1B[48;5;236m';  // dark grey background
var CODE_RESET2 = '\x1B[0m';

// Whether to print console.log output from executed jsh-run code to the terminal.
// Off by default — output is always sent to the LLM regardless of this flag.
// Toggle with \verbose.
var verboseExec = false;

// Print a code block with a simple border so the user can review it.
function printCodeBlock(code, lang) {
    console.println(DIM + '┌─ ' + lang + ' ─────────────────────────────────────────' + RESET);
    var codeLines = code.split('\n');
    for (var i = 0; i < codeLines.length; i++) {
        console.println(codeLines[i]);
    }
    console.println(DIM + '└────────────────────────────────────────────────' + RESET);
}

// Ask the user whether to execute a runnable block.
// Returns 'yes', 'no', 'all' (execute this and all following blocks), or 'cancel'.
function promptExec(confirmRL, lang) {
    while (true) {
        var answer;
        try {
            answer = confirmRL.readLine({
                prompt: function () { return YELLOW + 'Execute this ' + lang + ' block? [y/n/a(ll)/c(ancel)] ' + RESET; }
            });
        } catch (e) {
            return 'cancel';
        }
        if (answer === null || answer === undefined) { return 'cancel'; }
        answer = answer.trim().toLowerCase();
        if (answer === 'y' || answer === 'yes') { return 'yes'; }
        if (answer === 'n' || answer === 'no') { return 'no'; }
        if (answer === 'a' || answer === 'all') { return 'all'; }
        if (answer === 'c' || answer === 'cancel') { return 'cancel'; }
        printInfo('Invalid input. Use y, n, a, or c.');
    }
}

// Run all confirmed runnable blocks from an LLM response.
// Returns an object with summary string and execution metrics,
// or null if no blocks were executed.
//
// When --no-exec is set, blocks are displayed but not executed.
//
// @param {string} responseText   full LLM response text
// @returns {{summary: string, editStats: object, diagnostics: object[], executedCount: number}|null}
function handleCodeBlocks(responseText) {
    var blocks = extractRunnableCandidates(responseText, { autoRepair: true });
    if (blocks.length === 0) { return null; }

    if (values.noExec) {
        console.println('');
        printInfo('[--no-exec] ' + blocks.length + ' runnable block(s) detected (not executed)');
        for (var i = 0; i < blocks.length; i++) {
            printCodeBlock(blocks[i].code, blocks[i].lang);
        }
        return null;
    }

    var confirmRL = new ReadLine({ historyName: '' });
    var execAll = false;
    var allOutput = [];
    var allResults = [];
    var allEvidence = [];
    var executedCount = 0;
    var accElapsedMs = 0;

    try {
        for (var i = 0; i < blocks.length; i++) {
            var block = blocks[i];
            var lineCount = block.code.split('\n').length;
            // The LLM response already streamed the code to the screen.
            // Showing it again in a box would be redundant — show a compact summary instead.
            console.println('');
            printInfo('[Runnable block ' + (i + 1) + '/' + blocks.length + '] ' + block.lang + ' · ' + lineCount + ' lines');
            if (block.promoted) {
                sessionMetrics.autoRepairCount += 1;
                printInfo('[Auto-repair] promoted plain ' + block.promotedFrom + ' fence to ' + block.lang + '.');
            }

            var decision = execAll ? 'yes' : promptExec(confirmRL, block.lang);
            if (decision === 'all') {
                execAll = true;
                decision = 'yes';
            }
            if (decision === 'cancel') {
                printInfo('Cancelled execution flow.');
                break;
            }
            if (decision !== 'yes') {
                printInfo('Skipped.');
                continue;
            }

            printInfo('Executing...');
            var results = executeBlock(block, execOpts);
            executedCount++;
            allResults = allResults.concat(results);
            allEvidence = allEvidence.concat(collectExecutionEvidence(results, block));

            // Print execution results to the user.
            var hadError = false;
            var lastElapsedMs = 0;
            for (var j = 0; j < results.length; j++) {
                var r = results[j];
                if (!r.ok) {
                    console.println(RED + 'Error: ' + r.error + RESET);
                    hadError = true;
                } else if (r.type === 'print') {
                    // console.log/println output captured from the executed code.
                    // Only shown to the user when verbose mode is on; always sent to LLM.
                    if (verboseExec) {
                        console.println(String(r.value));
                    }
                } else if (isSqlEvidence(r.value)) {
                    console.println(String(r.value.rendered || '(no rows)'));
                    if (r.value.truncated) {
                        printInfo('[output truncated]');
                    }
                } else if (isRenderEnvelope(r.value)) {
                    if (!renderAgentEnvelope(r.value)) {
                        if (typeof r.value === 'object') {
                            console.println(JSON.stringify(r.value, null, 2));
                        } else {
                            console.println(String(r.value));
                        }
                    }
                } else if (r.value !== undefined && r.value !== null && r.type !== 'undefined') {
                    if (typeof r.value === 'object') {
                        console.println(JSON.stringify(r.value, null, 2));
                    } else {
                        console.println(String(r.value));
                    }
                    if (r.truncated) {
                        printInfo('[output truncated]');
                    }
                }
                if (r.elapsedMs !== undefined) { lastElapsedMs = r.elapsedMs; accElapsedMs += lastElapsedMs; }
            }
            printInfo('[' + lastElapsedMs + 'ms]');

            var summary = formatResults(results);
            allOutput.push('```\n' + summary + '\n```');
        }
    } finally {
        confirmRL.close();
    }

    if (allOutput.length === 0) { return null; }

    var editStats = collectEditStats(allResults);
    var diagnostics = collectErrorDiagnostics(allResults, { contextLines: 2 });
    var evidence = allEvidence;
    var evidencePrompt = formatEvidencePrompt(evidence, { maxItems: 3 });

    return {
        summary: 'Code execution results:\n\n' + allOutput.join('\n\n'),
        editStats: editStats,
        diagnostics: diagnostics,
        evidence: evidence,
        evidencePrompt: evidencePrompt,
        executedCount: executedCount,
        elapsedMs: accElapsedMs,
    };
}

// ─── One-shot mode ────────────────────────────────────────────────────────────

function runOneShot(prompt) {
    history.push({ role: 'user', content: prompt });
    try {
        var resp = ai.send(history, systemPrompt());
        var content = resp.content || '';
        console.println(content);
        if (values.out === 'json') {
            console.println(JSON.stringify({
                content: content,
                inputTokens: resp.inputTokens,
                outputTokens: resp.outputTokens,
                provider: resp.provider,
                model: resp.model,
            }));
        }
        process.exit(0);
    } catch (e) {
        printError(e.message || String(e));
        process.exit(1);
    }
}

// One-shot: -e flag or positional arguments
if (values.eval !== undefined) {
    runOneShot(values.eval);
} else if (positionals.length > 0) {
    runOneShot(positionals.join(' '));
}

// ─── Slash command handler ────────────────────────────────────────────────────

function handleSlash(line) {
    line = normalizeSlashCommand(line);
    var parts = line.trim().split(/\s+/);
    var cmd = parts[0].toLowerCase();

    if (cmd === '\\bye' || cmd === '\\exit' || cmd === '\\quit') {
        printInfo('Goodbye.');
        process.exit(0);

    } else if (cmd === '\\help') {
        console.println('');
        console.println(BOLD + 'Slash Commands' + RESET);
        console.println('');
        console.println(BOLD + CYAN + '  Conversation' + RESET);
        console.println('    ' + BOLD + '/clear' + RESET);
        console.println('        Clear conversation history (start fresh context).');
        console.println('    ' + BOLD + '/save <file_path>' + RESET);
        console.println('        Save the current session as a Markdown transcript (.md recommended).');
        console.println('    ' + BOLD + '/metrics [reset]' + RESET);
        console.println('        Show current session KPI metrics or reset counters.');
        console.println('');
        console.println(BOLD + CYAN + '  Provider & Model' + RESET);
        console.println('    ' + BOLD + '/provider' + RESET);
        console.println('        Show current active LLM provider and model, plus supported provider examples.');
        console.println('    ' + BOLD + '/provider <name>' + RESET);
        console.println('        Switch provider (e.g. claude, openai). Conversation history is kept.');
        console.println('    ' + BOLD + '/model <name>' + RESET);
        console.println('        Change model for the current provider and save it as the last selection.');
        console.println('');
        console.println(BOLD + CYAN + '  System Prompt' + RESET);
        console.println('    ' + BOLD + '/prompt' + RESET);
        console.println('        List currently active system prompt segments.');
        console.println('    ' + BOLD + '/prompt show' + RESET);
        console.println('        Print the full assembled system prompt text.');
        console.println('    ' + BOLD + '/prompt list' + RESET);
        console.println('        List all available segments (builtin + custom overrides).');
        console.println('    ' + BOLD + '/prompt add <segment>' + RESET);
        console.println('        Add a segment to the active list for this session.');
        console.println('    ' + BOLD + '/prompt rm <segment>' + RESET);
        console.println('        Remove a segment from the active list for this session.');
        console.println('');
        console.println(BOLD + CYAN + '  Configuration' + RESET);
        console.println('    ' + BOLD + '/config show' + RESET);
        console.println('        Print the current config file contents.');
        console.println('        Path: ' + DIM + '$HOME/.config/machbase/llm/config.json' + RESET);
        console.println('    ' + BOLD + '/config path' + RESET);
        console.println('        Print the absolute path to the config file.');
        console.println('    ' + BOLD + '/config lastpath' + RESET);
        console.println('        Print the absolute path to the last-selection file.');
        console.println('    ' + BOLD + '/config set <key> <value>' + RESET);
        console.println('        Update a single config value using dot-notation key.');
        console.println('        Examples:');
        console.println('          ' + DIM + '/config set defaultProvider openai' + RESET);
        console.println('          ' + DIM + '/config set providers.openai.baseUrl https://api.openai.com/v1' + RESET);
        console.println('          ' + DIM + '/config set exec.maxRows 500' + RESET);
        console.println('    ' + BOLD + '/config edit' + RESET);
        console.println('        Open the config file in $EDITOR / vi / nano.');
        console.println('');
        console.println(BOLD + CYAN + '  Exit' + RESET);
        console.println('    ' + BOLD + '/bye' + RESET + '  ' + BOLD + '/exit' + RESET + '  ' + BOLD + '/quit' + RESET);
        console.println('        Exit the AI assistant.');
        console.println('');
        console.println(BOLD + CYAN + '  Execution output' + RESET);
        console.println('    ' + BOLD + '/verbose' + RESET);
        console.println('        Toggle verbose mode for jsh code execution.');
        console.println('        When ON, console.log output from executed code is printed to the terminal.');
        console.println('        Output is always sent to the LLM regardless of this setting.');
        console.println('        Current: ' + (verboseExec ? BOLD + 'ON' + RESET : DIM + 'OFF' + RESET));
        console.println('');

    } else if (cmd === '\\clear') {
        history = [];
        printInfo('Conversation history cleared.');

    } else if (cmd === '\\metrics') {
        var metricSub = parts[1] ? parts[1].toLowerCase() : '';
        if (metricSub === 'reset') {
            resetSessionMetrics();
            printInfo('Session metrics reset.');
        } else {
            printSessionMetrics();
        }

    } else if (cmd === '\\save') {
        var saveArg = line.trim().slice(cmd.length).trim();
        if (!saveArg) {
            printInfo('Usage: \\save <file_path>');
            return;
        }
        try {
            var provider = ai.providerInfo();
            var saved = saveTranscript(saveArg, {
                cwd: process.cwd(),
                history: history,
                provider: provider.name || 'unknown',
                model: provider.model || 'unknown',
                promptSegments: activeSegments,
            });
            printInfo('Saved ' + saved.turns + ' turn(s) to ' + saved.path);
        } catch (e) {
            printError(e.message || String(e));
        }

    } else if (cmd === '\\provider') {
        if (parts.length > 1) {
            try {
                ai.setProvider(parts[1]);
                saveLastSelection();
                var info = ai.providerInfo();
                printInfo('Switched to provider: ' + info.name + ' / ' + info.model);
            } catch (e) {
                printError(e.message || String(e));
                printInfo('Supported providers:');
                printProviderExamples();
            }
        } else {
            var info = ai.providerInfo();
            printInfo('Provider: ' + info.name + '  Model: ' + info.model);
            printProviderExamples();
        }

    } else if (cmd === '\\model') {
        if (parts.length < 2) {
            printInfo('Usage: /model <name>');
            printModelExamples(ai.providerInfo().name);
            return;
        }
        try {
            ai.setModel(parts[1]);
            saveLastSelection();
            printInfo('Model set to: ' + parts[1]);
            printModelExamples(ai.providerInfo().name);
        } catch (e) {
            printError(e.message || String(e));
        }

    } else if (cmd === '\\prompt') {
        var sub = parts[1] ? parts[1].toLowerCase() : '';
        if (!sub) {
            printInfo('Active segments: ' + activeSegments.join(', '));
        } else if (sub === 'show') {
            console.println(systemPrompt());
        } else if (sub === 'list') {
            var all = listSegments();
            printInfo('Available segments: ' + all.join(', '));
        } else if (sub === 'add') {
            if (!parts[2]) { printInfo('Usage: \\prompt add <segment>'); return; }
            if (activeSegments.indexOf(parts[2]) < 0) {
                activeSegments.push(parts[2]);
                printInfo('Added segment: ' + parts[2]);
            } else {
                printInfo('Segment already active: ' + parts[2]);
            }
        } else if (sub === 'rm') {
            if (!parts[2]) { printInfo('Usage: \\prompt rm <segment>'); return; }
            var idx = activeSegments.indexOf(parts[2]);
            if (idx >= 0) {
                activeSegments.splice(idx, 1);
                printInfo('Removed segment: ' + parts[2]);
            } else {
                printInfo('Segment not in active list: ' + parts[2]);
            }
        } else {
            printInfo('Unknown \\prompt sub-command: ' + sub);
        }

    } else if (cmd === '\\config') {
        var sub = parts[1] ? parts[1].toLowerCase() : 'show';
        if (sub === 'show') {
            try {
                var c = ai.config.load();
                console.println(JSON.stringify(c, null, 2));
            } catch (e) {
                printError(e.message || String(e));
            }
        } else if (sub === 'path') {
            console.println(ai.config.path());
        } else if (sub === 'set') {
            if (parts.length < 4) { printInfo('Usage: \\config set <key> <value>'); return; }
            try {
                ai.config.set(parts[2], parts[3]);
                printInfo('Config updated: ' + parts[2] + ' = ' + parts[3]);
                cfg = ai.config.load();
            } catch (e) {
                printError(e.message || String(e));
            }
        } else if (sub === 'edit') {
            var result = ai.editConfig();
            if (result === 'saved') {
                cfg = ai.config.load();
                printInfo('Config saved and reloaded.');
            } else if (result === 'no-editor') {
                printInfo('No host editor found. Use \\config set <key> <value>.');
            } else if (result === 'invalid-json') {
                printInfo('Invalid JSON in config — changes discarded.');
            } else if (result === 'cancelled') {
                printInfo('Edit cancelled.');
            }
        } else if (sub === 'lastpath') {
            console.println(ai.lastConfig.path());
        } else {
            printInfo('Unknown \\config sub-command: ' + sub);
        }

    } else if (cmd === '\\verbose') {
        verboseExec = !verboseExec;
        printInfo('Verbose execution output: ' + (verboseExec ? 'ON' : 'OFF'));

    } else {
        printInfo('Unknown slash command: ' + cmd + '  (type \\help for list)');
    }
}

// ─── Interactive loop ─────────────────────────────────────────────────────────

var providerInfo = ai.providerInfo();
console.println(BOLD + '  machbase-neo AI assistant' + RESET);
console.println(DIM + '  provider: ' + providerInfo.name + '  model: ' + providerInfo.model + RESET);
console.println(DIM + '  type /help for commands, /bye to exit' + RESET);
console.println('');

var rl = new ReadLine({ historyName: 'ai_history', prompt: function () { return 'ai> '; } });

while (true) {
    var line;
    try {
        line = rl.readLine({ prompt: function () { return BOLD + CYAN + 'You' + RESET + '> '; } });
    } catch (e) {
        // EOF or Ctrl-D
        console.println('');
        break;
    }

    if (line === null || line === undefined) {
        break;
    }

    line = line.trim();
    if (!line) {
        continue;
    }

    // Record non-empty input in readline history.
    rl.addHistory(line);

    // Slash commands
    if (line.charAt(0) === '\\' || line.charAt(0) === '/') {
        handleSlash(line);
        continue;
    }

    // Check provider configuration before sending
    if (!ensureProviderReady()) {
        continue;
    }

    var turnState = {
        analysisIntent: detectAnalysisIntent(line),
        hasEvidence: false,
    };

    if (turnState.analysisIntent) {
        sessionMetrics.analysisIntentTurns += 1;
    }

    // User message — add to history and stream response
    history.push({ role: 'user', content: line });

    var streamResult = streamAssistantReply();
    var responseContent = streamResult.content;
    var streamErr = streamResult.error;

    if (streamErr) {
        history.pop();
        if (isAuthError(String(streamErr))) {
            if (promptForProviderSetup('auth')) {
                // Re-push user message and retry once
                history.push({ role: 'user', content: line });
                streamResult = streamAssistantReply();
                responseContent = streamResult.content;
                streamErr = streamResult.error;
                if (streamErr) {
                    printError(String(streamErr));
                    history.pop();
                }
            }
            continue;
        }
        printError(String(streamErr));
        continue;
    }

    sessionMetrics.turns += 1;
    recordStreamMetrics(streamResult);

    if (responseContent) {
        history.push({ role: 'assistant', content: responseContent });
    }

    if (turnState.analysisIntent && responseContent && extractRunnableCandidates(responseContent, { autoRepair: true }).length === 0) {
        var evidenceGatePrompt = buildEvidenceGatePrompt();
        history.push({ role: 'user', content: evidenceGatePrompt });
        sessionMetrics.evidenceGateRetryCount += 1;
        printInfo('[Evidence Gate] analysis/report request detected; requesting runnable evidence first.');

        var evidenceRetryResult = streamAssistantReply();
        var evidenceRetryContent = evidenceRetryResult.content;
        var evidenceRetryErr = evidenceRetryResult.error;
        if (evidenceRetryErr) {
            printError(String(evidenceRetryErr));
        } else if (evidenceRetryContent) {
            recordStreamMetrics(evidenceRetryResult);
            history.push({ role: 'assistant', content: evidenceRetryContent });
            responseContent = evidenceRetryContent;
        }
    }

    // Detect jsh-run blocks, ask the user, and execute confirmed ones.
    // Loop: if the analysis response itself contains more code blocks, handle them too.
    var currentContent = responseContent;
    while (true) {
        var execResult = handleCodeBlocks(currentContent);
        if (!execResult) { break; }
        recordExecMetrics(execResult);
        turnState.hasEvidence = execResult.executedCount > 0;

        // Display execution metrics to user
        if (execResult.editStats && execResult.editStats.totalOps > 0) {
            var metricsLine = `[Exec] blocks=${execResult.executedCount}, ops=${execResult.editStats.totalOps} ` +
                `(run=${execResult.editStats.runOps || 0} create=${execResult.editStats.createOps || 0} patch=${execResult.editStats.patchOps || 0}), ` +
                `time=${execResult.elapsedMs || 0}ms`;
            printInfo(metricsLine);
        }

        // Record what the tool produced so the LLM sees execution context.
        history.push({ role: 'user', content: execResult.summary });

        // Add structured diagnostics so follow-up can patch the exact location.
        var diagPrompt = formatDiagnosticsPrompt(execResult.diagnostics, 3);
        if (diagPrompt) {
            history.push({ role: 'user', content: diagPrompt });
            printInfo('[Diagnostics] attached ' + execResult.diagnostics.length + ' error location(s) for patch-first retry.');

            var patchSuggestionPrompt = buildAutoPatchSuggestionPrompt(execResult.diagnostics, { maxCount: 2 });
            if (patchSuggestionPrompt) {
                history.push({ role: 'user', content: patchSuggestionPrompt });
                printInfo('[Patch Suggestion] attached minimal patch candidates for faster retry.');
            }
        }

        if (execResult.evidencePrompt) {
            history.push({ role: 'user', content: execResult.evidencePrompt });
            printInfo('[Evidence] attached structured execution evidence for grounded follow-up.');
        }

        // Ask LLM to interpret the results.
        printInfo('Sending execution results for analysis...');
        var analysisResult = streamAssistantReply();
        var analysisContent = analysisResult.content;
        var analysisErr = analysisResult.error;

        if (analysisErr) {
            printError(String(analysisErr));
            break;
        }
        if (!analysisContent) { break; }

        recordStreamMetrics(analysisResult);

        // Guardrail: if diagnostics exist and analysis returns large regeneration-style code,
        // ask one more time for patch-first response.
        var needPatchGuardrailRetry = detectPatchFirstViolation(
            analysisContent,
            execResult.diagnostics,
            { lineThreshold: 80 }
        );
        if (needPatchGuardrailRetry) {
            var guardrailPrompt = buildPatchGuardrailPrompt(execResult.diagnostics, { maxCount: 2 });
            if (guardrailPrompt) {
                history.push({ role: 'user', content: guardrailPrompt });
                printInfo('[Guardrail] regeneration-style response detected; requesting patch-first retry...');

                var guardrailRetryResult = streamAssistantReply();
                var guardrailRetryContent = guardrailRetryResult.content;
                var guardrailRetryErr = guardrailRetryResult.error;
                if (!guardrailRetryErr && guardrailRetryContent) {
                    analysisContent = guardrailRetryContent;
                    recordStreamMetrics(guardrailRetryResult);
                } else if (guardrailRetryErr) {
                    printError(String(guardrailRetryErr));
                }
            }
        }

        if (turnState.analysisIntent && turnState.hasEvidence) {
            var needGroundedReportRetry = detectUngroundedReport(
                analysisContent,
                execResult.evidence,
                { minResponseLength: 40, maxHints: 8 }
            );
            if (needGroundedReportRetry) {
                var groundedReportPrompt = buildGroundedReportPrompt(execResult.evidence, { maxHints: 8 });
                history.push({ role: 'user', content: groundedReportPrompt });
                sessionMetrics.groundedReportRetryCount += 1;
                printInfo('[Grounded Report] unsupported summary detected; requesting evidence-grounded rewrite...');

                var groundedRetryResult = streamAssistantReply();
                var groundedRetryContent = groundedRetryResult.content;
                var groundedRetryErr = groundedRetryResult.error;
                if (!groundedRetryErr && groundedRetryContent) {
                    analysisContent = groundedRetryContent;
                    recordStreamMetrics(groundedRetryResult);
                } else if (groundedRetryErr) {
                    printError(String(groundedRetryErr));
                }
            } else {
                // Validation passed
                sessionMetrics.groundedCitationPassCount += 1;
            }
        }

        history.push({ role: 'assistant', content: analysisContent });
        // Check if analysis response also contains code blocks.
        currentContent = analysisContent;
    }
}

rl.close();
