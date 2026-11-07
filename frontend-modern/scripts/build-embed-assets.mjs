import path from 'node:path';
import { spawn } from 'node:child_process';
import { mkdtemp, rename, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

import { withExclusiveLock } from '../../scripts/exclusive-lock.mjs';
import { syncEmbedDir, validateBuildOutput } from './sync-embed-dist.mjs';
import { syncPublicDocs } from './sync-public-docs.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const lockPath = path.join(repoRoot, 'tmp', 'locks', 'frontend-embed-build.lock');
// Invoke the installed CLI with this Node executable. Windows .cmd shims
// require a shell and are not portable child_process.spawn executables.
const viteCLI = path.join(frontendRoot, 'node_modules', 'vite', 'bin', 'vite.js');

function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code, signal) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(
        new Error(
          `${command} ${args.join(' ')} ${signal ? `terminated by signal ${signal}` : `exited with code ${code}`}`,
        ),
      );
    });
  });
}

async function buildDist() {
  // Vite empties its output before compiling. Keep that mutable output away
  // from the last completed dist, which standalone sync can read after this
  // build releases its lock even when the compiler exits unsuccessfully.
  // Use a sibling on dist's filesystem so promotion and restore can rename.
  const temporary = await mkdtemp(path.join(frontendRoot, '.dist-build-'));
  const stagingDir = path.join(temporary, 'dist');
  const backupDir = path.join(temporary, 'previous-dist');
  const distDir = path.join(frontendRoot, 'dist');
  let backupRetained = false;
  try {
    await run(process.execPath, [viteCLI, 'build', '--outDir', stagingDir, '--emptyOutDir'], {
      cwd: frontendRoot,
    });
    await validateBuildOutput(stagingDir);

    let replacedExistingDist = false;
    try {
      await rename(distDir, backupDir);
      replacedExistingDist = true;
    } catch (error) {
      if (error?.code !== 'ENOENT') throw error;
    }
    try {
      await rename(stagingDir, distDir);
    } catch (error) {
      if (replacedExistingDist) {
        try {
          await rename(backupDir, distDir);
        } catch (restoreError) {
          // Never let final cleanup erase the only retained successful build.
          backupRetained = true;
          throw new AggregateError(
            [error, restoreError],
            `Frontend output promotion and restore failed; previous build retained at ${backupDir}`,
          );
        }
      }
      throw error;
    }
  } finally {
    if (!backupRetained) await rm(temporary, { recursive: true, force: true });
  }
}

await withExclusiveLock(
  lockPath,
  async () => {
    await syncPublicDocs();
    await buildDist();
    await syncEmbedDir({ lock: false });
  },
  { description: 'frontend embedded asset build' },
);
