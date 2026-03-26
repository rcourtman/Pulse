import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';

import { withExclusiveLock } from '../../../../scripts/exclusive-lock.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = __dirname;
const repoRoot = path.resolve(frontendRoot, '../../../../');
const distRoot = path.resolve(frontendRoot, '../dist');
const lockPath = path.join(repoRoot, 'tmp', 'locks', 'pulse-account-frontend-build.lock');
const manifestPath = path.join(distRoot, 'build_manifest.json');
const buildInputs = [
  'package.json',
  'build.mjs',
  'src/index.ts',
  'src/shell.ts',
  'src/services.ts',
  'src/styles.css',
];

async function computeSourceHash() {
  const hash = crypto.createHash('sha256');
  for (const relativePath of buildInputs) {
    hash.update(relativePath);
    hash.update('\n');
    hash.update(await fs.readFile(path.join(frontendRoot, relativePath)));
    hash.update('\n');
  }
  return hash.digest('hex');
}

await withExclusiveLock(
  lockPath,
  async () => {
    const sourceHash = await computeSourceHash();
    await fs.mkdir(distRoot, { recursive: true });
    await fs.rm(path.join(distRoot, 'portal_app.js'), { force: true });
    await fs.rm(path.join(distRoot, 'portal_app.css'), { force: true });
    await fs.rm(manifestPath, { force: true });

    await build({
      absWorkingDir: frontendRoot,
      entryPoints: ['src/index.ts'],
      outfile: path.join(distRoot, 'portal_app.js'),
      bundle: true,
      format: 'iife',
      platform: 'browser',
      target: ['es2020'],
      legalComments: 'none',
      sourcemap: false,
      minify: false,
      logLevel: 'info',
    });

    await fs.writeFile(
      manifestPath,
      JSON.stringify(
        {
          source_hash: sourceHash,
          build_inputs: buildInputs,
        },
        null,
        2,
      ) + '\n',
      'utf8',
    );
  },
  { description: 'Pulse Account frontend build' },
);
