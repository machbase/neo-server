'use strict';
const { simple } = require('help/config');
module.exports = simple('ai_kpi', 'Run scenario batches and emit KPI reports', 'Usage: ai_kpi [options]', {
  allowPositionals: true,
    options: {
        scenarios: { type: 'string', short: 's', description: 'Scenario file path (JSONL or plain text lines)' },
        out: { type: 'string', short: 'o', description: 'Output report path', default: 'ai-kpi-report.json' },
        outNdjson: { type: 'string', description: 'Optional NDJSON output path for per-scenario entries' },
        outCsv: { type: 'string', description: 'Optional CSV output path for per-scenario entries' },
        provider: { type: 'string', short: 'p', description: 'LLM provider override' },
        model: { type: 'string', short: 'm', description: 'LLM model override' },
        timeout: { type: 'string', description: 'Execution timeout in ms (default: 30000)' },
        maxRows: { type: 'string', description: 'Max query rows (default: 1000)' },
        maxOutputBytes: { type: 'string', description: 'Max execution output bytes (default: 65536)' },
        noExec: { type: 'boolean', description: 'Disable runnable block execution', default: false },
        dryRun: { type: 'boolean', description: 'Do not call provider; only parse and report scenario metadata', default: false },
    },
    longDescription: `
Scenario file format:
  - JSON line: {"id":"s1","prompt":"..."}
  - Plain line: prompt text`,
});