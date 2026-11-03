#!/usr/bin/env node
// Ratchet on tests that read product source as text instead of exercising it.
//
// Context: on 3 November 2026 about 9,400 frontend assertions read source
// files with ?raw / readFileSync and checked for strings ("file contains
// helper X", "export function Y exists"). They broke on harmless refactors,
// passed on real bugs, and grew by one pin per change because a pin is the
// cheapest edit that satisfies the proof-file rule. The same audit found 492
// date-named coverage-chasing test files (*.branchcov0712b.test.ts).
//
// This audit keeps both from growing: a test file may not read source more
// times than its baseline entry (new files start at zero), and no new
// date-named coverage file may appear. Test behaviour instead: render the
// component or call the function. Tree-wide scans that genuinely need source
// text belong in a generic audit script like this one.
//
//   node scripts/source-text-test-audit.mjs                  check
//   node scripts/source-text-test-audit.mjs --write-baseline lower entries to current counts
//
// --write-baseline only ever lowers counts and drops entries for files that no
// longer read source. Raising an entry means editing the JSON by hand; say why
// in the commit.
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const REPO_ROOT = path.resolve(ROOT, '..');
const BASELINE_PATH = path.join(ROOT, 'scripts', 'source-text-test-baseline.json');
const SKIP_DIRS = new Set([
  'node_modules',
  '.git',
  'dist',
  'vendor',
  'coverage',
  'browser-verification',
]);

const FRONTEND_TEST = /\.(test|spec)\.(ts|tsx|mts|mjs)$/;
const GO_TEST = /_test\.go$/;
const COVERAGE_CHASING_NAME =
  /(?:^|[._-])(?:branchcov\d*[a-z]*|coverage\d+[a-z]*|cov\d{3,}[a-z]*)(?:[._-]|$)/i;
const SOURCE_LITERAL = /["'`][^"'`\n]*\.(?:tsx?|css|go)["'`]/;
const FIXTURE_HINT = /fixture|testdata|\.json\b/i;

function walk(dir, match, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) walk(path.join(dir, entry.name), match, out);
    } else if (match.test(entry.name)) {
      out.push(path.join(dir, entry.name));
    }
  }
  return out;
}

function frontendReads(source) {
  let count = (source.match(/\?raw['"`]/g) ?? []).length;
  const helpers = new Set();
  for (const line of source.split('\n')) {
    if (!/\breadFile(?:Sync)?\s*\(/.test(line) || FIXTURE_HINT.test(line)) continue;
    const helper = line.match(/(?:const\s+(\w+)\s*=\s*(?:async\s*)?\(|function\s+(\w+)\s*\()/);
    if (helper) helpers.add(helper[1] ?? helper[2]);
    else count += 1;
  }
  for (const name of helpers) {
    count += (source.match(new RegExp(`\\b${name}\\(`, 'g')) ?? []).length - 1;
  }
  return count;
}

function goReads(source) {
  let count = 0;
  for (const line of source.split('\n')) {
    if (!SOURCE_LITERAL.test(line) || FIXTURE_HINT.test(line)) continue;
    if (/ReadFile\(|\bread\w*\(|Source\(/.test(line)) count += 1;
  }
  return count;
}

function scan() {
  const reads = {};
  const coverageChasing = [];
  const files = [
    ...walk(path.join(ROOT, 'src'), FRONTEND_TEST).map((file) => [file, frontendReads]),
    ...walk(REPO_ROOT, GO_TEST).map((file) => [file, goReads]),
  ];
  for (const [file, counter] of files) {
    const rel = path.relative(REPO_ROOT, file).split(path.sep).join('/');
    const count = counter(fs.readFileSync(file, 'utf8'));
    if (count > 0) reads[rel] = count;
    if (COVERAGE_CHASING_NAME.test(path.basename(file))) coverageChasing.push(rel);
  }
  return { reads, coverageChasing: coverageChasing.sort() };
}

const sortKeys = (object) =>
  Object.fromEntries(Object.entries(object).sort(([a], [b]) => a.localeCompare(b)));
const current = scan();
const baseline = fs.existsSync(BASELINE_PATH)
  ? JSON.parse(fs.readFileSync(BASELINE_PATH, 'utf8'))
  : { sourceReads: {}, coverageChasingFiles: [] };

if (process.argv.includes('--write-baseline')) {
  const sourceReads = {};
  for (const [file, count] of Object.entries(current.reads)) {
    const previous = baseline.sourceReads[file];
    if (previous !== undefined || !fs.existsSync(BASELINE_PATH)) {
      sourceReads[file] = previous === undefined ? count : Math.min(previous, count);
    }
  }
  const allowed = new Set(baseline.coverageChasingFiles);
  const coverageChasingFiles = current.coverageChasing.filter(
    (file) => allowed.has(file) || !fs.existsSync(BASELINE_PATH),
  );
  fs.writeFileSync(
    BASELINE_PATH,
    `${JSON.stringify({ sourceReads: sortKeys(sourceReads), coverageChasingFiles }, null, 2)}\n`,
  );
  const total = Object.values(sourceReads).reduce((sum, count) => sum + count, 0);
  console.log(
    `Wrote ${path.relative(ROOT, BASELINE_PATH)}: ${Object.keys(sourceReads).length} files, ${total} source reads, ${coverageChasingFiles.length} coverage-chasing files.`,
  );
  process.exit(0);
}

const failures = [];
for (const [file, count] of Object.entries(current.reads)) {
  const allowed = baseline.sourceReads[file] ?? 0;
  if (count > allowed) {
    failures.push(
      `${file}: reads source as text ${count} times (baseline ${allowed}). Test behaviour instead: render the component or call the function.`,
    );
  }
}
const allowedCoverage = new Set(baseline.coverageChasingFiles);
for (const file of current.coverageChasing) {
  if (!allowedCoverage.has(file)) {
    failures.push(
      `${file}: new date-named coverage test file. Put the test in the file that tests the same unit, named for the behaviour it checks.`,
    );
  }
}

if (failures.length > 0) {
  console.error('Source-text test audit failed:');
  for (const failure of failures) console.error(`- ${failure}`);
  console.error(
    'After removing reads, lower the baseline with: node scripts/source-text-test-audit.mjs --write-baseline',
  );
  process.exit(1);
}

const total = Object.values(current.reads).reduce((sum, count) => sum + count, 0);
console.log(
  `Source-text test audit passed (${Object.keys(current.reads).length} files, ${total} source reads, ${current.coverageChasing.length} coverage-chasing files).`,
);
