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
  if (configuredRuntimeStatePath !== '') {
    return {
      ...env,
      PULSE_E2E_RUN_ID:
        configuredRunId || path.basename(configuredRuntimeStatePath).replace(/\.[^.]+$/, ''),
    };
  }

  const runId = `run-${Date.now()}-${process.pid}-${Math.random().toString(36).slice(2, 8)}`;
  return {
    ...env,
    PULSE_E2E_RUN_ID: String(env.PULSE_E2E_RUN_ID || runId).trim(),
    PULSE_E2E_RUNTIME_STATE_PATH: path.join(repoRoot, 'tmp', `${runId}.runtime-state.json`),
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
  await fs.mkdir(path.dirname(lockPath), { recursive: true });
  await fs.writeFile(
    lockPath,
    `pid=${process.pid}\ncreated_at=${new Date().toISOString()}\nrun_id=${String(env.PULSE_E2E_RUN_ID || '').trim()}\n`,
    'utf8',
  );
}

async function clearManagedVerifyLock(env = process.env) {
  const lockPath = managedVerifyLockPath(env);
  await fs.rm(lockPath, { force: true });
}

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
  });

let exitCode = 0;
const childEnv = buildRunScopedEnv();

try {
  await writeManagedVerifyLock(childEnv);
  const pretestCode = await run(nodeCmd, ['./scripts/pretest.mjs'], { env: childEnv });
  if (pretestCode !== 0) {
    process.exit(pretestCode);
  }

  exitCode = await run(npxCmd, ['playwright', 'test', ...playwrightArgs], { env: childEnv });
} finally {
  const posttestCode = await run(nodeCmd, ['./scripts/posttest.mjs'], { env: childEnv });
  if (exitCode === 0 && posttestCode !== 0) {
    exitCode = posttestCode;
  }
  await clearManagedVerifyLock(childEnv);
}

process.exit(exitCode);
