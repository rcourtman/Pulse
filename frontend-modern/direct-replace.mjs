#!/usr/bin/env node
/**
 * Design System Codemod: Direct single-class → token replacement.
 *
 * Replaces standalone extreme-shade grays in STATIC class="..." attributes
 * (no template literals, no ternaries) with semantic tokens.
 *
 * This is safe because in a static class string, a standalone gray like
 * "text-slate-700" is unambiguously body text, "bg-slate-100" is a subtle
 * surface, etc. — there's no conditional logic to second-guess.
 *
 * Usage:
 *   node direct-replace.mjs              # Dry run
 *   node direct-replace.mjs --write      # Apply
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, 'src');
const dryRun = !process.argv.includes('--write');

// Direct single-class replacements.
// ONLY applied inside static class="..." attributes (no ${} interpolation).
// Order: more specific first to avoid partial matches.
const DIRECT_MAP = [
    // ── Backgrounds ──
    // bg-white is a surface
    ['bg-white', 'bg-surface'],
    // Light subtle surfaces
    ['bg-slate-50', 'bg-surface-hover'],
    ['bg-slate-100', 'bg-surface-alt'],
    ['bg-slate-200', 'bg-surface-alt'],
    ['bg-gray-50', 'bg-surface-hover'],
    ['bg-gray-100', 'bg-surface-alt'],
    // Dark surfaces (standalone = dark-mode-only component or chart bg)
    ['bg-slate-800', 'bg-surface'],
    ['bg-slate-900', 'bg-base'],
    ['bg-slate-700', 'bg-surface-hover'],
    ['bg-gray-800', 'bg-surface'],
    ['bg-gray-900', 'bg-base'],

    // ── Text ──
    // Dark text = base content
    ['text-slate-900', 'text-base-content'],
    ['text-slate-800', 'text-base-content'],
    ['text-slate-700', 'text-base-content'],
    ['text-gray-900', 'text-base-content'],
    ['text-gray-800', 'text-base-content'],
    ['text-gray-700', 'text-base-content'],
    // Light text (used in dark contexts) = base content
    ['text-slate-100', 'text-base-content'],
    ['text-slate-200', 'text-base-content'],
    ['text-gray-100', 'text-base-content'],
    ['text-gray-200', 'text-base-content'],

    // ── Borders ──
    ['border-slate-200', 'border-border'],
    ['border-slate-100', 'border-border-subtle'],
    ['border-slate-700', 'border-border'],
    ['border-slate-800', 'border-border-subtle'],
    ['border-gray-200', 'border-border'],
    ['border-gray-100', 'border-border-subtle'],
    ['border-gray-700', 'border-border'],

    // ── Ring ──
    ['ring-slate-200', 'ring-border'],
    ['ring-slate-700', 'ring-border'],
    ['ring-gray-200', 'ring-border'],
];

function escapeRegex(str) {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

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

    // Only process static class="..." attributes (not class={`...`} templates)
    content = content.replace(/class="([^"]+)"/g, (fullMatch, classValue) => {
        // Skip if it contains interpolation markers (shouldn't in class="..." but be safe)
        if (classValue.includes('${')) return fullMatch;

        let modified = classValue;
        for (const [from, to] of DIRECT_MAP) {
            // Match the class as a whole word (bounded by spaces or string edges)
            const pattern = new RegExp(`(?<=^|\\s)${escapeRegex(from)}(?=\\s|$)`, 'g');
            const matches = modified.match(pattern);
            if (matches) {
                modified = modified.replace(pattern, to);
                fileReplacements += matches.length;
            }
        }

        if (modified !== classValue) {
            return `class="${modified}"`;
        }
        return fullMatch;
    });

    if (fileReplacements > 0) {
        const relPath = path.relative(__dirname, file);
        console.log(`  ${relPath}: ${fileReplacements} direct replacements`);
        totalReplacements += fileReplacements;
        filesModified++;

        if (!dryRun) {
            fs.writeFileSync(file, content, 'utf-8');
        }
    }
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${totalReplacements} direct replacements across ${filesModified} files`);
if (dryRun && totalReplacements > 0) {
    console.log('Run with --write to apply changes.');
}
