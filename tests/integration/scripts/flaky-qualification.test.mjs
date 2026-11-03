import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const integrationRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

test('installed Playwright returns failure for a passing retry only when qualification requires it', { timeout: 45_000 }, () => {
  const fixtureRoot = path.join(integrationRoot, 'tmp');
  fs.mkdirSync(fixtureRoot, { recursive: true });
  const directory = fs.mkdtempSync(path.join(fixtureRoot, 'flaky-qualification-'));
  try {
    const cli = createRequire(path.join(integrationRoot, 'package.json')).resolve('@playwright/test/cli');
    fs.writeFileSync(path.join(directory, 'playwright.config.mjs'), 'export default ' + JSON.stringify({
      testDir: directory, testMatch: 'retry.spec.ts', workers: 1, retries: 1,
      timeout: 1000, globalTimeout: 10000, outputDir: path.join(directory, 'results'),
    }) + ';\n');
    fs.writeFileSync(path.join(directory, 'retry.spec.ts'), [
      'import { test, expect } from "@playwright/test";',
      'test("controlled retry", async ({}, info) => {',
      '  expect(process.env.PULSE_FLAKY_CONTROL_CLEAN === "true" || info.retry > 0).toBe(true);',
      '});',
    ].join('\n'));
    // No browser fixture, Docker, network, npm lifecycle or application server.
    // This executes the pinned runner's real retry classification and exit code.
    for (const [strict, clean, status, flaky, passed] of [
      [false, false, 0, 1, 0],
      [true, false, 1, 1, 0],
      [true, true, 0, 0, 1],
    ]) {
      const result = spawnSync(process.execPath, [cli, 'test', '--config=' + path.join(directory, 'playwright.config.mjs'),
        '--reporter=json', ...(strict ? ['--fail-on-flaky-tests'] : [])], {
        cwd: integrationRoot, encoding: 'utf8', timeout: 15_000,
        env: { ...process.env, PWTEST_CACHE_DIR: path.join(directory, 'cache'), PULSE_FLAKY_CONTROL_CLEAN: String(clean) },
      });
      assert.ifError(result.error);
      assert.equal(result.signal, null, result.stdout + result.stderr);
      assert.equal(result.status, status, result.stdout + result.stderr);
      const report = JSON.parse(result.stdout);
      assert.deepEqual({ flaky: report.stats.flaky, passed: report.stats.expected, failed: report.stats.unexpected,
        skipped: report.stats.skipped }, { flaky, passed, failed: 0, skipped: 0 }, result.stdout);
    }
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
});
