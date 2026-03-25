import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const managedHotDevPidPath = path.join(repoRoot, 'tmp', 'hot-dev.bg.pid');

export type RuntimeState = {
  baseURL?: string;
  primaryAPIToken?: string;
};

const trim = (value: unknown): string => String(value ?? '').trim();

export const runtimeStatePath = (env: NodeJS.ProcessEnv = process.env): string => {
  const configuredPath = trim(env.PULSE_E2E_RUNTIME_STATE_PATH);
  if (configuredPath === '') {
    return path.resolve(repoRoot, 'tmp', 'e2e-runtime-state.json');
  }
  return path.isAbsolute(configuredPath) ? configuredPath : path.resolve(repoRoot, configuredPath);
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
    const pid = Number.parseInt(fs.readFileSync(managedHotDevPidPath, 'utf8').trim(), 10);
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
): string =>
  trim(env.PULSE_BASE_URL) ||
  trim(env.PLAYWRIGHT_BASE_URL) ||
  loadRuntimeBaseURL(env) ||
  managedDevBrowserBaseURL(env) ||
  'http://localhost:7655';
