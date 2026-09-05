import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const runner = fileURLToPath(new URL('./run-tests.sh', import.meta.url));

for (const suite of ['core', 'all']) {
  for (const cleanupStatus of [0, 17]) {
    test(`${suite} success requires cleanup success (cleanup exit ${cleanupStatus})`, () => {
      const dir = mkdtempSync(path.join(tmpdir(), 'pulse-cleanup-test-'));
      try {
        const stubs = {
          docker: `if [ "$1 $2 $4" = "compose -f down" ]; then exit ${cleanupStatus}; fi
  case "$1 $2" in
    'inspect -f') echo true ;;
  esac
  exit 0`,
          curl: 'exit 0',
          go: 'exit 0',
          node: 'exit 0',
          npx: `echo test >> '${dir}/tests-run'\nexit 0`,
        };
        for (const [name, body] of Object.entries(stubs)) {
          writeFileSync(path.join(dir, name), `#!/bin/bash\n${body}\n`, { mode: 0o755 });
        }
        const result = spawnSync('bash', [runner, suite], {
          env: { ...process.env, PATH: `${dir}:${process.env.PATH}` },
          encoding: 'utf8',
        });
        assert.equal(result.status, cleanupStatus === 0 ? 0 : 1, result.stdout + result.stderr);
        assert.equal(result.stdout.includes('All tests passed!'), cleanupStatus === 0);
        if (cleanupStatus !== 0) {
          assert.equal(readFileSync(path.join(dir, 'tests-run'), 'utf8'), 'test\n',
            'cleanup failure must stop before another suite can inherit dirty state');
        }
      } finally {
        rmSync(dir, { recursive: true, force: true });
      }
    });
  }
}
