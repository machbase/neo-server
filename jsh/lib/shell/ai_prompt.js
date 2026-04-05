'use strict';

// ai/prompt.js — System prompt assembler for the ai command.
//
// Loads named segments from Go (ai.loadSegment) and assembles them
// into a single system prompt string. Custom segments from
// $HOME/.config/machbase/llm/prompts/ take priority over builtins.
//
// Usage:
//   const { buildSystemPrompt, listSegments } = require('ai/prompt');
//   const prompt = buildSystemPrompt(['jsh-runtime', 'jsh-modules', 'agent-api'], extraContext);

const { ai } = require('@jsh/shell');

/**
 * Returns the list of all available segment names (builtin + custom).
 * @returns {string[]}
 */
function listSegments() {
    return ai.listSegments();
}

/**
 * Assembles a system prompt by concatenating the named segments.
 * Segments that cannot be found are silently skipped.
 * An optional extraContext string (e.g. DB schema) is appended last.
 *
 * @param {string[]} segmentNames
 * @param {string}   [extraContext]
 * @returns {string}
 */
function buildSystemPrompt(segmentNames, extraContext) {
    var parts = [];
    for (var i = 0; i < segmentNames.length; i++) {
        var name = segmentNames[i];
        try {
            var content = ai.loadSegment(name);
            if (content) {
                parts.push('## ' + name + '\n\n' + content.trim());
            }
        } catch (_) {
            // skip missing segments silently
        }
    }
    if (extraContext && extraContext.trim()) {
        parts.push('## context\n\n' + extraContext.trim());
    }
    return parts.join('\n\n---\n\n');
}

module.exports = {
    listSegments: listSegments,
    buildSystemPrompt: buildSystemPrompt,
};
