import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptsDir, '..', '..', '..');
const runtimeStatePath = path.join(repoRoot, 'tmp', 'e2e-runtime-state.json');

export function getRepoRoot() {
  return repoRoot;
}

export function getRuntimeStatePath() {
  return runtimeStatePath;
}

export async function writeRuntimeState(state) {
  await fs.mkdir(path.dirname(runtimeStatePath), { recursive: true });
  await fs.writeFile(runtimeStatePath, `${JSON.stringify(state, null, 2)}\n`, 'utf8');
}

export async function readRuntimeState() {
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

export async function clearRuntimeState() {
  try {
    await fs.rm(runtimeStatePath, { force: true });
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error && error.code === 'ENOENT') {
      return;
    }
    throw error;
  }
}
