#!/usr/bin/env node
/**
 * Bundle size checker — validates vite build output against .bundlesize.json baseline.
 *
 * Usage:
 *   node scripts/check-bundle-size.mjs              # check against baseline (requires prior vite build)
 *   node scripts/check-bundle-size.mjs --update-baseline  # rebuild baseline from current dist/
 *
 * Methodology:
 *   1. Reads all .js/.css files from dist/assets/
 *   2. Strips Vite content-hash suffix to recover logical chunk names
 *   3. Groups files sharing the same logical name (e.g. two "Dashboard" chunks)
 *   4. Computes gzip size for each file using Node zlib (level 6, default)
 *   5. Compares per-chunk and total gzip sizes against .bundlesize.json thresholds
 *   6. Asserts the built index.html preload posture: modulepreload links are
 *      limited to the entry's static imports (no lazy route chunks), and the
 *      import map integrity block covers every built JS asset
 *   7. Exits 0 on pass, 1 on any violation
 */

import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { gzipSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const ROOT = resolve(__dirname, '..');
const DIST_ASSETS = join(ROOT, 'dist', 'assets');
const BASELINE_PATH = join(ROOT, '.bundlesize.json');

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Strip Vite/Rollup 8-char base64url content hash from filename. */
function logicalName(filename) {
  // Vite appends -<8 char hash>.<ext>; hash chars are [A-Za-z0-9_-]
  const m = filename.match(/^(.+)-[A-Za-z0-9_-]{8}\.(js|css)$/);
  if (m) return { name: m[1], ext: m[2] };
  // Fallback: return filename without extension
  const dot = filename.lastIndexOf('.');
  return { name: filename.slice(0, dot), ext: filename.slice(dot + 1) };
}

function gzipSize(buf) {
  return gzipSync(buf, { level: 6 }).length;
}

function fmtKB(bytes) {
  return (bytes / 1024).toFixed(2) + ' kB';
}

// ---------------------------------------------------------------------------
// Measure current build
// ---------------------------------------------------------------------------

function measureBuild() {
  let files;
  try {
    files = readdirSync(DIST_ASSETS);
  } catch {
    console.error(
      'Error: dist/assets/ not found. Run "npx vite build" first.',
    );
    process.exit(1);
  }

  /** @type {Map<string, { gzip: number, raw: number }>} */
  const groups = new Map();
  let totalJsGzip = 0;
  let totalCssGzip = 0;

  for (const file of files) {
    if (!file.endsWith('.js') && !file.endsWith('.css')) continue;
    const fullPath = join(DIST_ASSETS, file);
    const buf = readFileSync(fullPath);
    const gz = gzipSize(buf);
    const raw = buf.length;

    const { name, ext } = logicalName(file);

    // Use name.ext as group key to avoid mixing JS and CSS with same name
    const groupKey = `${name}.${ext}`;
    const existing = groups.get(groupKey) || { gzip: 0, raw: 0, ext, name };
    existing.gzip += gz;
    existing.raw += raw;
    groups.set(groupKey, existing);

    if (ext === 'js') totalJsGzip += gz;
    else if (ext === 'css') totalCssGzip += gz;
  }

  return { groups, totalJsGzip, totalCssGzip };
}

// ---------------------------------------------------------------------------
// Built index.html preload posture
// ---------------------------------------------------------------------------

/**
 * Verify the deployment-installability preload invariant against the built
 * index.html (see the subsystem contract clause on modulepreload posture):
 *   - the import map integrity block exists and covers every JS asset in
 *     dist/assets, so dynamic-import SRI stays enforced without preloading;
 *   - modulepreload links reference only the entry chunk's static imports —
 *     preloading lazy route chunks (preloadDynamicChunks: true) would fetch
 *     the whole app at cold start and defeat route-level code splitting.
 * Returns a list of violation messages (empty on pass).
 */
function checkPreloadPosture() {
  const errors = [];
  let html;
  try {
    html = readFileSync(join(ROOT, 'dist', 'index.html'), 'utf8');
  } catch {
    errors.push('dist/index.html not found. Run "npx vite build" first.');
    return errors;
  }

  const importmapMatch = html.match(/<script type="importmap">([\s\S]*?)<\/script>/);
  let integrity = null;
  if (!importmapMatch) {
    errors.push('importmap script block missing from index.html (dynamic-import SRI unenforced)');
  } else {
    let parsed;
    try {
      parsed = JSON.parse(importmapMatch[1]);
    } catch {
      errors.push('importmap block in index.html is not valid JSON');
    }
    if (parsed) {
      if (!parsed.integrity || typeof parsed.integrity !== 'object' || Object.keys(parsed.integrity).length === 0) {
        errors.push('importmap in index.html has no integrity block (dynamic-import SRI unenforced)');
      } else {
        integrity = parsed.integrity;
      }
    }
  }
  if (integrity) {
    for (const file of readdirSync(DIST_ASSETS)) {
      if (!file.endsWith('.js')) continue;
      if (!(`/assets/${file}` in integrity)) {
        errors.push(`importmap integrity missing entry for /assets/${file}`);
      }
    }
  }

  const entryMatch = html.match(/<script type="module"[^>]*\bsrc="\/assets\/([^"]+\.js)"[^>]*>/);
  if (!entryMatch) {
    errors.push('module entry script not found in index.html');
    return errors;
  }
  const entryFile = entryMatch[1];
  if (!/\bintegrity="/.test(entryMatch[0])) {
    errors.push(`entry script /assets/${entryFile} missing integrity attribute`);
  }

  let entrySource;
  try {
    entrySource = readFileSync(join(DIST_ASSETS, entryFile), 'utf8');
  } catch {
    errors.push(`entry chunk ${entryFile} not found in dist/assets/`);
    return errors;
  }

  // Static imports/re-exports in Rollup output: import ... from"./x.js",
  // import"./x.js", export ... from"./x.js". Dynamic imports never match:
  // their specifier follows an opening parenthesis, not the keyword.
  const allowedPreloads = new Set([entryFile]);
  const staticImportRe = /\b(?:import|export)\s*(?:[\w$*{},\s]+?\s*from\s*)?["']\.\/([^"']+\.js)["']/g;
  let m;
  while ((m = staticImportRe.exec(entrySource))) allowedPreloads.add(m[1]);

  for (const tag of html.match(/<link rel="modulepreload"[^>]*>/g) ?? []) {
    const href = tag.match(/\bhref="\/assets\/([^"]+)"/);
    if (!href) {
      errors.push(`modulepreload link with unrecognized href in index.html: ${tag}`);
      continue;
    }
    if (!allowedPreloads.has(href[1])) {
      errors.push(
        `modulepreload of lazy chunk /assets/${href[1]} — only the entry's static imports may be preloaded (keep preloadDynamicChunks: false in vite.config.ts)`,
      );
    }
    if (!/\bintegrity="/.test(tag)) {
      errors.push(`modulepreload of /assets/${href[1]} missing integrity attribute`);
    }
  }

  return errors;
}

// ---------------------------------------------------------------------------
// Update baseline mode
// ---------------------------------------------------------------------------

function updateBaseline() {
  const { groups, totalJsGzip, totalCssGzip } = measureBuild();

  // Only include chunks > 5kB gzip in the per-chunk baseline
  const MIN_TRACKED_GZIP = 5 * 1024;
  const chunks = {};
  const sorted = [...groups.entries()].sort((a, b) => b[1].gzip - a[1].gzip);
  for (const [groupKey, { gzip, ext, name }] of sorted) {
    if (ext === 'js' && gzip >= MIN_TRACKED_GZIP) {
      chunks[name] = { baselineGzip: gzip };
    }
  }

  const baseline = {
    $comment:
      "Frontend bundle size baseline. Run 'npm run check:bundlesize' to validate. To update: run 'node scripts/check-bundle-size.mjs --update-baseline' after vite build.",
    version: 1,
    generated: new Date().toISOString().slice(0, 10),
    defaultThresholdPercent: 15,
    chunks,
    totals: {
      js: { baselineGzip: totalJsGzip },
      css: { baselineGzip: totalCssGzip },
    },
  };

  writeFileSync(BASELINE_PATH, JSON.stringify(baseline, null, 2) + '\n');
  console.log(`Baseline updated: ${BASELINE_PATH}`);
  console.log(`  JS total gzip:  ${fmtKB(totalJsGzip)}`);
  console.log(`  CSS total gzip: ${fmtKB(totalCssGzip)}`);
  console.log(`  Tracked chunks: ${Object.keys(chunks).length}`);
}

// ---------------------------------------------------------------------------
// Check mode (default)
// ---------------------------------------------------------------------------

function check() {
  let baseline;
  try {
    baseline = JSON.parse(readFileSync(BASELINE_PATH, 'utf8'));
  } catch (err) {
    if (err.code === 'ENOENT') {
      console.error(`Error: ${BASELINE_PATH} not found. Run with --update-baseline first.`);
    } else {
      console.error(`Error reading ${BASELINE_PATH}: ${err.message}`);
    }
    process.exit(1);
  }

  if (
    !baseline.chunks ||
    typeof baseline.chunks !== 'object' ||
    Array.isArray(baseline.chunks) ||
    !baseline.totals ||
    typeof baseline.totals !== 'object' ||
    typeof baseline.totals.js?.baselineGzip !== 'number' ||
    typeof baseline.totals.css?.baselineGzip !== 'number'
  ) {
    console.error(
      `Error: ${BASELINE_PATH} has invalid schema. Expected "chunks" (object) and "totals" with numeric js/css baselineGzip fields.\nRun with --update-baseline to regenerate.`,
    );
    process.exit(1);
  }

  // Validate each chunk entry has a numeric baselineGzip
  for (const [chunkName, config] of Object.entries(baseline.chunks)) {
    if (!config || typeof config !== 'object' || Array.isArray(config) || typeof config.baselineGzip !== 'number') {
      console.error(
        `Error: ${BASELINE_PATH} chunk "${chunkName}" has invalid or missing baselineGzip.\nRun with --update-baseline to regenerate.`,
      );
      process.exit(1);
    }
  }

  const { groups, totalJsGzip, totalCssGzip } = measureBuild();
  const threshold = baseline.defaultThresholdPercent ?? 15;
  const violations = [];

  // Check per-chunk limits
  for (const [chunkName, config] of Object.entries(baseline.chunks)) {
    const maxGzip = Math.ceil(
      config.baselineGzip * (1 + threshold / 100),
    );
    // Baseline uses clean names (e.g. "vendor-solid"); groups use "name.ext" keys
    const measured = groups.get(`${chunkName}.js`);
    const actualGzip = measured ? measured.gzip : 0;

    // Warn if a tracked chunk is missing from the build (could indicate hash format change)
    if (!measured) {
      violations.push({
        name: chunkName,
        baseline: config.baselineGzip,
        actual: 0,
        max: maxGzip,
        overBy: 0,
        missing: true,
      });
      continue;
    }

    if (actualGzip > maxGzip) {
      violations.push({
        name: chunkName,
        baseline: config.baselineGzip,
        actual: actualGzip,
        max: maxGzip,
        overBy: actualGzip - maxGzip,
      });
    }
  }

  // Check totals
  for (const [label, actual] of [['js', totalJsGzip], ['css', totalCssGzip]]) {
    const config = baseline.totals?.[label];
    if (!config) continue;
    const maxGzip = Math.ceil(config.baselineGzip * (1 + threshold / 100));
    if (actual > maxGzip) {
      violations.push({
        name: `total-${label}`,
        baseline: config.baselineGzip,
        actual,
        max: maxGzip,
        overBy: actual - maxGzip,
      });
    }
  }

  // Report
  console.log('Bundle size check');
  console.log('=================');
  console.log(`Threshold: ${threshold}% above baseline\n`);

  // Always show summary table of tracked chunks
  const chunkEntries = Object.entries(baseline.chunks).sort(
    (a, b) => b[1].baselineGzip - a[1].baselineGzip,
  );
  console.log('Chunk'.padEnd(28) + 'Baseline'.padStart(12) + 'Actual'.padStart(12) + 'Max'.padStart(12) + 'Status'.padStart(10));
  console.log('-'.repeat(74));

  for (const [chunkName, config] of chunkEntries) {
    const maxGzip = Math.ceil(config.baselineGzip * (1 + threshold / 100));
    const measured = groups.get(`${chunkName}.js`);
    const actualGzip = measured ? measured.gzip : 0;
    const status = !measured ? 'MISSING' : actualGzip > maxGzip ? 'OVER' : 'ok';
    console.log(
      chunkName.padEnd(28) +
        fmtKB(config.baselineGzip).padStart(12) +
        fmtKB(actualGzip).padStart(12) +
        fmtKB(maxGzip).padStart(12) +
        status.padStart(10),
    );
  }

  console.log('-'.repeat(74));
  console.log(
    'Total JS'.padEnd(28) +
      fmtKB(baseline.totals.js.baselineGzip).padStart(12) +
      fmtKB(totalJsGzip).padStart(12) +
      fmtKB(Math.ceil(baseline.totals.js.baselineGzip * (1 + threshold / 100))).padStart(12) +
      (totalJsGzip > Math.ceil(baseline.totals.js.baselineGzip * (1 + threshold / 100)) ? 'OVER' : 'ok').padStart(10),
  );
  console.log(
    'Total CSS'.padEnd(28) +
      fmtKB(baseline.totals.css.baselineGzip).padStart(12) +
      fmtKB(totalCssGzip).padStart(12) +
      fmtKB(Math.ceil(baseline.totals.css.baselineGzip * (1 + threshold / 100))).padStart(12) +
      (totalCssGzip > Math.ceil(baseline.totals.css.baselineGzip * (1 + threshold / 100)) ? 'OVER' : 'ok').padStart(10),
  );

  console.log();

  const postureErrors = checkPreloadPosture();
  console.log('index.html preload posture');
  console.log('==========================');
  if (postureErrors.length === 0) {
    console.log('ok: modulepreload limited to entry static imports; importmap integrity covers all JS assets.');
  } else {
    for (const message of postureErrors) {
      console.log(`  ${message}`);
    }
  }
  console.log();

  if (violations.length === 0 && postureErrors.length === 0) {
    console.log('All chunks within budget.');
    process.exit(0);
  }
  if (postureErrors.length > 0) {
    console.log(`${postureErrors.length} preload posture violation(s) found (listed above).`);
  }
  if (violations.length > 0) {
    console.log(`${violations.length} bundle size violation(s) found:\n`);
    for (const v of violations) {
      if (v.missing) {
        console.log(
          `  ${v.name}: MISSING — tracked chunk not found in build (expected ~${fmtKB(v.baseline)}). Hash format may have changed; run --update-baseline.`,
        );
      } else {
        console.log(
          `  ${v.name}: ${fmtKB(v.actual)} exceeds max ${fmtKB(v.max)} (baseline ${fmtKB(v.baseline)}, over by ${fmtKB(v.overBy)})`,
        );
      }
    }
    console.log(
      '\nTo update the baseline after an intentional size increase:',
    );
    console.log('  npx vite build && node scripts/check-bundle-size.mjs --update-baseline');
  }
  process.exit(1);
}

// ---------------------------------------------------------------------------
// Entry
// ---------------------------------------------------------------------------

if (process.argv.includes('--update-baseline')) {
  updateBaseline();
} else {
  check();
}
