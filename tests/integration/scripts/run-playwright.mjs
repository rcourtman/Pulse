#!/usr/bin/env node

import { spawn } from 'node:child_process';
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

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
  });

let exitCode = 0;
const childEnv = buildRunScopedEnv();

try {
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
}

process.exit(exitCode);
