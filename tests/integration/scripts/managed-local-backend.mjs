import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';
import net from 'node:net';

import { withExclusiveLock } from '../../../scripts/exclusive-lock.mjs';
import { applyRequestedEntitlementProfile } from './entitlement-bootstrap.mjs';
import { getRepoRoot, writeRuntimeState, readRuntimeState, clearRuntimeState } from './runtime-state.mjs';

const DEFAULT_E2E_BOOTSTRAP_TOKEN = '0123456789abcdef0123456789abcdef0123456789abcdef';
const DEFAULT_E2E_USERNAME = 'admin';
const DEFAULT_E2E_PASSWORD = 'adminadminadmin';
const DEFAULT_E2E_PRIMARY_API_TOKEN = '1111111111111111111111111111111111111111111111111111111111111111';
const DEFAULT_LOCAL_BACKEND_VARIANT = 'core';
const SUPPORTED_LOCAL_BACKEND_VARIANTS = new Set(['core', 'enterprise']);

const trim = (value) => String(value || '').trim();
const buildScopedPathSegment = (value) => String(value).replace(/[^A-Za-z0-9._-]+/g, '-');

function resolveManagedLocalBackendVariant(env = process.env) {
  const requestedVariant = trim(env.PULSE_E2E_LOCAL_BACKEND_VARIANT).toLowerCase();
  if (requestedVariant === '') {
    return DEFAULT_LOCAL_BACKEND_VARIANT;
  }
  if (!SUPPORTED_LOCAL_BACKEND_VARIANTS.has(requestedVariant)) {
    throw new Error(
      `Unsupported managed local backend variant ${JSON.stringify(requestedVariant)}. Supported variants: ${Array.from(SUPPORTED_LOCAL_BACKEND_VARIANTS).join(', ')}`,
    );
  }
  return requestedVariant;
}

function buildManagedLocalBackendBinaryConfig(env, repoRoot) {
  const backendVariant = resolveManagedLocalBackendVariant(env);
  if (backendVariant === 'enterprise') {
    const enterpriseRepoRoot = trim(env.PULSE_E2E_LOCAL_BACKEND_ENTERPRISE_REPO)
      || path.resolve(repoRoot, '..', 'pulse-enterprise');
    return {
      backendVariant,
      binaryPath: trim(env.PULSE_E2E_LOCAL_BACKEND_BINARY)
        || path.join(repoRoot, 'tmp', 'integration-local-backend', 'bin', 'pulse-enterprise'),
      binaryBuildLockPath: path.join(repoRoot, 'tmp', 'locks', 'managed-local-backend-binary-enterprise.lock'),
      binaryBuildCwd: enterpriseRepoRoot,
      binaryBuildArgs: ['build', '-buildvcs=false', '-o', '__OUTPUT__', './cmd/pulse-enterprise'],
      binarySourceRoots: [
        path.join(enterpriseRepoRoot, 'cmd'),
        path.join(enterpriseRepoRoot, 'internal'),
        path.join(enterpriseRepoRoot, 'test'),
        path.join(repoRoot, 'cmd'),
        path.join(repoRoot, 'internal'),
        path.join(repoRoot, 'pkg'),
      ],
      binaryManifestPaths: [
        path.join(enterpriseRepoRoot, 'go.mod'),
        path.join(enterpriseRepoRoot, 'go.sum'),
        path.join(repoRoot, 'go.mod'),
        path.join(repoRoot, 'go.sum'),
      ],
    };
  }

  return {
    backendVariant,
    binaryPath: trim(env.PULSE_E2E_LOCAL_BACKEND_BINARY) || path.join(repoRoot, 'pulse'),
    binaryBuildLockPath: path.join(repoRoot, 'tmp', 'locks', 'managed-local-backend-binary.lock'),
    binaryBuildCwd: repoRoot,
    binaryBuildArgs: ['build', '-o', '__OUTPUT__', './cmd/pulse'],
    binarySourceRoots: [
      path.join(repoRoot, 'cmd'),
      path.join(repoRoot, 'internal'),
      path.join(repoRoot, 'pkg'),
    ],
    binaryManifestPaths: [
      path.join(repoRoot, 'go.mod'),
      path.join(repoRoot, 'go.sum'),
    ],
  };
}

function resolveManagedLocalBackendRunId(env) {
  const explicitRunId = trim(env.PULSE_E2E_RUN_ID);
  if (explicitRunId !== '') {
    return explicitRunId;
  }

  const runtimeStatePath = trim(env.PULSE_E2E_RUNTIME_STATE_PATH);
  if (runtimeStatePath === '') {
    return '';
  }

  return path.basename(runtimeStatePath).replace(/\.[^.]+$/, '');
}

export function buildManagedLocalBackendState(env = process.env) {
  const repoRoot = getRepoRoot();
  const binaryConfig = buildManagedLocalBackendBinaryConfig(env, repoRoot);
  const port = trim(env.PULSE_E2E_LOCAL_BACKEND_PORT) || '8765';
  const metricsPort = trim(env.PULSE_E2E_LOCAL_BACKEND_METRICS_PORT) || '0';
  const host = trim(env.PULSE_E2E_LOCAL_BACKEND_HOST) || '127.0.0.1';
  const baseURL = trim(env.PULSE_BASE_URL) || `http://${host}:${port}`;
  const runId = resolveManagedLocalBackendRunId(env);
  const rootDir = trim(env.PULSE_E2E_LOCAL_BACKEND_ROOT) || (
    runId === ''
      ? path.join(repoRoot, 'tmp', 'integration-local-backend')
      : path.join(repoRoot, 'tmp', 'integration-local-backend', buildScopedPathSegment(runId))
  );

  return {
    managedLocalBackend: true,
    backendVariant: binaryConfig.backendVariant,
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
    binaryPath: binaryConfig.binaryPath,
    binaryBuildLockPath: binaryConfig.binaryBuildLockPath,
    binaryBuildCwd: binaryConfig.binaryBuildCwd,
    binaryBuildArgs: binaryConfig.binaryBuildArgs,
    binarySourceRoots: binaryConfig.binarySourceRoots,
    binaryManifestPaths: binaryConfig.binaryManifestPaths,
    frontendRoot: path.join(repoRoot, 'frontend-modern'),
    embeddedFrontendDistPath: path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist'),
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
    PULSE_E2E_PRIMARY_API_TOKEN: trim(env.PULSE_E2E_PRIMARY_API_TOKEN) || DEFAULT_E2E_PRIMARY_API_TOKEN,
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

  if (trim(env.PULSE_HOSTED_MODE).toLowerCase() === 'true') {
    nextEnv.PULSE_AUTH_USER = trim(env.PULSE_AUTH_USER) || trim(env.PULSE_E2E_USERNAME) || DEFAULT_E2E_USERNAME;
    nextEnv.PULSE_AUTH_PASS = trim(env.PULSE_AUTH_PASS) || trim(env.PULSE_E2E_PASSWORD) || DEFAULT_E2E_PASSWORD;
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

async function collectNewestMatchingMtime(entryPath, predicate) {
  let stats;
  try {
    stats = await fs.stat(entryPath);
  } catch {
    return 0;
  }

  if (stats.isFile()) {
    return predicate(entryPath) ? stats.mtimeMs : 0;
  }

  if (!stats.isDirectory()) {
    return 0;
  }

  let newest = 0;
  for (const child of await fs.readdir(entryPath, { withFileTypes: true })) {
    const childPath = path.join(entryPath, child.name);
    if (child.isDirectory()) {
      newest = Math.max(newest, await collectNewestMatchingMtime(childPath, predicate));
      continue;
    }
    if (child.isFile() && predicate(childPath)) {
      const childStats = await fs.stat(childPath);
      newest = Math.max(newest, childStats.mtimeMs);
    }
  }

  return newest;
}

async function collectNewestTreeMtime(entryPath) {
  return collectNewestMatchingMtime(entryPath, () => true);
}

export async function shouldBuildManagedLocalBackendFrontend(state) {
  let embeddedDistStats;
  try {
    embeddedDistStats = await fs.stat(state.embeddedFrontendDistPath);
  } catch {
    return true;
  }
  if (!embeddedDistStats.isDirectory()) {
    return true;
  }

  const frontendSourceRoots = ['src', 'scripts'].map((segment) => path.join(state.frontendRoot, segment));
  let newestSourceMtime = 0;
  for (const sourceRoot of frontendSourceRoots) {
    newestSourceMtime = Math.max(newestSourceMtime, await collectNewestTreeMtime(sourceRoot));
  }

  for (const manifestName of [
    'package.json',
    'package-lock.json',
    'vite.config.ts',
    'tsconfig.json',
    'tailwind.config.js',
    'postcss.config.js',
    'index.html',
  ]) {
    try {
      const manifestStats = await fs.stat(path.join(state.frontendRoot, manifestName));
      newestSourceMtime = Math.max(newestSourceMtime, manifestStats.mtimeMs);
    } catch {
      // ignore missing manifest files
    }
  }

  const newestEmbeddedMtime = await collectNewestTreeMtime(state.embeddedFrontendDistPath);
  return newestSourceMtime > newestEmbeddedMtime;
}

export async function shouldBuildManagedLocalBackendBinary(state) {
  let binaryStats;
  try {
    binaryStats = await fs.stat(state.binaryPath);
  } catch {
    return true;
  }

  const sourceRoots = Array.isArray(state.binarySourceRoots) && state.binarySourceRoots.length > 0
    ? state.binarySourceRoots
    : ['cmd', 'internal', 'pkg'].map((segment) => path.join(state.repoRoot, segment));
  let newestSourceMtime = 0;
  for (const sourceRoot of sourceRoots) {
    newestSourceMtime = Math.max(newestSourceMtime, await collectNewestGoMtime(sourceRoot));
  }

  newestSourceMtime = Math.max(
    newestSourceMtime,
    await collectNewestTreeMtime(state.embeddedFrontendDistPath),
  );

  const manifestPaths = Array.isArray(state.binaryManifestPaths) && state.binaryManifestPaths.length > 0
    ? state.binaryManifestPaths
    : ['go.mod', 'go.sum'].map((manifestName) => path.join(state.repoRoot, manifestName));

  for (const manifestPath of manifestPaths) {
    try {
      const manifestStats = await fs.stat(manifestPath);
      newestSourceMtime = Math.max(newestSourceMtime, manifestStats.mtimeMs);
    } catch {
      // ignore missing manifest files
    }
  }

  return newestSourceMtime > binaryStats.mtimeMs;
}

async function ensureFrontendAssets(state, logger) {
  if (!(await shouldBuildManagedLocalBackendFrontend(state))) {
    return;
  }

  logger.log('[integration] Building embedded frontend assets');
  await new Promise((resolve, reject) => {
    const child = spawn('npm', ['run', 'build'], {
      cwd: state.frontendRoot,
      stdio: 'inherit',
    });
    child.on('error', reject);
    child.on('close', (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`npm run build exited with code ${code}`));
    });
  });
}

async function ensureBackendBinary(state, logger) {
  await withExclusiveLock(
    state.binaryBuildLockPath,
    async () => {
      if (!(await shouldBuildManagedLocalBackendBinary(state))) {
        return;
      }

      logger.log(`[integration] Building ${state.backendVariant || 'core'} local backend binary at ${state.binaryPath}`);
      const temporaryBinaryPath = `${state.binaryPath}.${process.pid}.tmp`;
      const binaryBuildCwd = state.binaryBuildCwd || state.repoRoot;
      const binaryBuildArgs = Array.isArray(state.binaryBuildArgs) && state.binaryBuildArgs.length > 0
        ? state.binaryBuildArgs.map((arg) => (arg === '__OUTPUT__' ? temporaryBinaryPath : arg))
        : ['build', '-o', temporaryBinaryPath, './cmd/pulse'];
      await fs.mkdir(path.dirname(temporaryBinaryPath), { recursive: true });
      await fs.rm(temporaryBinaryPath, { force: true });
      try {
        await new Promise((resolve, reject) => {
          const child = spawn('go', binaryBuildArgs, {
            cwd: binaryBuildCwd,
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
        await fs.rename(temporaryBinaryPath, state.binaryPath);
      } finally {
        await fs.rm(temporaryBinaryPath, { force: true });
      }
    },
    { description: 'managed local backend binary build' },
  );
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

async function createManagedLocalBackendAPIToken(state, env, logger) {
  const username = trim(env.PULSE_AUTH_USER) || trim(env.PULSE_E2E_USERNAME) || DEFAULT_E2E_USERNAME;
  const password = trim(env.PULSE_AUTH_PASS) || trim(env.PULSE_E2E_PASSWORD) || DEFAULT_E2E_PASSWORD;
  const response = await fetch(`${state.baseURL}/api/security/tokens`, {
    method: 'POST',
    headers: {
      Authorization: `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      name: 'Managed local backend primary token',
      scopes: ['*'],
    }),
  });
  if (!response.ok) {
    throw new Error(`Managed local backend token creation failed: HTTP ${response.status} ${await response.text()}`);
  }

  const payload = await response.json();
  const rawToken = trim(payload?.token);
  if (rawToken === '') {
    throw new Error('Managed local backend token creation response did not include a token');
  }

  logger.log(`[integration] Created managed local backend API token for ${username}`);
  return rawToken;
}

async function ensureManagedLocalBackendSecurity(state, env, logger) {
  const security = await readSecurityStatus(state.baseURL);
  if (security?.hasAuthentication && security?.apiTokenConfigured) {
    return {
      security,
      primaryAPIToken: trim(env.PULSE_E2E_PRIMARY_API_TOKEN) || DEFAULT_E2E_PRIMARY_API_TOKEN,
    };
  }

  if (security?.hasAuthentication && !security?.apiTokenConfigured) {
    const primaryAPIToken = await createManagedLocalBackendAPIToken(state, env, logger);
    return {
      security: await readSecurityStatus(state.baseURL),
      primaryAPIToken,
    };
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
  return {
    security: await readSecurityStatus(state.baseURL),
    primaryAPIToken: payload.apiToken,
  };
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
  await ensureFrontendAssets(state, logger);
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
    const securityState = await ensureManagedLocalBackendSecurity(state, backendEnv, logger);
    await applyRequestedEntitlementProfile({ env: backendEnv, logger });

    const runtimeState = {
      managedLocalBackend: true,
      backendVariant: state.backendVariant,
      baseURL: state.baseURL,
      pid: child.pid,
      dataDir: state.dataDir,
      logPath: state.logPath,
      billingStatePath: state.billingStatePath,
      primaryAPIToken: securityState.primaryAPIToken,
    };
    await writeRuntimeState(runtimeState, env);
    logger.log(`[integration] Started managed local backend at ${state.baseURL} (pid ${child.pid})`);
    return runtimeState;
  } catch (error) {
    await stopManagedLocalBackend({
      env,
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
  env = process.env,
  logger = console,
  state = null,
} = {}) {
  const runtimeState = state || await readRuntimeState(env);
  if (!runtimeState || !runtimeState.managedLocalBackend) {
    await clearRuntimeState(env);
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
  await clearRuntimeState(env);
  logger.log('[integration] Stopped managed local backend');
  return true;
}
