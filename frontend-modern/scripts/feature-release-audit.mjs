#!/usr/bin/env node
// Keeps the feature release list honest.
//
// Context: on 19 November 2026 the map was a day from a release candidate
// after two days of work and no beta, and its guide was already linked from
// the docs index. Work lands on `main` at once and `main` ships every week,
// so a new feature is listed in src/utils/featureReleases.json and shown only
// in the builds its stage names (see src/utils/featureReleases.ts).
//
// This audit refuses:
//   - an entry that is not spelled the way the app reads it;
//   - a stage past `build` with no `approval` line. Only Richard moves a
//     feature to `preview` or `released`; the line says who said so and when;
//   - a link to a feature's guide from any shipped guide or release note
//     before the feature is released. A stable build carries those pages, so
//     a link there tells every user about a page they cannot open.
//
// Governance markdown under docs/release-control/ is not a shipped guide and
// is not scanned.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const ROOT = process.cwd();
const REPO_ROOT = path.resolve(ROOT, '..');
const REGISTRY_PATH = path.join(ROOT, 'src', 'utils', 'featureReleases.json');
const STAGES = ['build', 'preview', 'released'];
const SKIP_PREFIXES = ['docs/release-control/'];
const SCANNED = ['docs', 'README.md', 'frontend-modern/public/docs'];
const GUIDE_NAME = /^[A-Za-z0-9_-]+\.md$/;

function fail(message) {
  console.error(`feature-release-audit: ${message}`);
  process.exit(2);
}

function readRegistry() {
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(REGISTRY_PATH, 'utf8'));
  } catch (error) {
    fail(`unable to read ${path.relative(REPO_ROOT, REGISTRY_PATH)}: ${error.message}`);
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    fail('the feature release list must be an object of features');
  }
  return parsed;
}

function trackedMarkdown() {
  let tracked;
  try {
    tracked = execFileSync('git', ['-C', REPO_ROOT, 'ls-files', '-z', '--', ...SCANNED], {
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
    });
  } catch (error) {
    fail(`unable to enumerate tracked guides: ${error.message}`);
  }
  return tracked
    .split('\0')
    .filter(Boolean)
    .filter((relative) => relative.endsWith('.md'))
    .filter((relative) => !SKIP_PREFIXES.some((prefix) => relative.startsWith(prefix)));
}

const escapeRegExp = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// `MAP.md` as a file, or `docs/MAP` as the app addresses it, in any case, as
// the docs page resolves both. `ROADMAP.md` and `docs/MAPPING` are other
// pages. A mention that is not a link counts too: it names a page the reader
// cannot open.
function guideReference(guide) {
  const file = escapeRegExp(guide);
  const stem = escapeRegExp(guide.replace(/\.md$/, ''));
  return new RegExp(`(?<![\\w-])${file}(?!\\w)|(?<![\\w-])docs/${stem}(?![\\w.-])`, 'i');
}

const problems = [];
const registry = readRegistry();
const held = [];

for (const [id, entry] of Object.entries(registry)) {
  if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
    problems.push(`${id}: the entry must be an object`);
    continue;
  }
  if (typeof entry.name !== 'string' || !entry.name.trim()) {
    problems.push(`${id}: "name" must say what users call the feature`);
  }
  if (!STAGES.includes(entry.stage)) {
    problems.push(`${id}: "stage" must be one of ${STAGES.join(', ')}`);
  }
  const guides = Array.isArray(entry.guides) ? entry.guides : null;
  if (!guides || guides.some((guide) => typeof guide !== 'string' || !GUIDE_NAME.test(guide))) {
    problems.push(`${id}: "guides" must list file names under docs/, such as "MAP.md"`);
  }
  if (entry.stage !== 'build' && STAGES.includes(entry.stage)) {
    if (typeof entry.approval !== 'string' || !entry.approval.trim()) {
      problems.push(
        `${id}: stage "${entry.stage}" needs an "approval" line saying who moved it and when. ` +
          'Only Richard moves a feature past build.',
      );
    }
  }
  if (guides && entry.stage !== 'released') {
    for (const guide of guides) {
      if (typeof guide === 'string' && GUIDE_NAME.test(guide)) held.push({ id, guide });
    }
  }
}

if (held.length > 0) {
  const documents = trackedMarkdown();
  for (const { id, guide } of held) {
    const own = new Set([`docs/${guide}`, `frontend-modern/public/docs/${guide}`]);
    if (!documents.includes(`docs/${guide}`)) {
      problems.push(`${id}: guide docs/${guide} is not a tracked file`);
    }
    const reference = guideReference(guide);
    for (const relative of documents) {
      if (own.has(relative)) continue;
      let text;
      try {
        text = fs.readFileSync(path.join(REPO_ROOT, ...relative.split('/')), 'utf8');
      } catch {
        continue;
      }
      const lines = text.split('\n');
      for (let index = 0; index < lines.length; index += 1) {
        // `docs/./MAP` is the same address as `docs/MAP`.
        if (reference.test(lines[index].replace(/\/(?:\.\/)+/g, '/'))) {
          problems.push(
            `${relative}:${index + 1} points to ${guide}, the guide of "${id}", which is not released`,
          );
        }
      }
    }
  }
}

if (problems.length > 0) {
  console.error('feature-release-audit: the feature release list does not hold:');
  for (const problem of problems) console.error(`  ${problem}`);
  console.error('');
  console.error(
    'A feature that is not released is shown only in the builds its stage names, and the\n' +
      'guides and release notes that ship to everyone do not point to it. Remove the link,\n' +
      "or release the feature on Richard's word. See src/utils/featureReleases.ts.",
  );
  process.exit(1);
}

console.log(
  `feature-release-audit: ${Object.keys(registry).length} feature(s) listed, ` +
    `${held.length} guide(s) held back and unlinked.`,
);
