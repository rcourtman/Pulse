import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { promisify } from 'node:util';
import test from 'node:test';

const execute = promisify(execFile);
// An exact-parent control can select its saved source without changing the
// production runner or replacing the assertions with source-string checks.
const runnerSource = await fs.readFile(
  process.env.PULSE_RUNNER_TEST_SOURCE || new URL('./run-playwright.mjs', import.meta.url),
  'utf8',
);
const phaseScript = `
import fs from 'node:fs';
import path from 'node:path';
const root = process.env.FIXTURE_ROOT;
const phase = process.env.FIXTURE_PHASE;
fs.appendFileSync(path.join(root, 'calls.jsonl'), JSON.stringify({ phase,
  runId: process.env.PULSE_E2E_RUN_ID,
  state: process.env.PULSE_E2E_RUNTIME_STATE_PATH,
  lockPath: process.env.HOT_DEV_VERIFY_LOCK_FILE,
  lock: fs.existsSync(process.env.HOT_DEV_VERIFY_LOCK_FILE)
    ? fs.readFileSync(process.env.HOT_DEV_VERIFY_LOCK_FILE, 'utf8') : null,
  args: process.argv.slice(2),
}) + '\\n');
`;

async function fixture(t, options = {}) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-npm-cleanup-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  await fs.mkdir(path.join(root, 'scripts'));
  await fs.mkdir(path.join(root, 'bin'));
  await fs.writeFile(path.join(root, 'scripts/run-playwright.mjs'), runnerSource);
  await fs.writeFile(path.join(root, 'scripts/pretest.mjs'), `
process.env.FIXTURE_PHASE = 'pretest';
${phaseScript}
fs.writeFileSync(path.join(root, 'partial-runtime'), 'started');
process.exit(${options.pretest ?? 0});
`);
  await fs.writeFile(path.join(root, 'scripts/posttest.mjs'), `
process.env.FIXTURE_PHASE = 'posttest';
${phaseScript}
fs.rmSync(path.join(root, 'partial-runtime'), { force: true });
${options.replaceLock ? "fs.writeFileSync(process.env.HOT_DEV_VERIFY_LOCK_FILE, 'later-owner\\n');" : ''}
process.exit(${options.posttest ?? 0});
`);
  if (!options.missingNpx) {
    await fs.writeFile(path.join(root, 'bin/npx'), `#!${process.execPath}
process.env.FIXTURE_PHASE = 'playwright';
${phaseScript}
process.exit(${options.playwright ?? 0});
`, { mode: 0o755 });
  }
  const lockPath = path.join(root, 'verify.lock');
  if (options.foreignLock) await fs.writeFile(lockPath, options.foreignLock);
  if (options.lockDirectory) await fs.mkdir(lockPath);
  const env = {
    ...process.env,
    PATH: path.join(root, 'bin'),
    FIXTURE_ROOT: root,
    PULSE_E2E_USE_HOT_DEV: options.hotDev === false ? 'false' : 'true',
    PULSE_E2E_RUN_ID: 'fixture-invocation',
    PULSE_E2E_RUNTIME_STATE_PATH: path.join(root, 'runtime-state.json'),
    HOT_DEV_VERIFY_LOCK_FILE: lockPath,
  };
  let result;
  try {
    const output = await execute(process.execPath,
      ['./scripts/run-playwright.mjs', '--project=chromium'],
      { cwd: root, env, timeout: 5000 });
    result = { code: 0, ...output };
  } catch (err) {
    // A killed/timed-out fixture is an error, never the expected nonzero exit.
    assert.equal(err.killed, false, err.message);
    assert.equal(err.signal, null, err.message);
    assert.equal(typeof err.code, 'number', err.message);
    result = { code: err.code, stdout: err.stdout, stderr: err.stderr };
  }
  const calls = await fs.readFile(path.join(root, 'calls.jsonl'), 'utf8')
    .then(raw => raw.trim().split('\n').map(line => JSON.parse(line)))
    .catch(err => { if (err.code === 'ENOENT') return []; throw err; });
  for (const call of calls) {
    assert.equal(call.runId, env.PULSE_E2E_RUN_ID);
    assert.equal(call.state, env.PULSE_E2E_RUNTIME_STATE_PATH);
    assert.equal(call.lockPath, env.HOT_DEV_VERIFY_LOCK_FILE);
  }
  return { root, lockPath, calls, result };
}

async function assertAbsent(file) {
  await assert.rejects(fs.stat(file), { code: 'ENOENT' });
}

test('failed setup tears down its partial runtime and lock without launching tests', async t => {
  const f = await fixture(t, { pretest: 17 });
  assert.equal(f.result.code, 17);
  assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'posttest']);
  assert.match(f.calls[0].lock, /\nrun_id=fixture-invocation\n$/);
  assert.equal(f.calls[1].lock, f.calls[0].lock, 'handoff stays held through teardown');
  await assertAbsent(path.join(f.root, 'partial-runtime'));
  await assertAbsent(f.lockPath);
});

test('failed setup remains failed when teardown also fails', async t => {
  const f = await fixture(t, { pretest: 17, posttest: 23 });
  assert.equal(f.result.code, 17);
  assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'posttest']);
  await assertAbsent(f.lockPath);
});

for (const [playwright, posttest, expected] of [[0, 0, 0], [19, 0, 19], [0, 23, 23], [19, 23, 19]]) {
  test(`test exit ${playwright}, teardown exit ${posttest} retain exit ${expected} and cleanup`, async t => {
    const f = await fixture(t, { playwright, posttest });
    assert.equal(f.result.code, expected);
    assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'playwright', 'posttest']);
    assert.deepEqual(f.calls[1].args, ['playwright', 'test', '--project=chromium']);
    for (const call of f.calls) assert.equal(call.lock, f.calls[0].lock);
    await assertAbsent(path.join(f.root, 'partial-runtime'));
    await assertAbsent(f.lockPath);
  });
}

test('non-hot-dev invocation preserves a verification lock it never acquired', async t => {
  const foreignLock = `pid=${process.pid}\nrun_id=another-live-invocation\n`;
  const f = await fixture(t, { hotDev: false, foreignLock });
  assert.equal(f.result.code, 0);
  assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'playwright', 'posttest']);
  assert.equal(await fs.readFile(f.lockPath, 'utf8'), foreignLock);
});

test('teardown preserves a handoff replaced by a later owner', async t => {
  const f = await fixture(t, { replaceLock: true });
  assert.equal(f.result.code, 0);
  assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'playwright', 'posttest']);
  assert.equal(await fs.readFile(f.lockPath, 'utf8'), 'later-owner\n');
});

test('failed test launch still tears down setup and the acquired lock', async t => {
  const f = await fixture(t, { missingNpx: true });
  assert.equal(f.result.code, 1);
  assert.match(f.result.stderr, /ENOENT/);
  assert.deepEqual(f.calls.map(c => c.phase), ['pretest', 'posttest']);
  await assertAbsent(path.join(f.root, 'partial-runtime'));
  await assertAbsent(f.lockPath);
});

test('failed lock preparation cannot stop a runtime this invocation never started', async t => {
  const f = await fixture(t, { lockDirectory: true });
  assert.equal(f.result.code, 1);
  assert.deepEqual(f.calls, []);
  assert.equal((await fs.stat(f.lockPath)).isDirectory(), true);
});
