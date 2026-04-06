'use strict';

const fs = require('fs');
const path = require('path');
const process = require('process');

function formatSavedAt(date) {
    var year = date.getFullYear();
    var month = String(date.getMonth() + 1).padStart(2, '0');
    var day = String(date.getDate()).padStart(2, '0');
    var hour = String(date.getHours()).padStart(2, '0');
    var minute = String(date.getMinutes()).padStart(2, '0');
    var second = String(date.getSeconds()).padStart(2, '0');
    var offsetMinutes = -date.getTimezoneOffset();
    var sign = offsetMinutes >= 0 ? '+' : '-';
    var offsetHour = String(Math.floor(Math.abs(offsetMinutes) / 60)).padStart(2, '0');
    var offsetMinute = String(Math.abs(offsetMinutes) % 60).padStart(2, '0');
    return year + '-' + month + '-' + day + 'T' + hour + ':' + minute + ':' + second +
        sign + offsetHour + ':' + offsetMinute;
}

function normalizeSavePath(input, cwd) {
    var target = (input || '').trim();
    if (!target) {
        throw new Error('Usage: \\save <file_path>');
    }
    return path.resolve(cwd || process.cwd(), target);
}

function markdownRoleHeading(role) {
    if (role === 'user') {
        return 'User';
    }
    if (role === 'assistant') {
        return 'Assistant';
    }
    if (!role) {
        return 'Message';
    }
    var roleText = String(role);
    return roleText.charAt(0).toUpperCase() + roleText.slice(1);
}

function countTurns(entries) {
    var turns = 0;
    for (var i = 0; i < entries.length; i++) {
        if (entries[i] && entries[i].role === 'user') {
            turns++;
        }
    }
    return turns;
}

function renderSessionMarkdown(entries, meta) {
    var promptSegments = meta.promptSegments || [];
    var lines = [
        '# AI Session',
        '',
        '- Saved at: ' + meta.savedAt,
        '- Provider: ' + meta.provider,
        '- Model: ' + meta.model,
        '- Prompt segments: ' + (promptSegments.length ? promptSegments.join(', ') : '(none)'),
        '- Turns: ' + meta.turns,
        '',
        '---',
        ''
    ];

    if (!entries.length) {
        lines.push('_No conversation history saved._', '');
        return lines.join('\n');
    }

    for (var i = 0; i < entries.length; i++) {
        var entry = entries[i] || {};
        lines.push('## ' + markdownRoleHeading(entry.role), '');
        lines.push(String(entry.content || ''));
        lines.push('');
    }

    return lines.join('\n');
}

function saveTranscript(targetPathInput, session) {
    session = session || {};
    var history = session.history || [];
    var turns = countTurns(history);
    var targetPath = normalizeSavePath(targetPathInput, session.cwd);
    var markdown = renderSessionMarkdown(history, {
        savedAt: session.savedAt || formatSavedAt(new Date()),
        provider: session.provider || 'unknown',
        model: session.model || 'unknown',
        promptSegments: (session.promptSegments || []).slice(),
        turns: turns,
    });

    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    fs.writeFileSync(targetPath, markdown, 'utf8');

    return {
        path: targetPath,
        turns: turns,
    };
}

module.exports = {
    countTurns: countTurns,
    formatSavedAt: formatSavedAt,
    normalizeSavePath: normalizeSavePath,
    renderSessionMarkdown: renderSessionMarkdown,
    saveTranscript: saveTranscript,
};
