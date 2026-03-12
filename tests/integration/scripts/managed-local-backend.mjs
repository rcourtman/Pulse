import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';

import { applyRequestedEntitlementProfile } from './entitlement-bootstrap.mjs';
import { getRepoRoot, writeRuntimeState, readRuntimeState, clearRuntimeState } from './runtime-state.mjs';

const DEFAULT_E2E_BOOTSTRAP_TOKEN = '0123456789abcdef0123456789abcdef0123456789abcdef';
const DEFAULT_AUTH_HASH = '$2a$12$j9/pl2RCHGVGvtv4wocrx.FGBczUw97ZAeO8im0.Ty.fXDGFOviWS';

const trim = (value) => String(value || '').trim();

export function buildManagedLocalBackendState(env = process.env) {
  const repoRoot = getRepoRoot();
  const port = trim(env.PULSE_E2E_LOCAL_BACKEND_PORT) || '8765';
  const host = trim(env.PULSE_E2E_LOCAL_BACKEND_HOST) || '127.0.0.1';
  const baseURL = trim(env.PULSE_BASE_URL) || `http://${host}:${port}`;
  const rootDir = trim(env.PULSE_E2E_LOCAL_BACKEND_ROOT) || path.join(repoRoot, 'tmp', 'integration-local-backend');

  return {
    managedLocalBackend: true,
    repoRoot,
    port,
    host,
    baseURL: baseURL.replace(/\/+$/, ''),
    rootDir,
    dataDir: path.join(rootDir, 'data'),
    logPath: path.join(rootDir, 'pulse.log'),
    pidPath: path.join(rootDir, 'pulse.pid'),
    billingStatePath: path.join(rootDir, 'data', 'billing.json'),
    binaryPath: trim(env.PULSE_E2E_LOCAL_BACKEND_BINARY) || path.join(repoRoot, 'pulse'),
  };
}

export function buildManagedLocalBackendEnv(state, env = process.env) {
  const allowedOrigins = [
    `http://${state.host}:5173`,
    'http://localhost:5173',
    `http://${state.host}:${state.port}`,
    `http://localhost:${state.port}`,
  ];

  const nextEnv = {
    ...env,
    PORT: state.port,
    FRONTEND_PORT: state.port,
    PULSE_PUBLIC_URL: state.baseURL,
    PULSE_DATA_DIR: state.dataDir,
    PULSE_AUDIT_DIR: state.dataDir,
    PULSE_DEV: 'true',
    ALLOW_ADMIN_BYPASS: '1',
    PULSE_AUTH_USER: trim(env.PULSE_AUTH_USER) || 'admin',
    PULSE_AUTH_PASS: trim(env.PULSE_AUTH_PASS) || DEFAULT_AUTH_HASH,
    PULSE_E2E_BILLING_STATE_PATH: state.billingStatePath,
    PULSE_E2E_BOOTSTRAP_TOKEN: trim(env.PULSE_E2E_BOOTSTRAP_TOKEN) || DEFAULT_E2E_BOOTSTRAP_TOKEN,
    ALLOWED_ORIGINS: trim(env.ALLOWED_ORIGINS) || allowedOrigins.join(','),
  };

  if (trim(env.PULSE_E2E_ENTITLEMENT_PROFILE) !== '') {
    nextEnv.PULSE_E2E_ENTITLEMENT_PROFILE = trim(env.PULSE_E2E_ENTITLEMENT_PROFILE);
  }

  return nextEnv;
}

async function pidExists(pid) {
  if (!Number.isInteger(pid) || pid <= 0) {
    return false;
  }
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

async function ensureBackendBinary(state, logger) {
  try {
    await fs.access(state.binaryPath);
  } catch {
    logger.log(`[integration] Building local backend binary at ${state.binaryPath}`);
    await new Promise((resolve, reject) => {
      const child = spawn('go', ['build', '-o', state.binaryPath, './cmd/pulse'], {
        cwd: state.repoRoot,
        stdio: 'inherit',
      });
      child.on('error', reject);
      child.on('close', (code) => {
        if (code === 0) {
          resolve();
          return;
        }
        reject(new Error(`go build exited with code ${code}`));
      });
    });
  }
}

async function waitForHealth(healthURL, timeoutMs = 120_000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(healthURL);
      if (response.ok) {
        return;
      }
    } catch {
      // retry
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error(`Timed out waiting for ${healthURL}`);
}

export async function startManagedLocalBackend({
  env = process.env,
  logger = console,
} = {}) {
  const state = buildManagedLocalBackendState(env);
  const backendEnv = buildManagedLocalBackendEnv(state, env);

  await fs.rm(state.rootDir, { recursive: true, force: true });
  await fs.mkdir(state.dataDir, { recursive: true });
  await fs.writeFile(
    path.join(state.dataDir, '.bootstrap_token'),
    `${backendEnv.PULSE_E2E_BOOTSTRAP_TOKEN}\n`,
    { mode: 0o600 },
  );
  await ensureBackendBinary(state, logger);

  const logHandle = await fs.open(state.logPath, 'w');
  const child = spawn(state.binaryPath, [], {
    cwd: state.repoRoot,
    env: backendEnv,
    detached: true,
    stdio: ['ignore', logHandle.fd, logHandle.fd],
  });
  child.unref();
  await logHandle.close();

  if (!Number.isInteger(child.pid) || child.pid <= 0) {
    throw new Error('Managed local backend failed to start');
  }

  await fs.writeFile(state.pidPath, `${child.pid}\n`, 'utf8');
  await waitForHealth(`${state.baseURL}/api/health`);
  await applyRequestedEntitlementProfile({ env: backendEnv, logger });

  const runtimeState = {
    managedLocalBackend: true,
    baseURL: state.baseURL,
    pid: child.pid,
    dataDir: state.dataDir,
    logPath: state.logPath,
    billingStatePath: state.billingStatePath,
  };
  await writeRuntimeState(runtimeState);
  logger.log(`[integration] Started managed local backend at ${state.baseURL} (pid ${child.pid})`);
  return runtimeState;
}

export async function stopManagedLocalBackend({
  logger = console,
  state = null,
} = {}) {
  const runtimeState = state || await readRuntimeState();
  if (!runtimeState || !runtimeState.managedLocalBackend) {
    await clearRuntimeState();
    return false;
  }

  if (await pidExists(runtimeState.pid)) {
    try {
      process.kill(runtimeState.pid, 'SIGTERM');
    } catch {
      // process exited between checks
    }
    const deadline = Date.now() + 10_000;
    while (Date.now() < deadline) {
      if (!(await pidExists(runtimeState.pid))) {
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    if (await pidExists(runtimeState.pid)) {
      process.kill(runtimeState.pid, 'SIGKILL');
    }
  }

  if (trim(runtimeState.dataDir) !== '') {
    await fs.rm(path.dirname(runtimeState.dataDir), { recursive: true, force: true });
  }
  await clearRuntimeState();
  logger.log('[integration] Stopped managed local backend');
  return true;
}
