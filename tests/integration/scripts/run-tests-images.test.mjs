import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const runner = fileURLToPath(new URL('./run-tests.sh', import.meta.url));
const repo = path.resolve(path.dirname(runner), '../../..');

function run(failImage = '') {
  const dir = mkdtempSync(path.join(tmpdir(), 'pulse-image-test-'));
  const log = path.join(dir, 'calls');
  try {
    writeFileSync(path.join(dir, 'docker'), `#!/bin/bash
printf '%s\\n' "$*" >> "$CALL_LOG"
if [ "$1 $2" = 'compose version' ]; then
  printf 'target=%s command=%s file=%s path=%s skip=%s\\n' "$PULSE_E2E_PULSE_CONTAINER" "$PULSE_E2E_ENTITLEMENT_WRITE_COMMAND" "$PULSE_E2E_BILLING_STATE_PATH" "$PULSE_E2E_CONTAINER_BILLING_PATH" "$PULSE_E2E_SKIP_DOCKER" >> "$CALL_LOG"
  exit 0
fi
if [ "$1 $2" = 'image inspect' ]; then exit 0; fi
if [ "$1" = build ]; then
  [[ " $* " != *" -t $FAIL_IMAGE:"* ]]
  exit $?
fi
exit 91
`, { mode: 0o755 });
    const result = spawnSync('bash', [runner, failImage ? 'multi-tenant' : 'invalid-test-suite'], {
      env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, CALL_LOG: log, FAIL_IMAGE: failImage,
        PULSE_E2E_PULSE_CONTAINER: 'unowned-container',
        PULSE_E2E_ENTITLEMENT_WRITE_COMMAND: 'unowned-command',
        PULSE_E2E_BILLING_STATE_PATH: '/unowned/billing.json',
        PULSE_E2E_CONTAINER_BILLING_PATH: '/unowned/billing.json',
        PULSE_E2E_SKIP_DOCKER: 'true', },
      encoding: 'utf8',
    });
    const project = result.stdout.match(/Isolated integration project: (pulse-e2e-[a-f0-9]+)/)?.[1];
    return { ...result, project, calls: readFileSync(log, 'utf8').trim().split('\n') };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test('rebuilds both test images from this checkout even when local tags exist', () => {
  const result = run();
  assert.equal(result.status, 1); // Deliberately unknown suite: never starts services.
  assert.ok(result.stdout.includes('Unknown suite: invalid-test-suite'));
  assert.deepEqual(result.calls.filter(call => call.startsWith('build ')), [
    `build -t pulse-mock-github:${result.project} ${repo}/tests/integration/mock-github-server`,
    `build --target e2e_runtime -t pulse:${result.project} --build-arg GO_BUILD_TAGS= -f ${repo}/Dockerfile ${repo}`,
  ]);
});

for (const image of ['pulse-mock-github', 'pulse']) {
  test(`failed ${image} build cannot fall through to a cached image`, () => {
    const result = run(image);
    assert.equal(result.status, 1);
    assert.ok(result.calls.some(call => call.startsWith('build ') && call.includes(`-t ${image}:`)));
    assert.ok(!result.calls.some(call => call.includes(' up ')));
    assert.ok(!result.stdout.includes('Starting test environment'));
  });
}

test('independent invocations never reuse image/project identities', () => {
  const a = run();
  const b = run();
  assert.match(a.project, /^pulse-e2e-[a-f0-9]+$/);
  assert.notEqual(a.project, b.project);
});

test('failed startup cleanup is scoped to its own project', () => {
  const result = run('not-an-image');
  const composeCalls = result.calls.filter(call => call.startsWith('compose ') && call !== 'compose version');
  assert.ok(composeCalls.some(call => call.endsWith('down -v')));
  for (const call of composeCalls) assert.ok(call.includes(`-p ${result.project} `));
});

test('shell runner binds entitlement writes to its owned server and clears external overrides', () => {
  const result = run();
  assert.ok(result.calls.includes(`target=${result.project}-server command= file= path= skip=`));
});
