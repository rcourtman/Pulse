import assert from 'node:assert/strict';
import test from 'node:test';

import {
  assertHostedTenantRuntimeExists,
  hostedTenantContainerName,
  hostedTenantRuntimeExistsScript,
  restartHostedTenantRuntime,
} from './hosted-tenant-runtime.mjs';
import { parseArgs as parseApprovalStoreArgs } from './hosted-tenant-approval-store.mjs';
import { parseArgs as parseRuntimeCheckArgs } from './hosted-tenant-runtime-check.mjs';
import { parseArgs as parseRuntimeRestartArgs } from './hosted-tenant-runtime-restart.mjs';

test('hostedTenantContainerName derives the canonical runtime container name', () => {
  assert.equal(hostedTenantContainerName(' t-P62TP8K28Y '), 'pulse-t-P62TP8K28Y');
});

test('hostedTenantRuntimeExistsScript checks for the tenant container before reporting state', () => {
  const script = hostedTenantRuntimeExistsScript('t-P62TP8K28Y');

  assert.match(script, /docker inspect "\$container" >\/dev\/null 2>&1/);
  assert.match(script, /hosted tenant runtime container \$container does not exist/);
  assert.match(script, /docker inspect --format/);
});

test('assertHostedTenantRuntimeExists wraps missing runtime failures with proof guidance', () => {
  const calls = [];
  const runner = (host, command) => {
    calls.push({ command, host });
    const error = new Error('ssh failed');
    error.stderr = 'hosted tenant runtime container pulse-t-missing does not exist';
    throw error;
  };

  assert.throws(
    () => assertHostedTenantRuntimeExists('root@pulse-cloud', 't-missing', runner),
    /Hosted mobile proof seeding requires an active hosted tenant container/,
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0].host, 'root@pulse-cloud');
  assert.match(calls[0].command, /docker inspect/);
});

test('assertHostedTenantRuntimeExists prefers remote stderr over noisy ssh command text', () => {
  const runner = () => {
    const error = new Error('Command failed: ssh root@pulse-cloud sh -lc very-long-command');
    error.stderr = 'hosted tenant runtime container pulse-t-missing does not exist';
    throw error;
  };

  assert.throws(
    () => assertHostedTenantRuntimeExists('root@pulse-cloud', 't-missing', runner),
    (error) => {
      assert.match(error.message, /hosted tenant runtime container pulse-t-missing does not exist/);
      assert.doesNotMatch(error.message, /very-long-command/);
      return true;
    },
  );
});

test('restartHostedTenantRuntime checks runtime existence before restart', () => {
  const calls = [];
  const runner = (host, command) => {
    calls.push({ command, host });
    return '';
  };

  restartHostedTenantRuntime('root@pulse-cloud', 't-P62TP8K28Y', runner);

  assert.equal(calls.length, 1);
  assert.equal(calls[0].host, 'root@pulse-cloud');
  assert.match(calls[0].command, /docker inspect/);
  assert.match(calls[0].command, /docker restart/);
});

test('hosted tenant runtime check CLI defaults to the production cloud host', () => {
  const parsed = parseRuntimeCheckArgs(['--tenant-id', 't-canary']);

  assert.equal(parsed.cloudHost, 'root@pulse-cloud');
  assert.equal(parsed.tenantId, 't-canary');
});

test('hosted tenant runtime restart CLI defaults to the production cloud host', () => {
  const parsed = parseRuntimeRestartArgs(['--tenant-id', 't-canary']);

  assert.equal(parsed.cloudHost, 'root@pulse-cloud');
  assert.equal(parsed.tenantId, 't-canary');
});

test('hosted approval seeding keeps runtime restart enabled by default', () => {
  const parsed = parseApprovalStoreArgs([
    'create',
    '--tenant-id',
    't-canary',
    '--approval-id',
    'approval-123',
  ]);

  assert.equal(parsed.restartAfterCreate, true);
  assert.deepEqual(parsed.passthrough, ['--approval-id', 'approval-123']);
});

test('hosted approval seeding can defer runtime restart for transactional proof setup', () => {
  const parsed = parseApprovalStoreArgs([
    'create',
    '--tenant-id',
    't-canary',
    '--no-restart',
    '--approval-id',
    'approval-123',
  ]);

  assert.equal(parsed.restartAfterCreate, false);
  assert.deepEqual(parsed.passthrough, ['--approval-id', 'approval-123']);
});
