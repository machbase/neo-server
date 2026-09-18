'use strict';
const { simple } = require('help/config');
module.exports = simple('ai', 'Interactive LLM chat with machbase-neo context', 'Usage: ai [options] [prompt]', {
    options: {
        eval: { type: 'string', short: 'e', description: 'One-shot prompt (non-interactive, prints response and exits)' },
        provider: { type: 'string', short: 'p', description: 'LLM provider name (default: from config, e.g. "claude")' },
        model: { type: 'string', short: 'm', description: 'Model name override' },
        maxTokens: { type: 'string', description: 'Maximum response tokens (default: from config)' },
        noExec: { type: 'boolean', description: 'Disable jsh-run code execution prompts (safe mode)', default: false },
        timeout: { type: 'string', description: 'jsh code execution timeout in ms (default: 30000)' },
        maxRows: { type: 'string', description: 'Query max rows (default: 1000)' },
        out: { type: 'string', description: 'Output format: text|json (default: text)', default: 'text' },
    },
    positionals: [{ name: 'prompt', variadic: true, optional: true, description: 'Prompt text' }],
        longDescription: `
Slash commands (during interactive session, prefix with "\\" or "/"):
    /provider [name]       Show or switch active LLM provider
    /model <name>          Change model for current provider
    /prompt                List active system prompt segments
    /prompt show           Print assembled system prompt
    /prompt add <segment>  Add a prompt segment
    /prompt rm <segment>   Remove a prompt segment
    /prompt list           List all available segments
    /config show           Print config file contents
    /config set <k> <v>    Set a config value (dot-notation)
    /config edit           Edit config file in host editor
    /config path           Print config file path
    /metrics [reset]       Show or reset session KPI metrics
    /clear                 Clear conversation history
    /save <file_path>      Save the current session as Markdown (.md recommended)
    /help                  Show this help
    /bye /exit /quit       Exit`,
});