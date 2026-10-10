import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import { withExclusiveLock } from '../../scripts/exclusive-lock.mjs';
import { syncEmbedDir } from './sync-embed-dist.mjs';
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
    child.on('close', (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`${command} ${args.join(' ')} exited with code ${code}`));
    });
  });
}

await withExclusiveLock(
  lockPath,
  async () => {
    await syncPublicDocs();
    await run(process.execPath, [viteCLI, 'build'], { cwd: frontendRoot });
    await syncEmbedDir({ lock: false });
  },
  { description: 'frontend embedded asset build' },
);
