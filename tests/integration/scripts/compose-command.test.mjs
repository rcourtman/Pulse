import test from 'node:test';
import assert from 'node:assert/strict';

import { resolveComposeInvocation } from './compose-command.mjs';

test('resolveComposeInvocation prefers docker compose when available', async () => {
  const invocation = await resolveComposeInvocation(async (command, args) => {
    return command === 'docker' && args.join(' ') === 'compose version';
  });

  assert.deepEqual(invocation, {
    command: 'docker',
    args: ['compose', '-f', 'docker-compose.test.yml', 'up', '-d'],
    label: 'modern docker compose',
  });
});

test('resolveComposeInvocation falls back to docker-compose', async () => {
  const invocation = await resolveComposeInvocation(async (command) => command === 'docker-compose');

  assert.deepEqual(invocation, {
    command: 'docker-compose',
    args: ['-f', 'docker-compose.test.yml', 'up', '-d'],
    label: 'legacy docker-compose',
  });
});

test('resolveComposeInvocation fails with an actionable message when Docker is unavailable', async () => {
  await assert.rejects(
    () => resolveComposeInvocation(async () => false),
    /Neither `docker compose` nor `docker-compose` is available/,
  );
});
