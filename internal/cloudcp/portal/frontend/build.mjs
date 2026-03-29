import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { build } from 'esbuild';

import { withExclusiveLock } from '../../../../scripts/exclusive-lock.mjs';
import { createPortalBuildOptions, distRoot, frontendRoot, manifestPath, repoRoot } from './build_config.mjs';

const lockPath = path.join(repoRoot, 'tmp', 'locks', 'pulse-account-frontend-build.lock');

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
    if (entry.isFile() && /\.(ts|css|woff2|woff|ttf|otf)$/.test(entry.name)) {
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
    'build_config.mjs',
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

    await build(createPortalBuildOptions());

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
