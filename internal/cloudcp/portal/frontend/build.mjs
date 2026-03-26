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

async function listSourceInputs(relativeDir) {
  const absoluteDir = path.join(frontendRoot, relativeDir);
  const entries = await fs.readdir(absoluteDir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const childRelativePath = path.posix.join(relativeDir, entry.name);
    if (entry.isDirectory()) {
      files.push(...await listSourceInputs(childRelativePath));
      continue;
    }
    if (entry.isFile() && /\.(ts|css)$/.test(entry.name)) {
      files.push(childRelativePath);
    }
  }
  files.sort();
  return files;
}

async function getBuildInputs() {
  const sourceInputs = await listSourceInputs('src');
  return [
    'package.json',
    'tsconfig.json',
    'build.mjs',
    ...sourceInputs,
  ];
}

async function computeSourceHash(buildInputs) {
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
    const buildInputs = await getBuildInputs();
    const sourceHash = await computeSourceHash(buildInputs);
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
