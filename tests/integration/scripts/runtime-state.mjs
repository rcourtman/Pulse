import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptsDir, '..', '..', '..');
const defaultRuntimeStatePath = path.join(repoRoot, 'tmp', 'e2e-runtime-state.json');

function trim(value) {
  return String(value || '').trim();
}

export function getRepoRoot() {
  return repoRoot;
}

export function getRuntimeStatePath(env = process.env) {
  const configuredPath = trim(env.PULSE_E2E_RUNTIME_STATE_PATH);
  if (configuredPath === '') {
    return defaultRuntimeStatePath;
  }
  if (path.isAbsolute(configuredPath)) {
    return configuredPath;
  }
  return path.resolve(repoRoot, configuredPath);
}

export async function writeRuntimeState(state, env = process.env) {
  const runtimeStatePath = getRuntimeStatePath(env);
  await fs.mkdir(path.dirname(runtimeStatePath), { recursive: true });
  await fs.writeFile(runtimeStatePath, `${JSON.stringify(state, null, 2)}\n`, 'utf8');
}

export async function readRuntimeState(env = process.env) {
  const runtimeStatePath = getRuntimeStatePath(env);
  try {
    const raw = await fs.readFile(runtimeStatePath, 'utf8');
    return JSON.parse(raw);
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error && error.code === 'ENOENT') {
      return null;
    }
    throw error;
  }
}

export async function clearRuntimeState(env = process.env) {
  const runtimeStatePath = getRuntimeStatePath(env);
  try {
    await fs.rm(runtimeStatePath, { force: true });
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error && error.code === 'ENOENT') {
      return;
    }
    throw error;
  }
}
