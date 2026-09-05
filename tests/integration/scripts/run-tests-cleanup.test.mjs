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
          docker: `if [ "$1" = compose ]; then
  case " $* " in
    *" down -v "*) exit ${cleanupStatus} ;;
    *" port pulse-test 7655 "*) echo 127.0.0.1:17655; exit 0 ;;
    *" port pulse-test 7656 "*) echo 127.0.0.1:17656; exit 0 ;;
    *" port mock-github 8080 "*) echo 127.0.0.1:18080; exit 0 ;;
  esac
fi
  case "$1 $2" in
    'inspect -f') echo true ;;
  esac
  exit 0`,
          curl: 'exit 0',
          go: 'exit 0',
          node: `if [ "$1" = -e ]; then printf '%s' 0123456789abcdef0123456789abcdef; fi
exit 0`,
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

for (const failure of ['startup', 'health', 'entitlement', 'port-command', 'port-binding']) {
  test(`${failure} failure with failed teardown stops before the next suite`, () => {
    const dir = mkdtempSync(path.join(tmpdir(), 'pulse-cleanup-early-test-'));
    try {
      const stubs = {
        docker: `if [ "$1" = compose ]; then
  case " $* " in
    *" up -d "*) echo start >> '${dir}/starts'; exit ${failure === 'startup' ? 19 : 0} ;;
    *" down -v "*) exit 17 ;;
    *" port "*) echo ${failure === 'port-binding' ? 'invalid' : '127.0.0.1:17655'}; exit ${failure === 'port-command' ? 24 : 0} ;;
  esac
fi
if [ "$1 $2" = 'inspect -f' ]; then echo ${failure === 'health' ? 'false' : 'true'}; fi
exit 0`,
        curl: 'exit 0',
        node: `if [ "$1" = -e ]; then printf '%s' 0123456789abcdef0123456789abcdef; exit 0; fi
exit ${failure === 'entitlement' ? 23 : 0}`,
        npx: 'exit 0',
        go: 'exit 0',
      };
      for (const [name, body] of Object.entries(stubs)) {
        writeFileSync(path.join(dir, name), `#!/bin/bash\n${body}\n`, { mode: 0o755 });
      }
      const result = spawnSync('bash', [runner, 'all'], {
        env: { ...process.env, PATH: `${dir}:${process.env.PATH}` },
        encoding: 'utf8',
      });
      assert.equal(result.status, 1, result.stdout + result.stderr);
      assert.equal(readFileSync(path.join(dir, 'starts'), 'utf8'), 'start\n',
        'failed setup teardown must not let another suite inherit dirty state');
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
}
