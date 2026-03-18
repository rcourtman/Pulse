import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import { withExclusiveLock } from '../../scripts/exclusive-lock.mjs';
import { syncEmbedDir } from './sync-embed-dist.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const lockPath = path.join(repoRoot, 'tmp', 'locks', 'frontend-embed-build.lock');
const npxCmd = process.platform === 'win32' ? 'npx.cmd' : 'npx';

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
    await run(npxCmd, ['vite', 'build'], { cwd: frontendRoot });
    await syncEmbedDir({ lock: false });
  },
  { description: 'frontend embedded asset build' },
);
