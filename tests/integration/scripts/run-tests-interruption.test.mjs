import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, existsSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const runner = fileURLToPath(new URL('./run-tests.sh', import.meta.url));
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

for (const [signal, expected] of [['SIGTERM', 143], ['SIGINT', 130], ['SIGHUP', 129]]) {
  test(`shell ${signal} waits for writers and removes only invocation-owned artifacts`, { timeout: 15000 }, async () => {
    const dir = mkdtempSync(path.join(tmpdir(), 'pulse-run-signal-'));
    let child;
    try {
      const stubs = {
        docker: `echo "$*" >> "$FIXTURE_ROOT/docker-calls"
case " $* " in
  *" port "*) echo 127.0.0.1:17655 ;;
  *" inspect -f "*) echo true ;;
esac
exit 0`,
        curl: 'exit 0',
        node: 'exit 0',
        npx: `mkdir -p "$PULSE_E2E_REPORT_DIR" "$PULSE_E2E_RESULTS_DIR"
printf fixture > "$PULSE_E2E_COOKIE_STATE_PATH"
printf fixture > "$PULSE_E2E_REPORT_DIR/report"
printf fixture > "$PULSE_E2E_RESULTS_DIR/video"
printf '%s\\n' "$PULSE_E2E_COOKIE_STATE_PATH" "$PULSE_E2E_REPORT_DIR" "$PULSE_E2E_RESULTS_DIR" > "$FIXTURE_ROOT/paths"
trap 'sleep .2; echo late > "$PULSE_E2E_RESULTS_DIR/late"; echo stopped > "$FIXTURE_ROOT/writer-stopped"; exit 0' TERM
while true; do sleep .05; done`,
      };
      for (const [name, body] of Object.entries(stubs)) {
        writeFileSync(path.join(dir, name), `#!/bin/bash\n${body}\n`, { mode: 0o755 });
      }
      const unowned = path.join(dir, 'unowned-cookie');
      writeFileSync(unowned, 'keep');
      child = spawn('bash', [runner, 'multi-tenant'], {
        env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, FIXTURE_ROOT: dir,
          PULSE_E2E_COOKIE_STATE_PATH: unowned }, stdio: 'ignore',
      });
      const closed = new Promise(resolve => child.once('close', (code, sig) => resolve({ code, sig })));
      const deadline = Date.now() + 8000;
      while (!existsSync(path.join(dir, 'paths'))) {
        assert.ok(Date.now() < deadline && child.exitCode === null, 'writer must start');
        await sleep(20);
      }
      // Let the stub install its TERM handler after publishing the paths.
      await sleep(100);
      child.kill(signal);
      assert.deepEqual(await closed, { code: expected, sig: null });
      assert.equal(readFileSync(path.join(dir, 'writer-stopped'), 'utf8'), 'stopped\n');
      for (const [index, value] of readFileSync(path.join(dir, 'paths'), 'utf8').trim().split('\n').entries()) {
        assert.equal(existsSync(index === 0 ? path.dirname(value) : value), false, value);
      }
      assert.equal(readFileSync(unowned, 'utf8'), 'keep');
      assert.match(readFileSync(path.join(dir, 'docker-calls'), 'utf8'), /down -v/);
    } finally {
      if (child?.exitCode === null) child.kill('SIGTERM');
      rmSync(dir, { recursive: true, force: true });
    }
  });
}
