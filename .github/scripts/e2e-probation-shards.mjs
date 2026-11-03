import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { PROBATION_SPECS, QUARANTINED_SPECS } from '../../tests/integration/e2e-tiering.mjs';
import { createSpecMatcher } from './e2e-probation-scope.mjs';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const SPEC_ROOT = 'tests/integration/tests/';
const DEFAULT_SECONDS = 30;
const compareText = (left, right) => left < right ? -1 : left > right ? 1 : 0;

export function parseProbationShard(value) {
  const match = /^([1-9]\d*)\/([1-9]\d*)$/.exec(value);
  const current = Number(match?.[1]);
  const total = Number(match?.[2]);
  if (!Number.isSafeInteger(current) || !Number.isSafeInteger(total) || current > total) {
    throw new Error(`Invalid probation shard: ${value}`);
  }
  return { current, total };
}

// A file stays together within each browser project, preserving suite hooks
// and global-state ordering. Only placement changes; every pair appears once.
export function allocateProbationShards(files, projects, seconds, total) {
  if (!Number.isSafeInteger(total) || total < 1) throw new Error('Invalid shard total');
  if (!projects.length || new Set(projects).size !== projects.length) {
    throw new Error('Browser projects must be nonempty and unique');
  }
  if (new Set(files).size !== files.length || files.some((file) =>
    !file.startsWith(SPEC_ROOT) || !file.endsWith('.spec.ts') || file.split('/').includes('..'))) {
    throw new Error('Spec paths must be unique repository-relative files');
  }
  for (const weights of Object.values(seconds)) {
    for (const value of Object.values(weights)) {
      if (!Number.isFinite(value) || value < 0) throw new Error('Invalid recorded test duration');
    }
  }
  const work = files.flatMap((file) => projects.map((project) => ({
    file,
    project,
    seconds: seconds[project]?.[file] ?? DEFAULT_SECONDS,
  }))).sort((left, right) => right.seconds - left.seconds ||
    compareText(left.file, right.file) || compareText(left.project, right.project));
  const shards = Array.from({ length: total }, () => ({ seconds: 0, assignments: [] }));
  for (const item of work) {
    const target = shards.reduce((least, shard) => shard.seconds < least.seconds ? shard : least);
    target.assignments.push(item);
    target.seconds += item.seconds;
  }
  return shards;
}

function discoverSpecs(repoRoot) {
  const files = [];
  const walk = (directory, relative) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const file = `${relative}${entry.name}`;
      if (entry.isDirectory()) walk(join(directory, entry.name), `${file}/`);
      else if (entry.isFile() && entry.name.endsWith('.spec.ts')) files.push(file);
    }
  };
  walk(join(repoRoot, SPEC_ROOT), SPEC_ROOT);
  const probation = createSpecMatcher(PROBATION_SPECS);
  const quarantined = createSpecMatcher(QUARANTINED_SPECS);
  return files.filter((file) => probation(file) && !quarantined(file)).sort(compareText);
}

export function probationProjectMatches(shardValue, projects, repoRoot = REPO_ROOT) {
  const { current, total } = parseProbationShard(shardValue);
  const profile = JSON.parse(readFileSync(join(repoRoot, '.github/scripts/e2e-probation-seconds.json'), 'utf8'));
  if (profile.version !== 1 || !profile.seconds_by_project_file) {
    throw new Error('Unsupported probation timing profile');
  }
  const plan = allocateProbationShards(discoverSpecs(repoRoot), projects, profile.seconds_by_project_file, total);
  const selected = plan[current - 1];
  console.error(`[e2e] Probation shard ${current}/${total}: ${selected.assignments.length} file/project pairs, ${Math.round(selected.seconds)}s recorded estimate`);
  return new Map(projects.map((project) => {
    const matches = selected.assignments.filter((item) => item.project === project).map(({ file }) =>
      new RegExp(`^${join(repoRoot, file).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`));
    // An empty match array means Playwright's default match, which would
    // duplicate coverage. Use an explicit never-match expression instead.
    return [project, matches.length ? matches : [new RegExp('(?!)')]];
  }));
}
