import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import {
  applyRequestedEntitlementProfile,
  buildBillingState,
  resolveEntitlementProfile,
  resolveEntitlementTarget,
} from './entitlement-bootstrap.mjs';

test('resolveEntitlementProfile infers multi-tenant from feature flag', () => {
  assert.deepEqual(
    resolveEntitlementProfile({ PULSE_MULTI_TENANT_ENABLED: 'true' }),
    { profile: 'multi-tenant', explicit: false },
  );
});

test('resolveEntitlementTarget prefers explicit file path in skip-docker mode', () => {
  assert.deepEqual(
    resolveEntitlementTarget({
      PULSE_E2E_SKIP_DOCKER: '1',
      PULSE_E2E_BILLING_STATE_PATH: '/tmp/pulse/billing.json',
    }),
    { kind: 'file', path: '/tmp/pulse/billing.json' },
  );
});

test('buildBillingState returns enterprise capabilities for multi-tenant profile', () => {
  const state = buildBillingState('multi-tenant');
  assert.equal(state.plan_version, 'enterprise_eval');
  assert.equal(state.subscription_state, 'active');
  assert.ok(state.capabilities.includes('multi_tenant'));
  assert.ok(state.capabilities.includes('rbac'));
});

test('applyRequestedEntitlementProfile writes a billing state file when a path is provided', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-entitlement-'));
  const billingPath = path.join(dir, 'billing.json');

  const result = await applyRequestedEntitlementProfile({
    env: {
      PULSE_E2E_SKIP_DOCKER: '1',
      PULSE_E2E_ENTITLEMENT_PROFILE: 'multi-tenant',
      PULSE_E2E_BILLING_STATE_PATH: billingPath,
    },
    logger: { log() {}, warn() {} },
  });

  assert.equal(result.applied, true);
  const raw = JSON.parse(await fs.readFile(billingPath, 'utf8'));
  assert.equal(raw.plan_version, 'enterprise_eval');
  assert.ok(raw.capabilities.includes('multi_tenant'));
});

test('applyRequestedEntitlementProfile uses docker exec for managed docker runs', async () => {
  const calls = [];
  await applyRequestedEntitlementProfile({
    env: {
      PULSE_E2E_ENTITLEMENT_PROFILE: 'multi-tenant',
    },
    logger: { log() {}, warn() {} },
    run: async (command, args, options = {}) => {
      calls.push({ command, args, input: options.input });
    },
  });

  assert.equal(calls.length, 1);
  assert.equal(calls[0].command, 'docker');
  assert.deepEqual(calls[0].args.slice(0, 5), ['exec', '-i', 'pulse-test-server', 'sh', '-lc']);
  assert.match(calls[0].args[5], /\/data\/billing\.json/);
  assert.match(calls[0].input, /"multi_tenant"/);
});

test('applyRequestedEntitlementProfile warns instead of failing for inferred local live-instance runs', async () => {
  const warnings = [];
  const result = await applyRequestedEntitlementProfile({
    env: {
      PULSE_E2E_SKIP_DOCKER: '1',
      PULSE_MULTI_TENANT_ENABLED: 'true',
    },
    logger: {
      log() {},
      warn(message) {
        warnings.push(message);
      },
    },
  });

  assert.equal(result.applied, false);
  assert.equal(result.reason, 'no_target_configured');
  assert.equal(warnings.length, 1);
});

test('applyRequestedEntitlementProfile fails for explicit live-instance runs without a write target', async () => {
  await assert.rejects(
    () =>
      applyRequestedEntitlementProfile({
        env: {
          PULSE_E2E_SKIP_DOCKER: '1',
          PULSE_E2E_ENTITLEMENT_PROFILE: 'multi-tenant',
        },
        logger: { log() {}, warn() {} },
      }),
    /no entitlement write target/i,
  );
});
