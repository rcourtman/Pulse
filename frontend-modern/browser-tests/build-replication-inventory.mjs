// Fixture-only compilation in a worker-owned exact-source copy, not the
// production/embed build. The consuming page and every served byte are bound.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.resolve(root, '../static');
const dependencyRoot = fs.realpathSync(path.join(root, 'node_modules'));
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const sources = {};
const dependencyInputs = {};
const artifacts = {};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    resolve: { alias: { '@': path.join(root, 'src') } },
    plugins: [
      {
        name: 'replication-inventory-source-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (!fs.existsSync(file) || !fs.statSync(file).isFile()) continue;
            if (file.startsWith(dependencyRoot + '/')) {
              dependencyInputs[path.relative(dependencyRoot, file)] = hash(fs.readFileSync(file));
            } else if (file.startsWith(root + '/') && !file.includes('/node_modules/')) {
              sources[path.relative(root, file)] = hash(fs.readFileSync(file));
            }
          }
        },
      },
      tailwindcss(),
      solid(),
    ],
    build: {
      target: 'esnext',
      outDir,
      emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/replication-inventory.html') },
    },
  });
  const scan = (directory, destination, suffix) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) scan(file, destination, suffix);
      else if (!suffix || suffix.test(file))
        destination[path.relative(destination === sources ? root : outDir, file)] = hash(
          fs.readFileSync(file),
        );
    }
  };
  // Tailwind scans source utility strings beyond the imported module graph.
  scan(path.join(root, 'src'), sources, /\.(tsx?|css)$/);
  scan(path.join(root, 'browser-tests'), sources, /\.(tsx?|html)$/);
  for (const file of [
    'package.json',
    'package-lock.json',
    'browser-tests/build-replication-inventory.mjs',
    'browser-tests/replication-result-verdicts.cjs',
  ])
    sources[file] = hash(fs.readFileSync(path.join(root, file)));
  scan(outDir, artifacts);
  fs.writeFileSync(
    path.join(outDir, 'binding.json'),
    JSON.stringify({ sources, dependencyInputs, artifacts }, null, 2) + '\n',
  );
} finally {
  stop();
}
