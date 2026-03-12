import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';

import {
  buildManagedLocalBackendEnv,
  buildManagedLocalBackendState,
} from './managed-local-backend.mjs';

test('buildManagedLocalBackendState uses deterministic defaults', () => {
  const state = buildManagedLocalBackendState({});
  assert.equal(state.repoRoot.endsWith('/repos/pulse'), true);
  assert.equal(state.baseURL, 'http://127.0.0.1:8765');
  assert.equal(state.billingStatePath, path.join(state.rootDir, 'data', 'billing.json'));
  assert.equal(state.binaryPath, path.join(state.repoRoot, 'pulse'));
});

test('buildManagedLocalBackendEnv seeds auth, bootstrap token, and billing path', () => {
  const state = buildManagedLocalBackendState({
    PULSE_E2E_LOCAL_BACKEND_PORT: '9001',
    PULSE_MULTI_TENANT_ENABLED: 'true',
  });
  const env = buildManagedLocalBackendEnv(state, {
    PULSE_MULTI_TENANT_ENABLED: 'true',
  });

  assert.equal(env.PORT, '9001');
  assert.equal(env.FRONTEND_PORT, '9001');
  assert.equal(env.PULSE_DEV, 'true');
  assert.equal(env.PULSE_DATA_DIR, state.dataDir);
  assert.equal(env.PULSE_E2E_BILLING_STATE_PATH, state.billingStatePath);
  assert.equal(env.PULSE_E2E_BOOTSTRAP_TOKEN.length > 0, true);
  assert.equal(env.PULSE_MULTI_TENANT_ENABLED, 'true');
  assert.match(env.ALLOWED_ORIGINS, /5173/);
});
