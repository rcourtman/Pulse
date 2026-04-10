import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export type RuntimeState = {
  baseURL?: string;
  primaryAPIToken?: string;
};

const trim = (value: unknown): string => String(value ?? '').trim();
const firstNonEmpty = (...values: readonly unknown[]): string =>
  values.map(trim).find((value) => value !== '') || '';

const repoRootFromEnv = (env: NodeJS.ProcessEnv = process.env): string =>
  trim(env.PULSE_E2E_REPO_ROOT) || path.resolve(__dirname, '..', '..', '..');

const managedHotDevPidPath = (env: NodeJS.ProcessEnv = process.env): string =>
  path.join(repoRootFromEnv(env), 'tmp', 'hot-dev.bg.pid');

export const runtimeStatePath = (env: NodeJS.ProcessEnv = process.env): string => {
  const configuredPath = trim(env.PULSE_E2E_RUNTIME_STATE_PATH);
  if (configuredPath === '') {
    return path.resolve(repoRootFromEnv(env), 'tmp', 'e2e-runtime-state.json');
  }
  return path.isAbsolute(configuredPath)
    ? configuredPath
    : path.resolve(repoRootFromEnv(env), configuredPath);
};

export const readRuntimeState = (
  env: NodeJS.ProcessEnv = process.env,
): RuntimeState | null => {
  try {
    const raw = fs.readFileSync(runtimeStatePath(env), 'utf8');
    return JSON.parse(raw) as RuntimeState;
  } catch {
    return null;
  }
};

export const loadRuntimeBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
): string | null => {
  const parsed = readRuntimeState(env);
  return typeof parsed?.baseURL === 'string' && parsed.baseURL.trim() !== ''
    ? parsed.baseURL.trim()
    : null;
};

export const managedDevBrowserBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
): string | null => {
  try {
    const pid = Number.parseInt(fs.readFileSync(managedHotDevPidPath(env), 'utf8').trim(), 10);
    if (!Number.isInteger(pid) || pid <= 0) {
      return null;
    }
    process.kill(pid, 0);
    const host = trim(env.FRONTEND_DEV_HOST) || '127.0.0.1';
    const port = trim(env.FRONTEND_DEV_PORT) || '5173';
    return `http://${host}:${port}`;
  } catch {
    return null;
  }
};

export const preferredBrowserBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
  overrides: readonly unknown[] = [],
): string =>
  firstNonEmpty(
    ...overrides,
    env.PLAYWRIGHT_BASE_URL,
    env.PULSE_BASE_URL,
    loadRuntimeBaseURL(env),
    managedDevBrowserBaseURL(env),
    'http://localhost:7655',
  );

export const preferredPlaywrightRouteBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
  overrides: readonly unknown[] = [],
): string => preferredBrowserBaseURL(env, overrides).replace(/\/+$/, '');
