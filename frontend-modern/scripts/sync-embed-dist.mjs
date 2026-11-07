import { cp, lstat, mkdir, readFile, rename, rm, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { withExclusiveLock } from '../../scripts/exclusive-lock.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..');
const sourceDir = path.join(repoRoot, 'frontend-modern', 'dist');
const targetDir = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist');
// Serialize standalone copies with the producer, not just with other copies.
// The builder skips this inner lock only while holding the same outer lock.
const lockPath = path.join(repoRoot, 'tmp', 'locks', 'frontend-embed-build.lock');

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

export async function validateBuildOutput(stagingDir) {
  const incomplete = (requirement, cause) =>
    new Error(
      `frontend build output incomplete: ${requirement}. Rebuild the frontend before syncing; existing embedded assets were not replaced.`,
      { cause },
    );

  let index;
  try {
    const directory = await lstat(stagingDir);
    if (!directory.isDirectory()) throw new Error('irregular stage');
    const entry = await lstat(path.join(stagingDir, 'index.html'));
    if (!entry.isFile() || entry.size === 0) {
      throw new Error('missing or irregular entry');
    }
    index = await readFile(path.join(stagingDir, 'index.html'), 'utf8');
  } catch (error) {
    throw incomplete('index.html must be a nonempty regular file', error);
  }

  // Vite emits quoted /assets/ references for the entry, CSS, modulepreloads
  // and import-map integrity keys. A surviving entry alone is not a usable
  // build: go:embed can compile it even when its advertised chunks are gone.
  // This is structural admission, not a replacement for bundle/SRI checks.
  const assets = new Set(
    [...index.matchAll(/["'](\/assets\/[^"']+)["']/g)].map((match) => match[1].slice(1)),
  );
  if (![...assets].some((asset) => asset.endsWith('.js'))) {
    throw incomplete('index.html must reference a JavaScript asset under /assets/');
  }
  for (const asset of assets) {
    const parts = asset.split('/');
    if (
      parts.some(
        (part) =>
          !part || part === '.' || part === '..' || part.includes('\\') || part.includes('\0'),
      )
    ) {
      throw incomplete('index.html contains an invalid /assets/ reference');
    }
    try {
      let parent = stagingDir;
      for (const part of parts.slice(0, -1)) {
        parent = path.join(parent, part);
        if (!(await lstat(parent)).isDirectory()) throw new Error('irregular asset directory');
      }
      const entry = await lstat(path.join(parent, parts.at(-1)));
      if (!entry.isFile() || entry.size === 0) throw new Error('missing or irregular asset');
    } catch (error) {
      throw incomplete(`referenced asset ${asset} must be a nonempty regular file`, error);
    }
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

  let replacedExistingTarget = false;
  try {
    await cp(sourceDir, stagingDir, { recursive: true });
    // Validate the owned copy, not the mutable dist directory checked before
    // waiting for the lock. Do not move the usable target until it passes.
    await validateBuildOutput(stagingDir);
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
    const failures = [error];
    const message = (failure) =>
      failure instanceof Error ? failure.message : String(failure);
    const details = [message(error)];
    // Restore usable output before fallible scratch cleanup. A cleanup error
    // must not strand the only good build under its backup name.
    if (replacedExistingTarget) {
      try {
        await rename(backupDir, targetDir);
      } catch (restoreError) {
        failures.push(restoreError);
        details.push(
          `previous embedded build could not be restored; backup at ${backupDir}: ${message(restoreError)}`,
        );
      }
    }
    try {
      await rm(stagingDir, { force: true, recursive: true });
    } catch (cleanupError) {
      failures.push(cleanupError);
      details.push(
        `staging cleanup failed at ${stagingDir}: ${message(cleanupError)}`,
      );
    }
    if (failures.length > 1) {
      throw new AggregateError(failures, `Frontend embed publication failed: ${details.join('; ')}`);
    }
    throw error;
  }

  if (replacedExistingTarget) {
    try {
      await rm(backupDir, { force: true, recursive: true });
    } catch (error) {
      throw new Error(
        `Frontend embed published at ${targetDir}, but backup cleanup failed at ${backupDir}: ${error instanceof Error ? error.message : String(error)}`,
        { cause: error },
      );
    }
  }
}

export async function syncEmbedDir({ lock = true } = {}) {
  const action = async () => {
    // A build holding the lock may still be creating (or emptying) dist.
    await ensureSourceExists();
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
