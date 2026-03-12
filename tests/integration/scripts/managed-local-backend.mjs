import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';
import net from 'node:net';

import { applyRequestedEntitlementProfile } from './entitlement-bootstrap.mjs';
import { getRepoRoot, writeRuntimeState, readRuntimeState, clearRuntimeState } from './runtime-state.mjs';

const DEFAULT_E2E_BOOTSTRAP_TOKEN = '0123456789abcdef0123456789abcdef0123456789abcdef';
const DEFAULT_E2E_USERNAME = 'admin';
const DEFAULT_E2E_PASSWORD = 'adminadminadmin';
const DEFAULT_E2E_PRIMARY_API_TOKEN = '1111111111111111111111111111111111111111111111111111111111111111';

const trim = (value) => String(value || '').trim();

export function buildManagedLocalBackendState(env = process.env) {
  const repoRoot = getRepoRoot();
  const port = trim(env.PULSE_E2E_LOCAL_BACKEND_PORT) || '8765';
  const metricsPort = trim(env.PULSE_E2E_LOCAL_BACKEND_METRICS_PORT) || '0';
  const host = trim(env.PULSE_E2E_LOCAL_BACKEND_HOST) || '127.0.0.1';
  const baseURL = trim(env.PULSE_BASE_URL) || `http://${host}:${port}`;
  const rootDir = trim(env.PULSE_E2E_LOCAL_BACKEND_ROOT) || path.join(repoRoot, 'tmp', 'integration-local-backend');

  return {
    managedLocalBackend: true,
    repoRoot,
    port,
    metricsPort,
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

function withManagedLocalBackendPort(state, port) {
  const nextPort = String(port);
  return {
    ...state,
    port: nextPort,
    baseURL: `http://${state.host}:${nextPort}`,
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
    PULSE_APP_ROOT: state.repoRoot,
    PULSE_PUBLIC_URL: state.baseURL,
    PULSE_DATA_DIR: state.dataDir,
    PULSE_AUDIT_DIR: state.dataDir,
    PULSE_METRICS_PORT: state.metricsPort,
    PULSE_DEV: 'true',
    PULSE_E2E_BILLING_STATE_PATH: state.billingStatePath,
    PULSE_E2E_BOOTSTRAP_TOKEN: trim(env.PULSE_E2E_BOOTSTRAP_TOKEN) || DEFAULT_E2E_BOOTSTRAP_TOKEN,
    ALLOWED_ORIGINS: trim(env.ALLOWED_ORIGINS) || allowedOrigins.join(','),
  };

  if (trim(env.ALLOW_ADMIN_BYPASS) !== '') {
    nextEnv.ALLOW_ADMIN_BYPASS = trim(env.ALLOW_ADMIN_BYPASS);
  }
  if (trim(env.PULSE_AUTH_USER) !== '') {
    nextEnv.PULSE_AUTH_USER = trim(env.PULSE_AUTH_USER);
  }
  if (trim(env.PULSE_AUTH_PASS) !== '') {
    nextEnv.PULSE_AUTH_PASS = trim(env.PULSE_AUTH_PASS);
  }

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

async function canListen(host, port) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.unref();
    server.on('error', () => resolve(false));
    server.listen({ host, port }, () => {
      server.close(() => resolve(true));
    });
  });
}

async function reserveAvailablePort(host, preferredPort) {
  if (preferredPort > 0 && await canListen(host, preferredPort)) {
    return preferredPort;
  }

  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on('error', reject);
    server.listen({ host, port: 0 }, () => {
      const address = server.address();
      if (!address || typeof address === 'string') {
        server.close(() => reject(new Error('Failed to determine managed local backend port')));
        return;
      }
      const { port } = address;
      server.close((closeError) => {
        if (closeError) {
          reject(closeError);
          return;
        }
        resolve(port);
      });
    });
  });
}

async function collectNewestGoMtime(entryPath) {
  let stats;
  try {
    stats = await fs.stat(entryPath);
  } catch {
    return 0;
  }

  if (stats.isFile()) {
    return entryPath.endsWith('.go') ? stats.mtimeMs : 0;
  }

  if (!stats.isDirectory()) {
    return 0;
  }

  let newest = 0;
  for (const child of await fs.readdir(entryPath, { withFileTypes: true })) {
    const childPath = path.join(entryPath, child.name);
    if (child.isDirectory()) {
      newest = Math.max(newest, await collectNewestGoMtime(childPath));
      continue;
    }
    if (child.isFile() && child.name.endsWith('.go')) {
      const childStats = await fs.stat(childPath);
      newest = Math.max(newest, childStats.mtimeMs);
    }
  }

  return newest;
}

export async function shouldBuildManagedLocalBackendBinary(state) {
  let binaryStats;
  try {
    binaryStats = await fs.stat(state.binaryPath);
  } catch {
    return true;
  }

  const sourceRoots = ['cmd', 'internal', 'pkg'].map((segment) => path.join(state.repoRoot, segment));
  let newestSourceMtime = 0;
  for (const sourceRoot of sourceRoots) {
    newestSourceMtime = Math.max(newestSourceMtime, await collectNewestGoMtime(sourceRoot));
  }

  for (const manifestName of ['go.mod', 'go.sum']) {
    try {
      const manifestStats = await fs.stat(path.join(state.repoRoot, manifestName));
      newestSourceMtime = Math.max(newestSourceMtime, manifestStats.mtimeMs);
    } catch {
      // ignore missing manifest files
    }
  }

  return newestSourceMtime > binaryStats.mtimeMs;
}

async function ensureBackendBinary(state, logger) {
  if (!(await shouldBuildManagedLocalBackendBinary(state))) {
    return;
  }

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

async function waitForHealth(healthURL, pid, timeoutMs = 120_000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (!(await pidExists(pid))) {
      throw new Error(`Managed local backend process ${pid} exited before becoming healthy`);
    }
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

async function readSecurityStatus(baseURL) {
  const response = await fetch(`${baseURL}/api/security/status`);
  if (!response.ok) {
    throw new Error(`Failed to read security status: HTTP ${response.status}`);
  }
  return response.json();
}

async function ensureManagedLocalBackendSecurity(state, env, logger) {
  const security = await readSecurityStatus(state.baseURL);
  if (security?.hasAuthentication && security?.apiTokenConfigured) {
    return security;
  }

  const payload = {
    username: trim(env.PULSE_E2E_USERNAME) || DEFAULT_E2E_USERNAME,
    password: trim(env.PULSE_E2E_PASSWORD) || DEFAULT_E2E_PASSWORD,
    apiToken: trim(env.PULSE_E2E_PRIMARY_API_TOKEN) || DEFAULT_E2E_PRIMARY_API_TOKEN,
  };

  const response = await fetch(`${state.baseURL}/api/security/quick-setup`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Setup-Token': env.PULSE_E2E_BOOTSTRAP_TOKEN,
    },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(`Managed local backend quick setup failed: HTTP ${response.status} ${await response.text()}`);
  }

  logger.log(`[integration] Applied quick security setup for managed local backend user ${payload.username}`);
  return readSecurityStatus(state.baseURL);
}

async function assertManagedLocalBackendStartupHealthy(state) {
  let logContents = '';
  try {
    logContents = await fs.readFile(state.logPath, 'utf8');
  } catch {
    return;
  }

  for (const marker of ['Failed to start HTTP server', 'Metrics server failed']) {
    if (!logContents.includes(marker)) {
      continue;
    }

    const excerpt = logContents
      .trim()
      .split('\n')
      .slice(-40)
      .join('\n');
    throw new Error(`Managed local backend startup failed (${marker}). Recent log output:\n${excerpt}`);
  }
}

export async function startManagedLocalBackend({
  env = process.env,
  logger = console,
} = {}) {
  let state = buildManagedLocalBackendState(env);
  if (trim(env.PULSE_E2E_LOCAL_BACKEND_PORT) === '' && trim(env.PULSE_BASE_URL) === '') {
    const port = await reserveAvailablePort(state.host, 0);
    state = withManagedLocalBackendPort(state, port);
  }
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
    cwd: state.rootDir,
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
  try {
    await waitForHealth(`${state.baseURL}/api/health`, child.pid);
    await assertManagedLocalBackendStartupHealthy(state);
    await ensureManagedLocalBackendSecurity(state, backendEnv, logger);
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
  } catch (error) {
    await stopManagedLocalBackend({
      logger,
      state: {
        managedLocalBackend: true,
        pid: child.pid,
        dataDir: state.dataDir,
      },
    });
    throw error;
  }
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
