#!/usr/bin/env node

import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const nodeCmd = process.execPath;
const npxCmd = process.platform === 'win32' ? 'npx.cmd' : 'npx';
const playwrightArgs = process.argv.slice(2);
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..', '..');

function buildRunScopedEnv(env = process.env) {
  const configuredRuntimeStatePath = String(env.PULSE_E2E_RUNTIME_STATE_PATH || '').trim();
  const configuredRunId = String(env.PULSE_E2E_RUN_ID || '').trim();
  const configuredVerifyLockPath = String(env.HOT_DEV_VERIFY_LOCK_FILE || '').trim();
  if (configuredRuntimeStatePath !== '') {
    return {
      ...env,
      PULSE_E2E_RUN_ID:
        configuredRunId || path.basename(configuredRuntimeStatePath).replace(/\.[^.]+$/, ''),
      HOT_DEV_VERIFY_LOCK_FILE:
        configuredVerifyLockPath || path.join(repoRoot, 'tmp', 'hot-dev.verify.lock'),
    };
  }

  const runId = `run-${Date.now()}-${process.pid}-${Math.random().toString(36).slice(2, 8)}`;
  return {
    ...env,
    PULSE_E2E_RUN_ID: String(env.PULSE_E2E_RUN_ID || runId).trim(),
    PULSE_E2E_RUNTIME_STATE_PATH: path.join(repoRoot, 'tmp', `${runId}.runtime-state.json`),
    HOT_DEV_VERIFY_LOCK_FILE:
      configuredVerifyLockPath || path.join(repoRoot, 'tmp', 'hot-dev.verify.lock'),
  };
}

function managedVerifyLockPath(env = process.env) {
  const configuredPath = String(env.HOT_DEV_VERIFY_LOCK_FILE || '').trim();
  return configuredPath || path.join(repoRoot, 'tmp', 'hot-dev.verify.lock');
}

async function writeManagedVerifyLock(env = process.env) {
  if (!['1', 'true', 'yes', 'on'].includes(String(env.PULSE_E2E_USE_HOT_DEV || '').trim().toLowerCase())) {
    return;
  }

  const lockPath = managedVerifyLockPath(env);
  const contents = `pid=${process.pid}\ncreated_at=${new Date().toISOString()}\nrun_id=${String(env.PULSE_E2E_RUN_ID || '').trim()}\n`;
  await fs.mkdir(path.dirname(lockPath), { recursive: true });
  await fs.writeFile(lockPath, contents, 'utf8');
  return { lockPath, contents };
}

async function clearManagedVerifyLock(lock) {
  if (!lock) return;
  try {
    // Non-hot-dev runs never acquired this handoff. A later owner may also
    // have replaced it while teardown ran; do not remove that owner's lock.
    if (await fs.readFile(lock.lockPath, 'utf8') === lock.contents) {
      await fs.unlink(lock.lockPath);
    }
  } catch (err) {
    if (err.code !== 'ENOENT') throw err;
  }
}

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
  });

let exitCode = 0;
const childEnv = buildRunScopedEnv();
let verifyLock;
let setupStarted = false;

function reportFailure(phase, err) {
  console.error(`[integration] ${phase} failed:`, err?.message || err);
  if (exitCode === 0) exitCode = 1;
}

try {
  verifyLock = await writeManagedVerifyLock(childEnv);
  setupStarted = true;
  exitCode = await run(nodeCmd, ['./scripts/pretest.mjs'], { env: childEnv });
  // A failed setup may already have started a managed runtime. Keep its
  // failure, skip Playwright, and still reach the existing teardown below.
  if (exitCode === 0) {
    exitCode = await run(npxCmd, ['playwright', 'test', ...playwrightArgs], { env: childEnv });
  }
} catch (err) {
  reportFailure('setup or test launch', err);
} finally {
  try {
    // If lock preparation failed, this invocation never started a runtime
    // and must not stop a pre-existing one through posttest.
    if (setupStarted) {
      const posttestCode = await run(nodeCmd, ['./scripts/posttest.mjs'], { env: childEnv });
      if (exitCode === 0) exitCode = posttestCode;
    }
  } catch (err) {
    reportFailure('teardown launch', err);
  } finally {
    try {
      await clearManagedVerifyLock(verifyLock);
    } catch (err) {
      reportFailure('verification lock cleanup', err);
    }
  }
}

process.exit(exitCode);
