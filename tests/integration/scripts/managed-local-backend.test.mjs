import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import {
  buildManagedLocalBackendEnv,
  buildManagedLocalBackendState,
  shouldBuildManagedLocalBackendFrontend,
  shouldBuildManagedLocalBackendBinary,
} from './managed-local-backend.mjs';

test('buildManagedLocalBackendState uses deterministic defaults', () => {
  const state = buildManagedLocalBackendState({});
  assert.equal(state.repoRoot.endsWith('/repos/pulse'), true);
  assert.equal(state.baseURL, 'http://127.0.0.1:8765');
  assert.equal(state.metricsPort, '0');
  assert.equal(state.billingStatePath, path.join(state.rootDir, 'data', 'billing.json'));
  assert.equal(state.binaryPath, path.join(state.repoRoot, 'pulse'));
  assert.equal(
    state.embeddedFrontendDistPath,
    path.join(state.repoRoot, 'internal', 'api', 'frontend-modern', 'dist'),
  );
});

test('buildManagedLocalBackendEnv seeds auth, bootstrap token, and billing path', () => {
  const state = buildManagedLocalBackendState({
    PULSE_E2E_LOCAL_BACKEND_PORT: '9001',
    PULSE_MULTI_TENANT_ENABLED: 'true',
  });
  const env = buildManagedLocalBackendEnv(state, {
    PULSE_MULTI_TENANT_ENABLED: 'true',
  });

  assert.equal(env.PORT, '9001');
  assert.equal(env.FRONTEND_PORT, '9001');
  assert.equal(env.PULSE_DEV, 'true');
  assert.equal(env.PULSE_DATA_DIR, state.dataDir);
  assert.equal(env.PULSE_E2E_BILLING_STATE_PATH, state.billingStatePath);
  assert.equal(env.PULSE_E2E_BOOTSTRAP_TOKEN.length > 0, true);
  assert.equal(env.PULSE_METRICS_PORT, '0');
  assert.equal('ALLOW_ADMIN_BYPASS' in env, false);
  assert.equal(env.PULSE_MULTI_TENANT_ENABLED, 'true');
  assert.match(env.ALLOWED_ORIGINS, /5173/);
});

test('shouldBuildManagedLocalBackendBinary returns true when binary is missing', async () => {
  const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-backend-'));
  const state = {
    repoRoot,
    binaryPath: path.join(repoRoot, 'pulse'),
    embeddedFrontendDistPath: path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist'),
  };

  await fs.mkdir(path.join(repoRoot, 'cmd', 'pulse'), { recursive: true });
  await fs.writeFile(path.join(repoRoot, 'cmd', 'pulse', 'main.go'), 'package main\n');
  await fs.writeFile(path.join(repoRoot, 'go.mod'), 'module example.com/pulse\n');

  await assert.doesNotReject(async () => {
    assert.equal(await shouldBuildManagedLocalBackendBinary(state), true);
  });
});

test('shouldBuildManagedLocalBackendBinary detects stale source inputs', async () => {
  const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-backend-'));
  const binaryPath = path.join(repoRoot, 'pulse');
  const mainPath = path.join(repoRoot, 'cmd', 'pulse', 'main.go');
  const internalPath = path.join(repoRoot, 'internal', 'api', 'router.go');
  const embeddedIndexPath = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist', 'index.html');
  await fs.mkdir(path.dirname(mainPath), { recursive: true });
  await fs.mkdir(path.dirname(internalPath), { recursive: true });
  await fs.mkdir(path.dirname(embeddedIndexPath), { recursive: true });
  await fs.writeFile(path.join(repoRoot, 'go.mod'), 'module example.com/pulse\n');
  await fs.writeFile(mainPath, 'package main\n');
  await fs.writeFile(internalPath, 'package api\n');
  await fs.writeFile(embeddedIndexPath, '<!doctype html>');
  await fs.writeFile(binaryPath, 'binary');

  const older = new Date('2026-03-12T09:00:00Z');
  const newer = new Date('2026-03-12T10:00:00Z');
  await fs.utimes(binaryPath, older, older);
  await fs.utimes(mainPath, older, older);
  await fs.utimes(internalPath, newer, newer);
  await fs.utimes(embeddedIndexPath, older, older);

  assert.equal(
    await shouldBuildManagedLocalBackendBinary({
      repoRoot,
      binaryPath,
      embeddedFrontendDistPath: path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist'),
    }),
    true,
  );
});

test('shouldBuildManagedLocalBackendBinary skips rebuild when binary is fresh', async () => {
  const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-backend-'));
  const binaryPath = path.join(repoRoot, 'pulse');
  const mainPath = path.join(repoRoot, 'cmd', 'pulse', 'main.go');
  const goModPath = path.join(repoRoot, 'go.mod');
  const embeddedIndexPath = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist', 'index.html');
  await fs.mkdir(path.dirname(mainPath), { recursive: true });
  await fs.mkdir(path.dirname(embeddedIndexPath), { recursive: true });
  await fs.writeFile(goModPath, 'module example.com/pulse\n');
  await fs.writeFile(mainPath, 'package main\n');
  await fs.writeFile(embeddedIndexPath, '<!doctype html>');
  await fs.writeFile(binaryPath, 'binary');

  const older = new Date('2026-03-12T09:00:00Z');
  const newer = new Date('2026-03-12T10:00:00Z');
  await fs.utimes(mainPath, older, older);
  await fs.utimes(goModPath, older, older);
  await fs.utimes(embeddedIndexPath, older, older);
  await fs.utimes(binaryPath, newer, newer);

  assert.equal(
    await shouldBuildManagedLocalBackendBinary({
      repoRoot,
      binaryPath,
      embeddedFrontendDistPath: path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist'),
    }),
    false,
  );
});

test('shouldBuildManagedLocalBackendFrontend rebuilds when frontend sources are newer', async () => {
  const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-backend-'));
  const frontendRoot = path.join(repoRoot, 'frontend-modern');
  const embeddedFrontendDistPath = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist');
  const sourcePath = path.join(frontendRoot, 'src', 'App.tsx');
  const buildOutputPath = path.join(embeddedFrontendDistPath, 'index.html');

  await fs.mkdir(path.dirname(sourcePath), { recursive: true });
  await fs.mkdir(path.dirname(buildOutputPath), { recursive: true });
  await fs.writeFile(path.join(frontendRoot, 'package.json'), '{"name":"pulse-modern"}');
  await fs.writeFile(path.join(frontendRoot, 'index.html'), '<!doctype html>');
  await fs.writeFile(sourcePath, 'export const App = () => null;\n');
  await fs.writeFile(buildOutputPath, '<!doctype html>');

  const older = new Date('2026-03-12T09:00:00Z');
  const newer = new Date('2026-03-12T10:00:00Z');
  await fs.utimes(buildOutputPath, older, older);
  await fs.utimes(path.join(frontendRoot, 'package.json'), older, older);
  await fs.utimes(path.join(frontendRoot, 'index.html'), older, older);
  await fs.utimes(sourcePath, newer, newer);

  assert.equal(
    await shouldBuildManagedLocalBackendFrontend({ frontendRoot, embeddedFrontendDistPath }),
    true,
  );
});

test('shouldBuildManagedLocalBackendFrontend skips rebuild when embedded frontend is fresh', async () => {
  const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-backend-'));
  const frontendRoot = path.join(repoRoot, 'frontend-modern');
  const embeddedFrontendDistPath = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist');
  const sourcePath = path.join(frontendRoot, 'src', 'App.tsx');
  const buildOutputPath = path.join(embeddedFrontendDistPath, 'index.html');

  await fs.mkdir(path.dirname(sourcePath), { recursive: true });
  await fs.mkdir(path.dirname(buildOutputPath), { recursive: true });
  await fs.writeFile(path.join(frontendRoot, 'package.json'), '{"name":"pulse-modern"}');
  await fs.writeFile(path.join(frontendRoot, 'index.html'), '<!doctype html>');
  await fs.writeFile(sourcePath, 'export const App = () => null;\n');
  await fs.writeFile(buildOutputPath, '<!doctype html>');

  const older = new Date('2026-03-12T09:00:00Z');
  const newer = new Date('2026-03-12T10:00:00Z');
  await fs.utimes(sourcePath, older, older);
  await fs.utimes(path.join(frontendRoot, 'package.json'), older, older);
  await fs.utimes(path.join(frontendRoot, 'index.html'), older, older);
  await fs.utimes(buildOutputPath, newer, newer);

  assert.equal(
    await shouldBuildManagedLocalBackendFrontend({ frontendRoot, embeddedFrontendDistPath }),
    false,
  );
});
