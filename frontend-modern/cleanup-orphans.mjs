#!/usr/bin/env node
/**
 * Design System Cleanup: Remove orphaned dark: overrides.
 *
 * After migrating paired gray classes → semantic tokens, many class strings
 * now contain BOTH a semantic token AND an orphaned dark: override that does
 * nothing (because the token already handles dark mode via CSS variables).
 *
 * Example:
 *   Before: "text-base-content dark:text-slate-300"
 *   After:  "text-base-content"
 *
 * This codemod removes those orphaned dark: classes safely.
 *
 * Usage:
 *   node cleanup-orphans.mjs              # Dry run
 *   node cleanup-orphans.mjs --write      # Apply
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, 'src');

const dryRun = !process.argv.includes('--write');

// ── Token → property mapping ──
// Each semantic token "owns" a CSS property. Any dark: class targeting the
// same property on the same element is redundant.
const TOKEN_OWNS = {
    // Background tokens
    'bg-surface': /\bdark:bg-slate-\d+\b/g,
    'bg-surface-hover': /\bdark:bg-slate-\d+\b/g,
    'bg-surface-alt': /\bdark:bg-slate-\d+\b/g,
    'bg-base': /\bdark:bg-slate-\d+\b/g,
    // Hover background tokens
    'hover:bg-surface-hover': /\bdark:hover:bg-slate-\d+\b/g,
    // Text tokens
    'text-base-content': /\bdark:text-(?:slate|gray|zinc|neutral|white)-?\d*\b/g,
    'text-muted': /\bdark:text-(?:slate|gray|zinc|neutral)-\d+\b/g,
    // Hover text tokens
    'hover:text-base-content': /\bdark:hover:text-(?:slate|gray)-\d+\b/g,
    // Border tokens
    'border-border': /\bdark:border-(?:slate|gray)-\d+\b/g,
    'border-border-subtle': /\bdark:border-(?:slate|gray)-\d+\b/g,
    // Divider tokens
    'divide-border': /\bdark:divide-(?:slate|gray)-\d+\b/g,
    // Ring tokens
    'ring-border': /\bdark:ring-(?:slate|gray)-\d+\b/g,
    'focus:ring-border': /\bdark:focus:ring-(?:slate|gray)-\d+\b/g,
};

// Also strip orphaned LIGHT-mode halves when a semantic token is present.
// E.g., "bg-white bg-surface" → "bg-surface" (bg-white is redundant)
const TOKEN_OWNS_LIGHT = {
    'bg-surface': /\bbg-white\b/g,
    'bg-surface-hover': /\bbg-(?:slate|gray)-(?:50|100)\b/g,
    'bg-surface-alt': /\bbg-(?:slate|gray)-(?:50|100)\b/g,
    'bg-base': /\bbg-(?:slate|gray)-(?:50|100)\b/g,
    'text-base-content': /\btext-(?:slate|gray)-(?:700|800|900)\b/g,
    'text-muted': /\btext-(?:slate|gray)-(?:400|500|600)\b/g,
    'border-border': /\bborder-(?:slate|gray)-(?:200|300)\b/g,
    'border-border-subtle': /\bborder-(?:slate|gray)-(?:100)\b/g,
};

function findTsxFiles(dir) {
    const results = [];
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory() && entry.name !== 'node_modules') {
            results.push(...findTsxFiles(full));
        } else if (entry.isFile() && (entry.name.endsWith('.tsx') || entry.name.endsWith('.ts'))) {
            results.push(full);
        }
    }
    return results;
}

// Process a single class-string value.
// Returns [cleaned, removedCount].
function cleanClassString(classStr) {
    let cleaned = classStr;
    let removed = 0;

    for (const [token, orphanPattern] of Object.entries(TOKEN_OWNS)) {
        if (cleaned.includes(token)) {
            const before = cleaned;
            cleaned = cleaned.replace(orphanPattern, '');
            if (cleaned !== before) {
                removed += (before.match(orphanPattern) || []).length;
            }
        }
    }

    for (const [token, orphanPattern] of Object.entries(TOKEN_OWNS_LIGHT)) {
        if (cleaned.includes(token)) {
            const before = cleaned;
            cleaned = cleaned.replace(orphanPattern, '');
            if (cleaned !== before) {
                removed += (before.match(orphanPattern) || []).length;
            }
        }
    }

    // Collapse multiple spaces
    cleaned = cleaned.replace(/  +/g, ' ').trim();
    return [cleaned, removed];
}

const files = findTsxFiles(SRC);
let totalRemovals = 0;
let filesModified = 0;

for (const file of files) {
    let content = fs.readFileSync(file, 'utf-8');
    let original = content;
    let fileRemovals = 0;

    // Match class="..." and class={`...`} strings (simplified but effective)
    // Process quoted class attributes: class="..."
    content = content.replace(/class="([^"]+)"/g, (match, classes) => {
        const [cleaned, removed] = cleanClassString(classes);
        fileRemovals += removed;
        return `class="${cleaned}"`;
    });

    // Process template literal class attributes: class={`...`}
    // Only handle simple cases (no nested expressions)
    content = content.replace(/class=\{`([^`]+)`\}/g, (match, classes) => {
        const [cleaned, removed] = cleanClassString(classes);
        fileRemovals += removed;
        return `class={\`${cleaned}\`}`;
    });

    // Process string literals in conditionals: 'classes...'
    // Match single-quoted strings that contain both a token and a dark: class
    content = content.replace(/'([^']{10,})'/g, (match, classes) => {
        if (!classes.includes('dark:') && !Object.keys(TOKEN_OWNS_LIGHT).some(t => classes.includes(t))) return match;
        const [cleaned, removed] = cleanClassString(classes);
        if (removed === 0) return match;
        fileRemovals += removed;
        return `'${cleaned}'`;
    });

    if (fileRemovals > 0) {
        const relPath = path.relative(__dirname, file);
        console.log(`  ${relPath}: ${fileRemovals} orphans removed`);
        totalRemovals += fileRemovals;
        filesModified++;

        if (!dryRun) {
            fs.writeFileSync(file, content, 'utf-8');
        }
    }
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${totalRemovals} orphaned classes removed across ${filesModified} files`);
if (dryRun && totalRemovals > 0) {
    console.log('Run with --write to apply changes.');
}
