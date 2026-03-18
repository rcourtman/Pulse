import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { withExclusiveLock } from '../../../scripts/exclusive-lock.mjs';

test('withExclusiveLock serializes competing callers', async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-exclusive-lock-'));
  const lockPath = path.join(tempRoot, 'build.lock');
  const order = [];
  let releaseFirst;
  const firstHasLock = new Promise((resolve) => {
    releaseFirst = resolve;
  });

  const first = withExclusiveLock(lockPath, async () => {
    order.push('first:start');
    await firstHasLock;
    order.push('first:end');
  });

  await new Promise((resolve) => setTimeout(resolve, 50));

  const second = withExclusiveLock(lockPath, async () => {
    order.push('second:start');
    order.push('second:end');
  });

  await new Promise((resolve) => setTimeout(resolve, 50));
  assert.deepEqual(order, ['first:start']);

  releaseFirst();
  await Promise.all([first, second]);

  assert.deepEqual(order, ['first:start', 'first:end', 'second:start', 'second:end']);
});

test('withExclusiveLock removes stale locks left by dead owners', async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-exclusive-lock-'));
  const lockPath = path.join(tempRoot, 'stale.lock');

  await fs.mkdir(lockPath, { recursive: true });
  await fs.writeFile(
    path.join(lockPath, 'owner.json'),
    `${JSON.stringify({ pid: 999999, createdAt: '2026-03-12T00:00:00Z' })}\n`,
    'utf8',
  );

  let acquired = false;
  await withExclusiveLock(
    lockPath,
    async () => {
      acquired = true;
    },
    { timeoutMs: 1_000, staleAfterMs: 0 },
  );

  assert.equal(acquired, true);
});
