import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import test from 'node:test';
const script = new URL('./with-offline-entitlements.mjs', import.meta.url).pathname;
function run(args, extra = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [script, ...args], {
      env: { PATH: process.env.PATH, HOME: process.env.HOME, PULSE_E2E_USE_LOCAL_BACKEND: 'true', ...extra },
    });
    let output = '';
    child.stdout.on('data', data => { output += data; });
    child.stderr.on('data', data => { output += data; });
    child.on('error', reject);
    child.on('exit', code => resolve({ code, output }));
  });
}
test('wrapper keeps issuer alive for child and closes it after child failure', async () => {
  const result = await run([process.execPath, '-e', `
    const assert = require('node:assert/strict');
    assert.equal(process.env.PULSE_LICENSE_DEV_MODE, 'false');
    assert.equal(process.env.PULSE_MOCK_MODE, 'false');
    fetch(process.env.PULSE_LICENSE_SERVER_URL + '/v1/activate', {
      method: 'POST', body: JSON.stringify({activation_key: process.env.PULSE_E2E_OFFLINE_ACTIVATION_KEY,
      instance_fingerprint: 'test', runtime: {build: 'community'}})
    }).then(r => { assert.equal(r.status, 201); console.log(process.env.PULSE_LICENSE_SERVER_URL); process.exitCode = 7; });
  `]);
  assert.equal(result.code, 7, result.output);
  const url = result.output.trim().split('\n')[0];
  await assert.rejects(fetch(url));
});
test('wrapper refuses remote, supplied binaries and release build flags before spawning', async () => {
  for (const extra of [{ PULSE_BASE_URL: 'https://example.invalid' }, { PULSE_E2E_LOCAL_BACKEND_BINARY: '/tmp/release' }, { GOFLAGS: '-tags=release' }, { PULSE_E2E_USE_LOCAL_BACKEND: 'false' }]) {
    const result = await run([process.execPath, '-e', "console.log('CHILD RAN')"], extra);
    assert.notEqual(result.code, 0); assert.ok(!result.output.includes('\nCHILD RAN\n'));
  }
});
