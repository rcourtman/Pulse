// Compile before the bounded browser tree starts. This is fixture preparation,
// not browser acceptance and not a production application/release build.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import {
  entryPaths,
  harnessPaths,
  runtimePaths,
  hash,
  loadPublicationFixture,
} from './publication-fixture-server.cjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.resolve(root, '../tmp/publication-final-static');
const started = Date.now();
const binding = {
  version: 1,
  source_sha: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim(),
  graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
  entries: entryPaths,
  sources: {},
  artifacts: {},
};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'publication-fixture-source-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (!file.startsWith(root + '/') || file.includes('/node_modules/')) continue;
            if (fs.existsSync(file) && fs.statSync(file).isFile()) {
              binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
            }
          }
          // A required runtime must really have entered the compiled graph,
          // not be added to the manifest merely because it exists on disk.
          for (const file of [...runtimePaths, 'src/index.css']) {
            if (!binding.sources[file])
              throw new Error(`Required runtime was not compiled: ${file}`);
          }
        },
      },
      tailwindcss(),
      solid(),
    ],
    resolve: {
      alias: { '@': path.join(root, 'src') },
      conditions: ['import', 'browser', 'default'],
    },
    build: {
      target: 'esnext',
      outDir,
      emptyOutDir: true,
      rollupOptions: { input: entryPaths.map((file) => path.join(root, file)) },
    },
  });
  for (const file of [
    ...harnessPaths,
    ...entryPaths,
    'package.json',
    'tsconfig.json',
    'index.html',
    'public/docs/VM_DISK_MONITORING.md',
  ]) {
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
  }
  // Tailwind's @source scan and palette imports also affect the served CSS,
  // even when a scanned TSX file is not in Rollup's JS module graph.
  const bindStyleInputs = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) bindStyleInputs(file);
      else if (/\.(css|js|jsx|ts|tsx|html)$/.test(file)) {
        binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
      }
    }
  };
  bindStyleInputs(path.join(root, 'src'));
  bindStyleInputs(path.join(root, 'browser-tests'));
  fs.mkdirSync(path.join(outDir, 'docs'), { recursive: true });
  fs.copyFileSync(
    path.join(root, 'public/docs/VM_DISK_MONITORING.md'),
    path.join(outDir, 'docs/VM_DISK_MONITORING.md'),
  );
  let bytes = 0;
  const walk = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(file);
      else {
        const data = fs.readFileSync(file);
        bytes += data.length;
        binding.artifacts[path.relative(outDir, file)] = hash(data);
      }
    }
  };
  walk(outDir);
  fs.writeFileSync(path.join(outDir, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
  loadPublicationFixture(root, outDir);
  console.log(
    JSON.stringify({
      result: 'prepared-not-browser-verified',
      source_sha: binding.source_sha,
      entries: entryPaths.length,
      sources: Object.keys(binding.sources).length,
      artifacts: Object.keys(binding.artifacts).length,
      artifact_bytes: bytes,
      elapsed_ms: Date.now() - started,
      content_sha256: Object.fromEntries(runtimePaths.map((file) => [file, binding.sources[file]])),
    }),
  );
} finally {
  stop();
}
