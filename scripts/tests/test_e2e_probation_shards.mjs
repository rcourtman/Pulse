import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import test from 'node:test';

import { allocateProbationShards, parseProbationShard, probationProjectMatches } from '../../.github/scripts/e2e-probation-shards.mjs';
import { PROBATION_SPECS } from '../../tests/integration/e2e-tiering.mjs';
import { parseListedTests, validateProbationAllocation } from '../../tests/integration/scripts/check-probation-shards.mjs';

const root = 'tests/integration/tests/';
const identity = ({ project, file }) => `${project}:${file}`;

test('real-list accounting rejects missing, duplicate, foreign and unparsed cases', () => {
  const expected = new Set(['desktop:one', 'phone:one']);
  validateProbationAllocation(expected, [['desktop:one'], [], ['phone:one']]);
  assert.throws(() => validateProbationAllocation(expected, [['desktop:one']]), /Unallocated/);
  assert.throws(() => validateProbationAllocation(expected, [['desktop:one'], ['desktop:one', 'phone:one']]), /twice/);
  assert.throws(() => validateProbationAllocation(expected, [['desktop:one', 'phone:one', 'foreign']]), /Unexpected/);
  const line = '  [desktop] › one.spec.ts:1:1 › group › case';
  assert.equal(parseListedTests(`${line}\nTotal: 1 test in 1 file`, 'fixture').length, 1);
  assert.throws(() => parseListedTests(`${line}\nTotal: 2 tests in 1 file`, 'fixture'), /Incomplete/);
  assert.throws(() => parseListedTests(`${line}\n${line}\nTotal: 2 tests in 1 file`, 'fixture'), /Duplicate/);
  assert.throws(() => parseListedTests(line, 'fixture'), /Incomplete/);
});

test('file/project allocation partitions coverage and is independent of input order', () => {
  const files = ['one', 'two', 'three'].map((name) => `${root}${name}.spec.ts`);
  const projects = ['desktop', 'phone'];
  const plan = allocateProbationShards(files, projects, {}, 8);
  const actual = plan.flatMap((shard) => shard.assignments.map(identity)).sort();
  const expected = files.flatMap((file) => projects.map((project) => identity({ project, file }))).sort();
  assert.deepEqual(actual, expected);
  assert.equal(new Set(actual).size, actual.length);
  assert.deepEqual(plan, allocateProbationShards([...files].reverse(), [...projects].reverse(), {}, 8));
});

test('a long measurement file is isolated when the other work fits on the remaining jobs', () => {
  const files = Array.from({ length: 15 }, (_, i) => `${root}${i}.spec.ts`);
  const weights = { desktop: Object.fromEntries(files.map((file, i) => [file, i === 0 ? 450 : 20])) };
  const plan = allocateProbationShards(files, ['desktop'], weights, 8);
  const heavy = plan.find((shard) => shard.assignments.some(({ file }) => file === files[0]));
  assert.equal(heavy.assignments.length, 1);
  assert.equal(Math.max(...plan.map((shard) => shard.seconds)), 450);
  assert.equal(plan.flatMap((shard) => shard.assignments).length, files.length);
});

test('new files and browser projects receive a finite estimate and retain their coverage', () => {
  const files = [`${root}new.spec.ts`];
  const plan = allocateProbationShards(files, ['new-browser'], {}, 8);
  assert.deepEqual(plan.flatMap((shard) => shard.assignments).map(identity), [`new-browser:${files[0]}`]);
  assert.ok(plan.every((shard) => Number.isFinite(shard.seconds)));
  assert.ok(plan.some((shard) => shard.seconds > 0));
  assert.equal(allocateProbationShards([], ['desktop'], {}, 8).flatMap((shard) => shard.assignments).length, 0);
});

test('zero-duration pairs remain assigned and malformed input fails admission', () => {
  const file = `${root}skipped.spec.ts`;
  assert.equal(allocateProbationShards([file], ['desktop'], { desktop: { [file]: 0 } }, 8)
    .flatMap((shard) => shard.assignments).length, 1);
  for (const value of [-1, NaN, Infinity, '30']) {
    assert.throws(() => allocateProbationShards([file], ['desktop'], { desktop: { [file]: value } }, 8));
  }
  assert.throws(() => allocateProbationShards([file, file], ['desktop'], {}, 8));
  assert.throws(() => allocateProbationShards([file], ['desktop', 'desktop'], {}, 8));
  assert.throws(() => allocateProbationShards([`${root}../escape.spec.ts`], ['desktop'], {}, 8));
});

test('shard coordinates reject zero, fractions, unsafe integers and out-of-range jobs', () => {
  assert.deepEqual(parseProbationShard('3/8'), { current: 3, total: 8 });
  for (const value of ['', '0/8', '1/0', '9/8', '1.5/8', ' 1/8', '1/9007199254740992']) {
    assert.throws(() => parseProbationShard(value), /Invalid probation shard/);
  }
});

test('actual path expressions escape metacharacters and empty allocations never select defaults', () => {
  const directory = mkdtempSync(join(tmpdir(), 'probation [literal].'));
  try {
    const file = `${root}${PROBATION_SPECS[0].slice(3)}`;
    mkdirSync(dirname(join(directory, file)), { recursive: true });
    writeFileSync(join(directory, file), '');
    mkdirSync(join(directory, '.github/scripts'), { recursive: true });
    writeFileSync(join(directory, '.github/scripts/e2e-probation-seconds.json'), JSON.stringify({ version: 1, seconds_by_project_file: {} }));
    const projects = ['desktop', 'phone'];
    const counts = Object.fromEntries(projects.map((project) => [project, 0]));
    for (let shard = 1; shard <= 8; shard++) {
      const matches = probationProjectMatches(`${shard}/8`, projects, directory);
      for (const project of projects) {
        if (matches.get(project).some((pattern) => pattern.test(join(directory, file)))) counts[project]++;
        assert.equal(matches.get(project).some((pattern) => pattern.test(join(directory, file.replace('.spec.ts', 'XspecXts')))), false);
        assert.equal(matches.get(project).some((pattern) => pattern.test(join(directory, `${root}unrelated.spec.ts`))), false);
      }
    }
    assert.deepEqual(counts, { desktop: 1, phone: 1 });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
