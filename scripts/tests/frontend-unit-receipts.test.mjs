import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { test } from 'node:test';
import CIUnitReceipts from '../../frontend-modern/scripts/ci-unit-receipts.mjs';

const sha = 'a'.repeat(40);

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'unit-receipts-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const lock = '{"synthetic":"lock"}\n';
  fs.writeFileSync(path.join(root, 'package-lock.json'), lock);
  const outputFile = path.join(root, 'evidence', 'units.json');
  const reporter = new CIUnitReceipts({ outputFile, sourceSha: sha });
  const ctx = { config: { root, shard: { index: 2, count: 4 } } };
  const spec = name => ({
    taskId: name, moduleId: path.join(root, `${name}.test.ts`),
    project: { name: '' }, pool: 'forks',
  });
  const module = (name, state = 'passed', states = ['passed'], errors = []) => ({
    id: name, state: () => state, errors: () => errors,
    children: { *allTests() {
      for (const state of states) yield {
        result: () => ({ state, errors: [{ message: 'SYNTHETIC_PRIVATE_ERROR' }] }),
        fullName: 'SYNTHETIC_PRIVATE_NAME',
      };
    } },
  });
  const read = () => JSON.parse(fs.readFileSync(outputFile, 'utf8'));
  reporter.onInit(ctx);
  return { root, lock, outputFile, reporter, ctx, spec, module, read };
}

test('initial receipt binds source, lock and actual shard, with no invented inventory or exit', t => {
  const f = fixture(t);
  const receipt = f.read();
  assert.equal(receipt.source_sha, sha);
  assert.equal(receipt.package_lock_sha256, createHash('sha256').update(f.lock).digest('hex'));
  assert.deepEqual(receipt.shard, { index: 2, count: 4 });
  assert.equal(receipt.inventory_observed, false);
  assert.equal(receipt.collection_complete, false);
  assert.equal(receipt.test_run_end_observed, false);
  assert.equal(receipt.test_run_reason, null);
  assert.equal(receipt.unhandled_error_count, null);
  assert.deepEqual(receipt.modules, []);
  assert.equal('exit_code' in receipt, false);
  assert.equal('passed' in receipt, false);
  assert.equal(fs.statSync(f.outputFile).mode & 0o777, 0o600);
});

test('completed failure and success survive while another selected module is unobserved', t => {
  const { reporter, spec, module, read, outputFile } = fixture(t);
  reporter.onTestRunStart(['z-held', 'b-pass', 'a-fail'].map(spec));
  reporter.onTestModuleEnd(module('b-pass', 'passed', ['passed', 'skipped']));
  reporter.onTestModuleEnd(module('a-fail', 'failed', ['failed'], [{ message: 'SYNTHETIC_PRIVATE_ERROR' }]));
  const receipt = read();
  assert.deepEqual(receipt.modules.map(m => m.id), ['a-fail', 'b-pass', 'z-held']);
  assert.deepEqual(receipt.unobserved_modules, ['z-held']);
  assert.equal(receipt.collection_complete, false);
  assert.equal(receipt.test_run_end_observed, false);
  assert.equal(receipt.modules[0].state, 'failed');
  assert.equal(receipt.modules[0].module_error_count, 1);
  assert.equal(receipt.modules[1].tests.skipped, 1);
  assert.equal(receipt.modules[2].tests, null);
  const bytes = fs.readFileSync(outputFile, 'utf8');
  assert.ok(!bytes.includes('SYNTHETIC_PRIVATE'));
  assert.ok(!bytes.includes('message'));
});

test('complete failed run remains failed, including collection and unhandled errors', t => {
  const { reporter, spec, module, read } = fixture(t);
  const before = process.exitCode;
  reporter.onTestRunStart(['bad-import', 'skip', 'pass'].map(spec));
  reporter.onTestRunEnd([
    module('bad-import', 'failed', [], [{ message: 'secret' }]),
    module('skip', 'skipped', ['skipped']), module('pass'),
  ], [{ message: 'secret' }], 'failed');
  const receipt = read();
  assert.equal(receipt.collection_complete, true);
  assert.equal(receipt.test_run_reason, 'failed');
  assert.equal(receipt.unhandled_error_count, 1);
  assert.deepEqual(receipt.modules[0].tests, { passed: 0, failed: 0, skipped: 0, pending: 0 });
  assert.equal(receipt.modules[0].module_error_count, 1);
  assert.equal(process.exitCode, before, 'the reporter must never replace the test runner verdict');
});

test('an interrupted end, missing results or an empty inventory cannot claim complete proof', t => {
  const { reporter, spec, module, read } = fixture(t);
  reporter.onTestRunStart(['pass', 'held'].map(spec));
  reporter.onTestRunEnd([module('pass')], [], 'interrupted');
  assert.equal(read().collection_complete, false);
  assert.deepEqual(read().unobserved_modules, ['held']);
  reporter.onTestRunEnd([], [], 'failed');
  assert.equal(read().collection_complete, false);
  reporter.onTestRunStart([]);
  assert.equal(read().test_run_end_observed, false);
  assert.equal(read().test_run_reason, null);
  reporter.onTestRunEnd([], [], 'passed');
  assert.equal(read().collection_complete, false);
});

test('queued and pending are observations, not terminal completions', t => {
  const { reporter, spec, module, read } = fixture(t);
  reporter.onTestRunStart(['held'].map(spec));
  for (const state of ['queued', 'pending']) {
    reporter.onTestRunEnd([module('held', state, ['pending'])], [], 'failed');
    assert.equal(read().modules[0].state, state);
    assert.equal(read().modules[0].tests.pending, 1);
    assert.equal(read().collection_complete, false);
    assert.deepEqual(read().unobserved_modules, ['held']);
  }
});

test('all selected terminal modules and the run end are required for a complete observation', t => {
  const { reporter, spec, module, read } = fixture(t);
  reporter.onTestRunStart(['one', 'two'].map(spec));
  reporter.onTestModuleEnd(module('one'));
  reporter.onTestModuleEnd(module('two'));
  assert.equal(read().collection_complete, false);
  reporter.onTestRunEnd([module('one'), module('two')], [], 'passed');
  assert.equal(read().collection_complete, true);
  assert.equal(read().test_run_reason, 'passed');
  assert.equal(read().unhandled_error_count, 0);
  assert.deepEqual(read().unobserved_modules, []);
});

test('unknown results, duplicate inventories and outside-root paths fail without replacing proof', t => {
  const { reporter, spec, module, read, root } = fixture(t);
  reporter.onTestRunStart(['known'].map(spec));
  const before = read();
  for (const bad of [
    () => reporter.onTestModuleEnd(module('unknown')),
    () => reporter.onTestRunStart([spec('known'), spec('known')]),
    () => reporter.onTestRunStart([{ ...spec('outside'), moduleId: path.join(root, '..', 'outside.ts') }]),
    () => reporter.onTestModuleEnd(module('known', 'unexpected')),
    () => reporter.onTestModuleEnd(module('known', 'passed', ['unexpected'])),
    () => reporter.onTestRunEnd([], [], 'unknown'),
  ]) {
    assert.throws(bad);
    assert.deepEqual(read(), before);
  }
});

test('a failed atomic write preserves the last complete JSON snapshot', t => {
  const { reporter, spec, module, read, outputFile } = fixture(t);
  reporter.onTestRunStart(['one'].map(spec));
  const before = read();
  const temporary = `${outputFile}.${process.pid}.tmp`;
  fs.mkdirSync(temporary);
  assert.throws(() => reporter.onTestModuleEnd(module('one')));
  assert.deepEqual(read(), before);
  assert.equal(fs.statSync(temporary).isDirectory(), true, 'do not remove an unowned temporary');
});

test('missing source binding and stale output refuse rather than reusing prior proof', t => {
  assert.throws(() => new CIUnitReceipts({ outputFile: '/unused', sourceSha: 'main' }));
  assert.throws(() => new CIUnitReceipts({ outputFile: '/unused', sourceSha: '' }));
  const { outputFile, ctx, read } = fixture(t);
  const before = read();
  assert.throws(() => new CIUnitReceipts({ outputFile, sourceSha: sha }).onInit(ctx));
  assert.deepEqual(read(), before);
});
