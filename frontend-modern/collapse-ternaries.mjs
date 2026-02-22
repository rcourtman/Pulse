#!/usr/bin/env node
/**
 * Design System Codemod: Collapse runtime dark-mode ternaries into semantic tokens.
 *
 * Finds patterns like:
 *   isDark() ? 'bg-slate-800' : 'bg-white'         → 'bg-surface'
 *   isDark() ? 'text-slate-300' : 'text-slate-700'  → 'text-base-content'
 *   isDark() ? 'border-slate-700' : 'border-slate-200' → 'border-border'
 *
 * These are runtime dark-mode switches that can be replaced with CSS-variable tokens.
 *
 * Usage:
 *   node collapse-ternaries.mjs              # Dry run
 *   node collapse-ternaries.mjs --write      # Apply
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, 'src');
const dryRun = !process.argv.includes('--write');

// Ternary collapse rules: [darkValue, lightValue] → token
const TERNARY_RULES = [
    // bg surfaces
    [/bg-slate-(?:800|900)/, /bg-(?:white|slate-(?:50|100))/, 'bg-surface'],
    [/bg-slate-700/, /bg-slate-(?:50|100|200)/, 'bg-surface-hover'],
    [/bg-slate-800/, /bg-slate-(?:100|200)/, 'bg-surface-alt'],
    // text
    [/text-slate-(?:100|200|300)/, /text-slate-(?:700|800|900)/, 'text-base-content'],
    [/text-(?:white|slate-100)/, /text-slate-(?:800|900)/, 'text-base-content'],
    [/text-slate-(?:300|400)/, /text-slate-(?:500|600)/, 'text-muted'],
    [/text-slate-400/, /text-slate-(?:500|600|700)/, 'text-muted'],
    // borders
    [/border-slate-(?:600|700)/, /border-slate-(?:200|300)/, 'border-border'],
    [/border-slate-800/, /border-slate-100/, 'border-border-subtle'],
];

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

const files = findTsxFiles(SRC);
let totalReplacements = 0;
let filesModified = 0;

for (const file of files) {
    let content = fs.readFileSync(file, 'utf-8');
    let fileReplacements = 0;

    for (const [darkRe, lightRe, token] of TERNARY_RULES) {
        // Match: isDark()/isDarkMode()/darkMode/etc ? 'dark-class' : 'light-class'
        // Also handles the reverse: condition ? 'light' : 'dark'
        const darkStr = darkRe.source;
        const lightStr = lightRe.source;

        // Pattern 1: isDark ? 'dark-value' : 'light-value'
        const p1 = new RegExp(
            `(isDark[A-Za-z()]*\\s*\\?\\s*)'(${darkStr})'\\s*:\\s*'(${lightStr})'`,
            'g'
        );

        // Pattern 2: isDark ? 'light-value' : 'dark-value' (inverted condition, less common)
        // Actually this would be !isDark, skip for safety

        // Pattern 3: Using template literals
        const p3 = new RegExp(
            `\\$\\{\\s*isDark[A-Za-z()]*\\s*\\?\\s*'(${darkStr})'\\s*:\\s*'(${lightStr})'\\s*\\}`,
            'g'
        );

        const before = content;
        content = content.replace(p1, `'${token}'/* was ternary dark-switch */ ? '${token}' : '${token}'`);
        // Actually, that's wrong - we need to replace the WHOLE ternary, not just the values
        content = before; // revert

        // Better approach: just replace the whole matched expression with the token string
        const fullP1 = new RegExp(
            `isDark[A-Za-z()]*\\s*\\?\\s*'(${darkStr})'\\s*:\\s*'(${lightStr})'`,
            'g'
        );
        const matches1 = content.match(fullP1);
        if (matches1) {
            content = content.replace(fullP1, `'${token}'`);
            fileReplacements += matches1.length;
        }

        const fullP3 = new RegExp(
            `\\$\\{\\s*isDark[A-Za-z()]*\\s*\\?\\s*'(${darkStr})'\\s*:\\s*'(${lightStr})'\\s*\\}`,
            'g'
        );
        const matches3 = content.match(fullP3);
        if (matches3) {
            content = content.replace(fullP3, token);
            fileReplacements += matches3.length;
        }
    }

    if (fileReplacements > 0) {
        const relPath = path.relative(__dirname, file);
        console.log(`  ${relPath}: ${fileReplacements} ternaries collapsed`);
        totalReplacements += fileReplacements;
        filesModified++;

        if (!dryRun) {
            fs.writeFileSync(file, content, 'utf-8');
        }
    }
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${totalReplacements} ternaries collapsed across ${filesModified} files`);
if (dryRun && totalReplacements > 0) {
    console.log('Run with --write to apply changes.');
}
