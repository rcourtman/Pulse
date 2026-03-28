import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import {
  managedVerifyLockActive,
  shouldRestartManagedDevRuntimeForVerification,
} from './managed-dev-runtime.mjs';

test('managedVerifyLockActive returns true for a live verify lock owner', async () => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-dev-runtime-'));
  const lockPath = path.join(rootDir, 'hot-dev.verify.lock');
  await fs.writeFile(lockPath, `pid=${process.pid}\ncreated_at=2026-03-28T23:00:00Z\n`, 'utf8');

  assert.equal(managedVerifyLockActive({ HOT_DEV_VERIFY_LOCK_FILE: lockPath }), true);
});

test('managedVerifyLockActive clears false when the verify lock owner is stale', async () => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-dev-runtime-'));
  const lockPath = path.join(rootDir, 'hot-dev.verify.lock');
  await fs.writeFile(lockPath, 'pid=999999\ncreated_at=2026-03-28T23:00:00Z\n', 'utf8');

  assert.equal(managedVerifyLockActive({ HOT_DEV_VERIFY_LOCK_FILE: lockPath }), false);
});

test('shouldRestartManagedDevRuntimeForVerification only restarts existing sessions under verify lock', async () => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-dev-runtime-'));
  const lockPath = path.join(rootDir, 'hot-dev.verify.lock');
  await fs.writeFile(lockPath, `pid=${process.pid}\ncreated_at=2026-03-28T23:00:00Z\n`, 'utf8');

  assert.equal(
    shouldRestartManagedDevRuntimeForVerification({
      env: { HOT_DEV_VERIFY_LOCK_FILE: lockPath },
      wasRunning: true,
    }),
    true,
  );
  assert.equal(
    shouldRestartManagedDevRuntimeForVerification({
      env: { HOT_DEV_VERIFY_LOCK_FILE: lockPath },
      wasRunning: false,
    }),
    false,
  );
});
