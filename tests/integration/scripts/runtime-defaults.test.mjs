import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import {
  managedDevBrowserBaseURL,
  preferredBrowserBaseURL,
  preferredPlaywrightRouteBaseURL,
  runtimeStatePath,
} from '../tests/runtime-defaults.ts';

test('preferredBrowserBaseURL lets PLAYWRIGHT_BASE_URL override the browser target', () => {
  const resolved = preferredBrowserBaseURL({
    PLAYWRIGHT_BASE_URL: 'http://127.0.0.1:4174',
    PULSE_BASE_URL: 'http://127.0.0.1:7655',
  });

  assert.equal(resolved, 'http://127.0.0.1:4174');
});

test('preferredBrowserBaseURL still falls back to PULSE_BASE_URL when no browser override exists', () => {
  const resolved = preferredBrowserBaseURL({
    PULSE_BASE_URL: 'http://127.0.0.1:7655',
  });

  assert.equal(resolved, 'http://127.0.0.1:7655');
});

test('preferredPlaywrightRouteBaseURL honors explicit per-scenario overrides before shared browser defaults', () => {
  const resolved = preferredPlaywrightRouteBaseURL(
    {
      PLAYWRIGHT_BASE_URL: 'http://127.0.0.1:4174',
      PULSE_BASE_URL: 'http://127.0.0.1:7655',
    },
    ['https://cloud.example.test///'],
  );

  assert.equal(resolved, 'https://cloud.example.test');
});

test('preferredPlaywrightRouteBaseURL normalizes the selected browser target for route concatenation', () => {
  const resolved = preferredPlaywrightRouteBaseURL({
    PLAYWRIGHT_BASE_URL: 'http://127.0.0.1:4174///',
  });

  assert.equal(resolved, 'http://127.0.0.1:4174');
});

test('runtimeStatePath resolves the default path from an overridden integration repo root', () => {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-runtime-defaults-'));

  try {
    assert.equal(
      runtimeStatePath({
        PULSE_E2E_REPO_ROOT: repoRoot,
      }),
      path.join(repoRoot, 'tmp', 'e2e-runtime-state.json'),
    );
  } finally {
    fs.rmSync(repoRoot, { recursive: true, force: true });
  }
});

test('managedDevBrowserBaseURL keeps the configured host and port when a managed session exists', () => {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-managed-dev-runtime-'));
  fs.mkdirSync(path.join(repoRoot, 'tmp'), { recursive: true });
  fs.writeFileSync(path.join(repoRoot, 'tmp', 'hot-dev.bg.pid'), `${process.pid}\n`);

  try {
    const resolved = managedDevBrowserBaseURL({
      FRONTEND_DEV_HOST: '127.0.0.1',
      FRONTEND_DEV_PORT: '4174',
      PULSE_E2E_REPO_ROOT: repoRoot,
    });

    assert.equal(resolved, 'http://127.0.0.1:4174');
  } finally {
    fs.rmSync(repoRoot, { recursive: true, force: true });
  }
});
