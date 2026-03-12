import { cp, mkdir, rename, rm, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { withExclusiveLock } from '../../scripts/exclusive-lock.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..');
const sourceDir = path.join(repoRoot, 'frontend-modern', 'dist');
const targetDir = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist');
const lockPath = path.join(repoRoot, 'tmp', 'locks', 'frontend-embed-sync.lock');

async function ensureSourceExists() {
  try {
    const sourceStat = await stat(sourceDir);
    if (!sourceStat.isDirectory()) {
      throw new Error(`${sourceDir} is not a directory`);
    }
  } catch (error) {
    throw new Error(`frontend build output missing at ${sourceDir}`, { cause: error });
  }
}

async function publishDirAtomically() {
  const targetParent = path.dirname(targetDir);
  const targetName = path.basename(targetDir);
  const nonce = `${Date.now()}-${process.pid}-${Math.random().toString(36).slice(2, 8)}`;
  const stagingDir = path.join(targetParent, `${targetName}.${nonce}.staging`);
  const backupDir = path.join(targetParent, `${targetName}.${nonce}.backup`);

  await rm(stagingDir, { force: true, recursive: true });
  await rm(backupDir, { force: true, recursive: true });
  await mkdir(targetParent, { recursive: true });
  await cp(sourceDir, stagingDir, { recursive: true });

  let replacedExistingTarget = false;
  try {
    try {
      await rename(targetDir, backupDir);
      replacedExistingTarget = true;
    } catch (error) {
      if (!error || typeof error !== 'object' || !('code' in error) || error.code !== 'ENOENT') {
        throw error;
      }
    }

    await rename(stagingDir, targetDir);
  } catch (error) {
    await rm(stagingDir, { force: true, recursive: true });
    if (replacedExistingTarget) {
      try {
        await rename(backupDir, targetDir);
      } catch {
        // Leave the original failure intact; cleanup best effort only.
      }
    }
    throw error;
  }

  if (replacedExistingTarget) {
    await rm(backupDir, { force: true, recursive: true });
  }
}

export async function syncEmbedDir({ lock = true } = {}) {
  await ensureSourceExists();
  const action = async () => {
    await publishDirAtomically();
  };

  if (lock) {
    await withExclusiveLock(lockPath, action, {
      description: 'frontend embedded asset sync',
    });
  } else {
    await action();
  }

  console.log(`Synced frontend embed assets to ${targetDir}`);
}

const isMainModule = process.argv[1]
  ? pathToFileURL(process.argv[1]).href === import.meta.url
  : false;

if (isMainModule) {
  syncEmbedDir().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  });
}
