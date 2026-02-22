#!/usr/bin/env node
/**
 * Design System Codemod: Migrate hardcoded Tailwind grays → semantic tokens.
 * 
 * This performs safe, mechanical replacements of known gray patterns.
 * It's deliberately conservative — it only replaces patterns where
 * the mapping is unambiguous. Manual review is still needed for
 * context-specific cases.
 * 
 * Usage:
 *   node migrate-tokens.mjs                  # Dry run (preview changes)
 *   node migrate-tokens.mjs --write          # Apply changes
 *   node migrate-tokens.mjs --write --file src/pages/Alerts.tsx  # Single file
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, 'src');

const dryRun = !process.argv.includes('--write');
const singleFileArg = process.argv.indexOf('--file');
const singleFile = singleFileArg !== -1 ? process.argv[singleFileArg + 1] : null;

// ── Replacement Map ──
// Each entry: [pattern, replacement, description]
// Order matters — more specific patterns first.
const REPLACEMENTS = [
    // ─── Backgrounds ───
    // Surface (cards, panels, modals, dropdowns)
    ['bg-white dark:bg-slate-800', 'bg-surface', 'surface bg'],
    ['bg-white dark:bg-gray-800', 'bg-surface', 'surface bg'],
    ['bg-white dark:bg-zinc-800', 'bg-surface', 'surface bg'],
    ['bg-white dark:bg-neutral-800', 'bg-surface', 'surface bg'],
    ['bg-white dark:bg-slate-900', 'bg-surface', 'surface bg (deep dark)'],
    // Hover surface
    ['bg-slate-50 dark:bg-slate-700', 'bg-surface-hover', 'hover surface'],
    ['bg-gray-50 dark:bg-gray-700', 'bg-surface-hover', 'hover surface'],
    ['hover:bg-slate-50 dark:hover:bg-slate-700', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-gray-50 dark:hover:bg-gray-700', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-slate-100 dark:hover:bg-slate-700', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-gray-100 dark:hover:bg-gray-700', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-slate-50 dark:hover:bg-slate-800', 'hover:bg-surface-hover', 'hover surface (deep)'],
    ['hover:bg-slate-100 dark:hover:bg-slate-800', 'hover:bg-surface-hover', 'hover surface (deep)'],
    // Base background
    ['bg-slate-100 dark:bg-slate-900', 'bg-base', 'base bg'],
    ['bg-gray-100 dark:bg-gray-900', 'bg-base', 'base bg'],
    ['bg-slate-50 dark:bg-slate-900', 'bg-base', 'base bg'],
    ['bg-gray-50 dark:bg-gray-900', 'bg-base', 'base bg'],
    // Surface alternatives
    ['bg-slate-100 dark:bg-slate-800', 'bg-surface-alt', 'surface alt bg'],
    ['bg-gray-100 dark:bg-gray-800', 'bg-surface-alt', 'surface alt bg'],
    ['bg-slate-50 dark:bg-slate-800', 'bg-surface-alt', 'surface alt bg'],
    ['bg-gray-50 dark:bg-gray-800', 'bg-surface-alt', 'surface alt bg'],
    ['bg-slate-100 dark:bg-slate-700', 'bg-surface-hover', 'surface hover bg'],
    ['bg-slate-200 dark:bg-slate-700', 'bg-surface-hover', 'surface hover bg (stronger)'],
    ['bg-slate-200 dark:bg-slate-600', 'bg-surface-hover', 'surface hover bg (strong)'],
    // Active/selected states
    ['bg-slate-200 dark:bg-slate-800', 'bg-surface-alt', 'surface alt bg (selected)'],

    // ─── Borders ───
    ['border-slate-200 dark:border-slate-700', 'border-border', 'standard border'],
    ['border-gray-200 dark:border-gray-700', 'border-border', 'standard border'],
    ['border-slate-200 dark:border-slate-600', 'border-border', 'standard border'],
    ['border-gray-200 dark:border-gray-600', 'border-border', 'standard border'],
    ['border-slate-300 dark:border-slate-600', 'border-border', 'input border'],
    ['border-gray-300 dark:border-gray-600', 'border-border', 'input border'],
    ['border-slate-300 dark:border-slate-700', 'border-border', 'input border'],
    ['border-gray-300 dark:border-gray-700', 'border-border', 'input border'],
    ['border-slate-100 dark:border-slate-800', 'border-border-subtle', 'subtle border'],
    ['border-gray-100 dark:border-gray-800', 'border-border-subtle', 'subtle border'],
    // Dividers
    ['divide-slate-200 dark:divide-slate-700', 'divide-border', 'divider'],
    ['divide-gray-200 dark:divide-gray-700', 'divide-border', 'divider'],
    ['divide-slate-100 dark:divide-slate-800', 'divide-border', 'divider subtle'],

    // ─── Text ───
    // Base text (headings, body)
    ['text-slate-900 dark:text-slate-100', 'text-base-content', 'base text'],
    ['text-gray-900 dark:text-gray-100', 'text-base-content', 'base text'],
    ['text-slate-900 dark:text-white', 'text-base-content', 'base text'],
    ['text-gray-900 dark:text-white', 'text-base-content', 'base text'],
    ['text-slate-800 dark:text-slate-200', 'text-base-content', 'base text'],
    ['text-gray-800 dark:text-gray-200', 'text-base-content', 'base text'],
    ['text-slate-800 dark:text-slate-100', 'text-base-content', 'base text'],
    ['text-slate-800 dark:text-white', 'text-base-content', 'base text'],
    ['text-slate-700 dark:text-slate-200', 'text-base-content', 'base text'],
    ['text-slate-700 dark:text-slate-300', 'text-base-content', 'base text'],
    ['text-gray-700 dark:text-gray-200', 'text-base-content', 'base text'],
    ['text-gray-700 dark:text-gray-300', 'text-base-content', 'base text'],
    // Muted text (descriptions, secondary)
    ['text-slate-500 dark:text-slate-400', 'text-muted', 'muted text'],
    ['text-gray-500 dark:text-gray-400', 'text-muted', 'muted text'],
    ['text-slate-600 dark:text-slate-400', 'text-muted', 'muted text'],
    ['text-gray-600 dark:text-gray-400', 'text-muted', 'muted text'],
    ['text-slate-400 dark:text-slate-500', 'text-muted', 'muted text'],
    ['text-gray-400 dark:text-gray-500', 'text-muted', 'muted text'],
    ['text-slate-600 dark:text-slate-300', 'text-muted', 'muted text'],
    ['text-slate-500 dark:text-slate-300', 'text-muted', 'muted text'],
    ['text-slate-600 dark:text-slate-200', 'text-muted', 'muted text'],
    ['text-slate-700 dark:text-slate-400', 'text-muted', 'muted text (body→muted)'],
    ['text-slate-800 dark:text-slate-400', 'text-muted', 'muted text'],
    ['text-slate-500 dark:text-slate-500', 'text-muted', 'muted text (same shade)'],
    ['text-slate-900 dark:text-slate-50', 'text-base-content', 'base text (extreme)'],
    ['text-slate-900 dark:text-slate-300', 'text-base-content', 'base text'],
    ['text-slate-900 dark:text-slate-400', 'text-base-content', 'base text'],
    ['text-slate-700 dark:text-slate-100', 'text-base-content', 'base text'],
    ['text-slate-800 dark:text-slate-300', 'text-base-content', 'base text'],
    ['text-slate-600 dark:text-slate-500', 'text-muted', 'muted text'],
    ['text-slate-400 dark:text-slate-600', 'text-muted', 'muted text (inverted)'],
    ['text-slate-300 dark:text-slate-600', 'text-muted', 'muted text (inverted)'],
    ['text-slate-200 dark:text-slate-700', 'text-muted', 'muted text (inverted)'],
    ['text-slate-300 dark:text-slate-700', 'text-muted', 'muted text (inverted)'],
    // Placeholder text
    ['placeholder-slate-400 dark:placeholder-slate-500', 'placeholder-muted', 'placeholder'],
    ['placeholder-gray-400 dark:placeholder-gray-500', 'placeholder-muted', 'placeholder'],

    // ─── Ring ───
    ['ring-slate-200 dark:ring-slate-700', 'ring-border', 'ring'],
    ['ring-gray-200 dark:ring-gray-700', 'ring-border', 'ring'],
    ['focus:ring-slate-300 dark:focus:ring-slate-600', 'focus:ring-border', 'focus ring'],
    ['focus:ring-gray-300 dark:focus:ring-gray-600', 'focus:ring-border', 'focus ring'],

    // ─── Hover text ───
    ['hover:text-slate-900 dark:hover:text-slate-100', 'hover:text-base-content', 'hover text'],
    ['hover:text-slate-900 dark:hover:text-white', 'hover:text-base-content', 'hover text'],
    ['hover:text-slate-800 dark:hover:text-slate-100', 'hover:text-base-content', 'hover text'],
    ['hover:text-slate-700 dark:hover:text-slate-200', 'hover:text-base-content', 'hover text'],
    ['hover:text-slate-900 dark:hover:text-slate-300', 'hover:text-base-content', 'hover text'],

    // ─── Additional borders (wave 3) ───
    ['border-slate-100 dark:border-slate-700', 'border-border-subtle', 'subtle border'],
    ['border-slate-200 dark:border-slate-800', 'border-border', 'border (alt shades)'],

    // ─── Additional backgrounds (wave 3) ───
    ['bg-slate-900 dark:bg-slate-800', 'bg-base', 'base bg (deep)'],
    ['bg-slate-300 dark:bg-slate-600', 'bg-surface-hover', 'hover surface (strong)'],
    ['bg-slate-300 dark:bg-slate-700', 'bg-surface-hover', 'hover surface (strong)'],

    // ─── Hover backgrounds (wave 3) ───
    ['hover:bg-slate-200 dark:hover:bg-slate-600', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-slate-200 dark:hover:bg-slate-700', 'hover:bg-surface-hover', 'hover surface'],
    ['hover:bg-slate-100 dark:hover:bg-slate-600', 'hover:bg-surface-hover', 'hover surface'],
];

// ── Find all .tsx files ──
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

// ── Process ──
const files = singleFile
    ? [path.resolve(singleFile)]
    : findTsxFiles(SRC);

let totalReplacements = 0;
let filesModified = 0;

for (const file of files) {
    let content = fs.readFileSync(file, 'utf-8');
    let fileReplacements = 0;
    let original = content;

    for (const [pattern, replacement] of REPLACEMENTS) {
        // Match the pattern as a whole-string segment within class strings
        const regex = new RegExp(escapeRegex(pattern), 'g');
        const matches = content.match(regex);
        if (matches) {
            content = content.replaceAll(pattern, replacement);
            fileReplacements += matches.length;
        }
    }

    if (fileReplacements > 0) {
        const relPath = path.relative(path.join(__dirname), file);
        console.log(`  ${relPath}: ${fileReplacements} replacements`);
        totalReplacements += fileReplacements;
        filesModified++;

        if (!dryRun) {
            fs.writeFileSync(file, content, 'utf-8');
        }
    }
}

function escapeRegex(str) {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${totalReplacements} replacements across ${filesModified} files`);
if (dryRun && totalReplacements > 0) {
    console.log('Run with --write to apply changes.');
}
