import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { clearRuntimeState, readRuntimeState, writeRuntimeState } from './runtime-state.mjs';

const truthy = (value) =>
  ['1', 'true', 'yes', 'on'].includes(String(value || '').trim().toLowerCase());

const trim = (value) => String(value || '').trim();

function repoRootFromEnv(env = process.env) {
  const scriptDir = path.dirname(fileURLToPath(import.meta.url));
  return trim(env.PULSE_E2E_REPO_ROOT) || path.resolve(scriptDir, '..', '..', '..');
}

function hotDevBgScriptPath(env = process.env) {
  return path.join(repoRootFromEnv(env), 'scripts', 'hot-dev-bg.sh');
}

function hotDevBrowserURL(env = process.env) {
  const host = trim(env.FRONTEND_DEV_HOST) || '127.0.0.1';
  const port = trim(env.FRONTEND_DEV_PORT) || '5173';
  return `http://${host}:${port}`;
}

function hotDevBackendURL(env = process.env) {
  const host = trim(env.PULSE_DEV_API_HOST) || '127.0.0.1';
  const port = trim(env.PULSE_DEV_API_PORT) || '7655';
  return `http://${host}:${port}`;
}

function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { ...options });
    let stdout = '';
    let stderr = '';

    child.stdout?.on('data', (chunk) => {
      stdout += chunk.toString();
    });
    child.stderr?.on('data', (chunk) => {
      stderr += chunk.toString();
    });
    child.on('error', reject);
    child.on('close', (code) => resolve({ code: code ?? 1, stdout, stderr }));
  });
}

async function runHotDevBg(args, env = process.env) {
  const mergedEnv = { ...process.env, ...env };
  return run(hotDevBgScriptPath(mergedEnv), args, {
    cwd: repoRootFromEnv(mergedEnv),
    env: mergedEnv,
  });
}

function statusReportsRunning(output) {
  return output.includes('[hot-dev-bg] Running');
}

async function ensureHealthyStatus(env = process.env) {
  const status = await runHotDevBg(['status'], env);
  if (status.code !== 0) {
    throw new Error(`hot-dev-bg status failed: ${status.stderr || status.stdout}`);
  }

  const output = `${status.stdout}${status.stderr}`;
  const requiredMarkers = [
    '[hot-dev-bg] Frontend shell HTTP: 200',
    '[hot-dev-bg] Frontend proxy /api/health: 200',
    '[hot-dev-bg] Backend /api/health: 200',
  ];
  for (const marker of requiredMarkers) {
    if (!output.includes(marker)) {
      throw new Error(`Managed dev runtime is not healthy enough for browser proof:\n${output}`);
    }
  }

  return output;
}

export async function startManagedDevRuntime({
  env = process.env,
  logger = console,
} = {}) {
  await clearRuntimeState(env);

  const statusBefore = await runHotDevBg(['status'], env);
  if (statusBefore.code !== 0) {
    throw new Error(`hot-dev-bg status failed before start: ${statusBefore.stderr || statusBefore.stdout}`);
  }
  const wasRunning = statusReportsRunning(`${statusBefore.stdout}${statusBefore.stderr}`);

  const startArgs = ['start'];
  if (truthy(env.PULSE_E2E_HOT_DEV_TAKEOVER)) {
    startArgs.push('--takeover');
  }
  const startResult = await runHotDevBg(startArgs, env);
  if (startResult.code !== 0) {
    throw new Error(`hot-dev-bg start failed: ${startResult.stderr || startResult.stdout}`);
  }

  await ensureHealthyStatus(env);

  const runtimeState = {
    managedDevRuntime: true,
    baseURL: hotDevBrowserURL(env),
    browserURL: hotDevBrowserURL(env),
    backendURL: hotDevBackendURL(env),
    startedByHarness: !wasRunning,
  };
  await writeRuntimeState(runtimeState, env);
  logger.log(
    `[integration] Managed dev runtime ready at ${runtimeState.browserURL} (backend ${runtimeState.backendURL})`,
  );
  return runtimeState;
}

export async function restartManagedDevRuntimeBackend({
  env = process.env,
} = {}) {
  const result = await runHotDevBg(['backend-restart'], env);
  if (result.code !== 0) {
    throw new Error(`hot-dev-bg backend-restart failed: ${result.stderr || result.stdout}`);
  }
  await ensureHealthyStatus(env);
}

export async function stopManagedDevRuntime({
  env = process.env,
  logger = console,
} = {}) {
  const runtimeState = await readRuntimeState(env);
  if (!runtimeState?.managedDevRuntime) {
    await clearRuntimeState(env);
    return false;
  }

  if (runtimeState.startedByHarness) {
    const result = await runHotDevBg(['stop'], env);
    if (result.code !== 0) {
      throw new Error(`hot-dev-bg stop failed: ${result.stderr || result.stdout}`);
    }
    logger.log('[integration] Stopped managed dev runtime started by harness');
  }

  await clearRuntimeState(env);
  return true;
}
