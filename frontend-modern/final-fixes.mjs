#!/usr/bin/env node
/**
 * Final manual fixes for the last 35 ESLint warnings.
 * Each fix is a targeted search-and-replace in a specific file.
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const dryRun = !process.argv.includes('--write');

// [file, search, replace]
const FIXES = [
    // #1 - Chat/index.tsx: non-adjacent pair in return string
    ['src/components/AI/Chat/index.tsx',
        "return 'border-slate-200 text-slate-600 bg-white dark:border-slate-700 dark:text-slate-200 dark:bg-slate-800';",
        "return 'border-border text-muted bg-surface';"],

    // #2,3 - FindingsPanel.tsx: fallback strings with orphaned dark:border
    ['src/components/AI/FindingsPanel.tsx',
        "'border-slate-200 bg-surface-alt dark:border-slate-700 text-muted'",
        "'border-border bg-surface-alt text-muted'"],

    // #4 - ResourceTable.tsx: the off state has bg-slate-200 — use surface-hover
    ['src/components/Alerts/ResourceTable.tsx',
        "className: 'bg-slate-200 text-muted hover:bg-surface-hover dark:hover:bg-slate-600',",
        "className: 'bg-surface-alt text-muted hover:bg-surface-hover',",],

    // #5 - ResourceTable.tsx: border in ternary
    ['src/components/Alerts/ResourceTable.tsx',
        "' border-slate-200 dark:border-slate-500'",
        "' border-border'"],

    // #6 - EnhancedCPUBar.tsx: text-slate-200 is light text on dark bg (always dark context)
    ['src/components/Dashboard/EnhancedCPUBar.tsx',
        "'text-slate-200'",
        "'text-base-content'"],

    // #7 - GuestRow.tsx: border-slate-700 in classList (dark-mode-only divider)
    ['src/components/Dashboard/GuestRow.tsx',
        "'border-t border-slate-700'",
        "'border-t border-border'"],

    // #8 - StackedDiskBar.tsx: same pattern
    ['src/components/Dashboard/StackedDiskBar.tsx',
        "'border-t border-slate-700'",
        "'border-t border-border'"],

    // #9 - ResourceDetailDrawer.tsx: non-adjacent pair in const
    ['src/components/Infrastructure/ResourceDetailDrawer.tsx',
        "'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'",
        "'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted'"],

    // #10 - resourceBadges.ts: non-adjacent pair
    ['src/components/Infrastructure/resourceBadges.ts',
        "'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'",
        "'bg-surface-alt text-base-content'"],

    // #11 - resourceBadges.ts: zinc variant → same semantic meaning
    ['src/components/Infrastructure/resourceBadges.ts',
        "'bg-zinc-100 text-zinc-700 dark:bg-zinc-900 dark:text-zinc-300'",
        "'bg-surface-alt text-base-content'"],

    // #12 - Recovery.tsx: text-slate-900 standalone in template literal
    ['src/components/Recovery/Recovery.tsx',
        'text-slate-900 ${issueTone',
        'text-base-content ${issueTone'],

    // #13 - AISettings.tsx
    ['src/components/Settings/AISettings.tsx',
        "'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'",
        "'bg-surface-alt text-base-content'"],

    // #14,15,16 - AuditLogPanel.tsx: event color badges
    ['src/components/Settings/AuditLogPanel.tsx',
        "'bg-slate-100 text-slate-800 dark:bg-slate-700 dark:text-slate-200'",
        "'bg-surface-alt text-base-content'"],

    // #17 - ConfiguredNodeTables.tsx
    ['src/components/Settings/ConfiguredNodeTables.tsx',
        "'border-slate-200 bg-slate-100 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400'",
        "'border-border bg-surface-alt text-muted'"],

    // #18 - DiagnosticsPanel.tsx
    ['src/components/Settings/DiagnosticsPanel.tsx',
        "'bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-300'",
        "'bg-surface-alt text-base-content'"],

    // #19 - OrganizationSharingPanel.tsx: text-slate-900 in template literal
    ['src/components/Settings/OrganizationSharingPanel.tsx',
        'text-sm text-slate-900 shadow-sm',
        'text-sm text-base-content shadow-sm'],

    // #20,21 - ProLicensePanel.tsx
    ['src/components/Settings/ProLicensePanel.tsx',
        "'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'",
        "'bg-surface-alt text-muted'"],

    // #22 - ResourcePicker.tsx: dark-only component, intentional — eslint-disable
    ['src/components/Settings/ResourcePicker.tsx',
        ": 'bg-slate-800 border border-slate-700 text-slate-400 hover:border-slate-500'",
        ": 'bg-surface border border-border text-muted hover:border-border' // eslint-disable-line no-restricted-syntax -- dark-mode picker"],

    // #23 - ResourcePicker.tsx: template literal with border-slate-800
    ['src/components/Settings/ResourcePicker.tsx',
        'border-b border-slate-800 last:border-b-0',
        'border-b border-border last:border-b-0'],

    // #24 - SuggestProfileModal.tsx
    ['src/components/Settings/SuggestProfileModal.tsx',
        "'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'",
        "'bg-surface-alt text-base-content'"],

    // #25 - SystemLogsPanel.tsx: terminal bg-slate-950 — intentional, add eslint-disable inline
    ['src/components/Settings/SystemLogsPanel.tsx',
        'class="bg-slate-950 text-slate-300 font-mono',
        'class="bg-slate-950 text-slate-300 font-mono'], // keep as-is, will suppress via eslint rule

    // #26 - SystemLogsPanel.tsx: terminal line styling
    ['src/components/Settings/SystemLogsPanel.tsx',
        'border-b border-slate-900 last:border-0 pb-0.5 mb-0.5 hover:bg-slate-900',
        'border-b border-border-subtle last:border-0 pb-0.5 mb-0.5 hover:bg-surface-hover'],

    // #27 - UnifiedAgents.tsx: inverted button — intentional dark/light swap
    ['src/components/Settings/UnifiedAgents.tsx',
        "'bg-slate-900 text-white hover:bg-black dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'",
        "'bg-slate-900 text-white hover:bg-black dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'"], // Keep: intentional inverted CTA

    // #28,29 - UnifiedAgents.tsx: pre blocks with bg-slate-950 for terminal output — intentional
    // Will suppress via eslint at rule level since 950 is a terminal bg

    // #30 - UnifiedAgents.tsx: badge
    ['src/components/Settings/UnifiedAgents.tsx',
        "'bg-slate-100 text-slate-800 dark:bg-slate-700 dark:text-slate-300'",
        "'bg-surface-alt text-base-content'"],

    // #31 - StoragePoolRow.tsx
    ['src/components/Storage/StoragePoolRow.tsx',
        "'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'",
        "'bg-surface-alt text-muted'"],

    // #32 - InvestigationSection.tsx
    ['src/components/patrol/InvestigationSection.tsx',
        "'border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400'",
        "'border-border bg-surface-hover text-muted'"],

    // #33 - Form.ts: input base styles
    ['src/components/shared/Form.ts',
        "'w-full min-h-10 sm:min-h-9 rounded-md border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900'",
        "'w-full min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-3 py-2.5 text-sm text-base-content'"],

    // #34 - NodeGroupHeader.tsx
    ['src/components/shared/NodeGroupHeader.tsx',
        "'bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-300'",
        "'bg-surface-alt text-muted'"],

    // #35 - ScrollToTopButton.tsx: floating button
    ['src/components/shared/ScrollToTopButton.tsx',
        'bg-slate-800 text-white shadow-sm transition-all duration-200 hover:bg-slate-700 dark:bg-slate-600 dark:hover:bg-slate-500',
        'bg-surface text-base-content shadow-sm transition-all duration-200 hover:bg-surface-hover border border-border'],
];

let total = 0;
let filesModified = 0;

for (const [relFile, search, replace] of FIXES) {
    if (search === replace) continue; // Skip no-ops (intentional keeps)
    const absPath = path.join(__dirname, relFile);
    if (!fs.existsSync(absPath)) {
        console.log(`  SKIP ${relFile} — file not found`);
        continue;
    }
    let content = fs.readFileSync(absPath, 'utf-8');
    if (!content.includes(search)) {
        console.log(`  SKIP ${relFile} — pattern not found`);
        continue;
    }
    const count = content.split(search).length - 1;
    content = content.replaceAll(search, replace);
    total += count;
    filesModified++;
    console.log(`  ${relFile}: ${count} fix(es)`);
    if (!dryRun) {
        fs.writeFileSync(absPath, content, 'utf-8');
    }
}

console.log(`\n${dryRun ? '[DRY RUN] ' : ''}${total} manual fixes across ${filesModified} files`);
if (dryRun && total > 0) console.log('Run with --write to apply.');
