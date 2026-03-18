import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import {
  clearRuntimeState,
  getRuntimeStatePath,
  readRuntimeState,
  writeRuntimeState,
} from './runtime-state.mjs';

test('getRuntimeStatePath uses repo default when override is absent', () => {
  const runtimeStatePath = getRuntimeStatePath({});
  assert.equal(runtimeStatePath.endsWith(path.join('tmp', 'e2e-runtime-state.json')), true);
});

test('runtime-state helpers respect PULSE_E2E_RUNTIME_STATE_PATH', async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-runtime-state-'));
  const runtimeStatePath = path.join(tempRoot, 'nested', 'state.json');
  const env = {
    PULSE_E2E_RUNTIME_STATE_PATH: runtimeStatePath,
  };

  await writeRuntimeState({ baseURL: 'http://127.0.0.1:9999' }, env);
  assert.deepEqual(await readRuntimeState(env), { baseURL: 'http://127.0.0.1:9999' });

  await clearRuntimeState(env);
  assert.equal(await readRuntimeState(env), null);
});
