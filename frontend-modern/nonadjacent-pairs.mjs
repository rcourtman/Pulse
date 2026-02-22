#!/usr/bin/env node
/**
 * Design System Codemod: Non-adjacent pair replacement.
 *
 * The pair codemod (migrate-tokens.mjs) only matches ADJACENT pairs like:
 *   "bg-slate-100 dark:bg-slate-800" → "bg-surface-alt"
 *
 * But many real class strings have the pairs SEPARATED by other classes:
 *   "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
 *
 * This codemod finds and replaces both halves independently, converting:
 *   "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
 *   → "bg-surface-alt text-base-content"
 *
 * Usage:
 *   node nonadjacent-pairs.mjs              # Dry run
 *   node nonadjacent-pairs.mjs --write      # Apply
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, 'src');
const dryRun = !process.argv.includes('--write');

// Non-adjacent pair rules: [lightClass, darkClass, token]
// When BOTH appear in the same class string (even non-adjacent), replace both with the token.
const PAIR_RULES = [
    // bg
    ['bg-white', 'dark:bg-slate-800', 'bg-surface'],
    ['bg-white', 'dark:bg-slate-900', 'bg-surface'],
    ['bg-white', 'dark:bg-gray-800', 'bg-surface'],
    ['bg-slate-50', 'dark:bg-slate-700', 'bg-surface-hover'],
    ['bg-slate-50', 'dark:bg-slate-800', 'bg-surface-alt'],
    ['bg-slate-50', 'dark:bg-slate-900', 'bg-base'],
    ['bg-slate-100', 'dark:bg-slate-800', 'bg-surface-alt'],
    ['bg-slate-100', 'dark:bg-slate-700', 'bg-surface-hover'],
    ['bg-slate-100', 'dark:bg-slate-900', 'bg-base'],
    ['bg-slate-200', 'dark:bg-slate-700', 'bg-surface-hover'],
    ['bg-slate-200', 'dark:bg-slate-800', 'bg-surface-alt'],
    ['bg-gray-100', 'dark:bg-gray-800', 'bg-surface-alt'],
    ['bg-gray-100', 'dark:bg-gray-700', 'bg-surface-hover'],
    ['bg-gray-200', 'dark:bg-gray-700', 'bg-surface-hover'],
    // text
    ['text-slate-900', 'dark:text-slate-100', 'text-base-content'],
    ['text-slate-900', 'dark:text-slate-50', 'text-base-content'],
    ['text-slate-900', 'dark:text-white', 'text-base-content'],
    ['text-slate-900', 'dark:text-slate-200', 'text-base-content'],
    ['text-slate-900', 'dark:text-slate-300', 'text-base-content'],
    ['text-slate-800', 'dark:text-slate-200', 'text-base-content'],
    ['text-slate-800', 'dark:text-slate-100', 'text-base-content'],
    ['text-slate-800', 'dark:text-slate-300', 'text-base-content'],
    ['text-slate-800', 'dark:text-white', 'text-base-content'],
    ['text-slate-700', 'dark:text-slate-200', 'text-base-content'],
    ['text-slate-700', 'dark:text-slate-300', 'text-base-content'],
    ['text-slate-700', 'dark:text-slate-100', 'text-base-content'],
    ['text-slate-700', 'dark:text-white', 'text-base-content'],
    ['text-gray-900', 'dark:text-gray-100', 'text-base-content'],
    ['text-gray-800', 'dark:text-gray-200', 'text-base-content'],
    ['text-gray-700', 'dark:text-gray-200', 'text-base-content'],
    ['text-gray-700', 'dark:text-gray-300', 'text-base-content'],
    // muted text
    ['text-slate-600', 'dark:text-slate-400', 'text-muted'],
    ['text-slate-600', 'dark:text-slate-300', 'text-muted'],
    ['text-slate-500', 'dark:text-slate-400', 'text-muted'],
    ['text-slate-700', 'dark:text-slate-400', 'text-muted'],
    ['text-slate-800', 'dark:text-slate-400', 'text-muted'],
    ['text-gray-700', 'dark:hover:text-gray-100', 'text-base-content'],
    // borders
    ['border-slate-200', 'dark:border-slate-700', 'border-border'],
    ['border-slate-200', 'dark:border-slate-600', 'border-border'],
    ['border-slate-300', 'dark:border-slate-600', 'border-border'],
    ['border-slate-300', 'dark:border-slate-700', 'border-border'],
    ['border-slate-100', 'dark:border-slate-700', 'border-border-subtle'],
    ['border-slate-100', 'dark:border-slate-800', 'border-border-subtle'],
    // hover bg
    ['hover:bg-slate-50', 'dark:hover:bg-slate-700', 'hover:bg-surface-hover'],
    ['hover:bg-slate-100', 'dark:hover:bg-slate-700', 'hover:bg-surface-hover'],
    ['hover:bg-slate-50', 'dark:hover:bg-slate-800', 'hover:bg-surface-hover'],
    ['hover:bg-slate-100', 'dark:hover:bg-slate-800', 'hover:bg-surface-hover'],
    // hover text
    ['hover:text-slate-900', 'dark:hover:text-slate-100', 'hover:text-base-content'],
    ['hover:text-gray-900', 'dark:hover:text-gray-100', 'hover:text-base-content'],
    // hover borders
    ['hover:border-slate-300', 'dark:hover:border-slate-600', 'hover:border-border'],
];

function escapeRegex(str) {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function processClassString(classStr) {
    let modified = classStr;
    let removed = 0;
    const classes = modified.split(/\s+/);

    for (const [lightCls, darkCls, token] of PAIR_RULES) {
        const hasLight = classes.some(c => c === lightCls);
        const hasDark = classes.some(c => c === darkCls);

        if (hasLight && hasDark) {
            // Replace light class with token, remove dark class
            const lightRe = new RegExp(`(?<=^|\\s)${escapeRegex(lightCls)}(?=\\s|$)`);
            const darkRe = new RegExp(`(?<=^|\\s)${escapeRegex(darkCls)}(?=\\s|$)`);
            modified = modified.replace(lightRe, token);
            modified = modified.replace(darkRe, '');
            removed++;
            // Refresh classes array
            // (we don't re-split to avoid complexity, just continue)
        }
    }

    // Clean up double spaces
    modified = modified.replace(/\s{2,}/g, ' ').trim();
    return [modified, removed];
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

    // Process class="..." static attributes
    content = content.replace(/class="([^"]+)"/g, (match, cls) => {
        const [cleaned, count] = processClassString(cls);
        fileReplacements += count;
        return count > 0 ? `class="${cleaned}"` : match;
    });

    // Process template literals: class={`...`}
    content = content.replace(/class=\{`([^`]+)`\}/g, (match, cls) => {
        const [cleaned, count] = processClassString(cls);
        fileReplacements += count;
        return count > 0 ? `class={\`${cleaned}\`}` : match;
    });

    // Process single-quoted strings (ternary branches, function returns)
    content = content.replace(/'([^']{15,})'/g, (match, cls) => {
        // Only process strings that look like CSS class lists
        if (!cls.includes('bg-') && !cls.includes('text-') && !cls.includes('border-')) return match;
        const [cleaned, count] = processClassString(cls);
        fileReplacements += count;
        return count > 0 ? `'${cleaned}'` : match;
    });

    // Process double-quoted strings in JS/TS contexts (not JSX class attrs — those are handled above)
    // Target: return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
    content = content.replace(/"([^"]{15,})"/g, (match, cls) => {
        if (match.startsWith('class="')) return match; // Skip JSX class attrs (already processed)
        if (!cls.includes('bg-') && !cls.includes('text-') && !cls.includes('border-')) return match;
        const [cleaned, count] = processClassString(cls);
        fileReplacements += count;
        return count > 0 ? `"${cleaned}"` : match;
    });

    // Process template literal parts with ${} interpolation
    // Handle: `prefix ${condition} bg-slate-100 dark:bg-slate-800 suffix`
    content = content.replace(/`([^`]+)`/g, (match, inner) => {
        // Only process if it has interpolation AND gray pairs
        if (!inner.includes('${')) return match;
        if (!inner.includes('dark:')) return match;
        const [cleaned, count] = processClassString(inner);
        fileReplacements += count;
        return count > 0 ? `\`${cleaned}\`` : match;
    });

    if (fileReplacements > 0) {
        const relPath = path.relative(__dirname, file);
        console.log(`  ${relPath}: ${fileReplacements} non-adjacent pairs`);
        totalReplacements += fileReplacements;
        filesModified++;
        if (!dryRun) {
            fs.writeFileSync(file, content, 'utf-8');
        }
    }
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${totalReplacements} non-adjacent pairs across ${filesModified} files`);
if (dryRun && totalReplacements > 0) {
    console.log('Run with --write to apply changes.');
}
