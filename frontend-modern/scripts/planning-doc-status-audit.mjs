#!/usr/bin/env node
// Requires an explicit status marker on every planning document under docs/.
//
// Context: on 2026-09-01 the autonomous web-product lane found
// docs/release-control/v6/internal/HOME_STATUS_WALL_SPEC.md (a handoff spec
// written 2026-07-10) and shipped it as a new navigation tab with no demand
// ledger entry. A repository spec, plan, or contract is a record of a past
// decision, not demand; the maintainer prompts now say so, and the stale-spec
// triage of 2026-09-01 gave every existing planning doc an honest Status line.
// This audit keeps that true for new documents: any *_SPEC.md, *_PLAN.md, or
// *_CONTRACT.md under docs/ must carry a `Status:` line (or a `## Status`
// section) in its header, so nobody, human or agent, has to infer whether the
// document is live, implemented, superseded, or parked.
//
// Subsystem contract markdown lives under
// docs/release-control/v6/internal/subsystems/ and is governed separately, so
// it is not scanned even though the filenames do not match the suffixes above.
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const REPO_ROOT = path.resolve(ROOT, '..');
const DOCS_ROOT = path.join(REPO_ROOT, 'docs');
const SKIP_DIRS = new Set([path.join(DOCS_ROOT, 'release-control', 'v6', 'internal', 'subsystems')]);
const PLANNING_SUFFIX = /_(SPEC|PLAN|CONTRACT)\.md$/;
const HEADER_LINES = 40;
const STATUS_LINE = /^(Status:\s*\S|## Status\s*$)/;

function walk(dir, out) {
  if (SKIP_DIRS.has(dir)) return out;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (entry.isFile() && PLANNING_SUFFIX.test(entry.name)) out.push(full);
  }
  return out;
}

const failures = [];
for (const file of walk(DOCS_ROOT, []).sort()) {
  const head = fs.readFileSync(file, 'utf8').split('\n').slice(0, HEADER_LINES);
  if (!head.some((line) => STATUS_LINE.test(line))) {
    failures.push(path.relative(REPO_ROOT, file));
  }
}

if (failures.length > 0) {
  console.error('planning-doc-status-audit: planning documents without a Status line:');
  for (const file of failures) console.error(`  ${file}`);
  console.error('');
  console.error(
    `Add a "Status:" line within the first ${HEADER_LINES} lines. A spec, plan, or contract is a\n` +
      'record of a decision, not demand: say whether it is ACTIVE (with its\n' +
      'FEATURE_REQUESTS.md ledger entry), IMPLEMENTED (cite the code), SUPERSEDED (name the\n' +
      'replacement), or PARKED (not a current signal; reactivate only through a\n' +
      'FEATURE_REQUESTS.md ledger entry), and date the review.',
  );
  process.exit(1);
}

console.log('planning-doc-status-audit: ok');
