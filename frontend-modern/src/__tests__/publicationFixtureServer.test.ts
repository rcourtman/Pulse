import { execFile } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { expect, it } from 'vitest';

const run = promisify(execFile);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

// Keep the server's existing node:test suite on its own runner, but make its
// exit part of npm test. The suite owns its synthetic files, listener and
// cleanup checks.
it('passes the standalone publication fixture server suite', async () => {
  const { stdout, stderr } = await run(
    process.execPath,
    ['--test', '--test-reporter=tap', 'browser-tests/publication-fixture-server.test.cjs'],
    { cwd: root, timeout: 25000, maxBuffer: 1024 * 1024 },
  );
  expect(stdout).toContain('# fail 0');
  expect(stdout).toContain(
    'serves only verified bytes, explicit routes and GET/HEAD; closes the listener',
  );
  // Retain the Node test count and outcomes beside the enclosing Vitest result.
  process.stdout.write(stdout);
  if (stderr) process.stderr.write(stderr);
}, 30000);
