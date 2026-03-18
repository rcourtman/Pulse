import fs from 'node:fs/promises';
import path from 'node:path';

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

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

async function readLockOwner(lockPath) {
  try {
    const raw = await fs.readFile(path.join(lockPath, 'owner.json'), 'utf8');
    return JSON.parse(raw);
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error && error.code === 'ENOENT') {
      return null;
    }
    throw error;
  }
}

async function removeStaleLock(lockPath, staleAfterMs) {
  let stats;
  try {
    stats = await fs.stat(lockPath);
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error && error.code === 'ENOENT') {
      return false;
    }
    throw error;
  }

  const owner = await readLockOwner(lockPath);
  if (owner && Number.isInteger(owner.pid) && await pidExists(owner.pid)) {
    return false;
  }

  const ageMs = Date.now() - stats.mtimeMs;
  if (owner || ageMs >= staleAfterMs) {
    await fs.rm(lockPath, { recursive: true, force: true });
    return true;
  }

  return false;
}

export async function withExclusiveLock(lockPath, action, {
  description = 'exclusive',
  pollIntervalMs = 100,
  staleAfterMs = 60_000,
  timeoutMs = 300_000,
} = {}) {
  await fs.mkdir(path.dirname(lockPath), { recursive: true });

  const deadline = Date.now() + timeoutMs;
  const owner = {
    pid: process.pid,
    createdAt: new Date().toISOString(),
    cwd: process.cwd(),
  };

  while (true) {
    try {
      await fs.mkdir(lockPath);
      await fs.writeFile(
        path.join(lockPath, 'owner.json'),
        `${JSON.stringify(owner, null, 2)}\n`,
        'utf8',
      );
      break;
    } catch (error) {
      if (!error || typeof error !== 'object' || !('code' in error) || error.code !== 'EEXIST') {
        throw error;
      }

      if (await removeStaleLock(lockPath, staleAfterMs)) {
        continue;
      }

      if (Date.now() >= deadline) {
        const currentOwner = await readLockOwner(lockPath);
        const ownerDetail =
          currentOwner && Number.isInteger(currentOwner.pid)
            ? ` owned by pid ${currentOwner.pid}`
            : '';
        throw new Error(
          `Timed out waiting for ${description} lock at ${lockPath}${ownerDetail}`,
        );
      }

      await sleep(pollIntervalMs);
    }
  }

  try {
    return await action();
  } finally {
    await fs.rm(lockPath, { recursive: true, force: true });
  }
}
