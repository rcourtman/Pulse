// Compile before bounded browser execution and bind every imported module to
// the exact source-proof revision and served bytes. No compiler runs in browser.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import sourceIdentity from './publication-fixture-source.cjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.resolve(root, '../tmp/notification-recovery-proof/static');
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const sources = {},
  artifacts = {};
const identity = sourceIdentity.publicationSourceIdentity(process.argv.slice(2), () => {
  throw new Error('Pass the containing source-proof --source-sha explicitly');
});
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'notification-recovery-source-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (!file.startsWith(root + '/') || file.includes('/node_modules/')) continue;
            if (fs.existsSync(file) && fs.statSync(file).isFile())
              sources[path.relative(root, file)] = hash(fs.readFileSync(file));
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
      rollupOptions: {
        input: path.join(root, 'browser-tests/notification-recovery-boundary.html'),
      },
    },
  });
  for (const file of [
    'package.json',
    'package-lock.json',
    'browser-tests/notification-recovery-boundary.html',
    'browser-tests/notification-recovery-boundary.cjs',
    'browser-tests/build-notification-recovery-boundary.mjs',
  ])
    sources[file] = hash(fs.readFileSync(path.join(root, file)));
  const walk = (directory, visit) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(file, visit);
      else visit(file);
    }
  };
  // Tailwind scans source strings, even in modules outside the imported graph.
  walk(path.join(root, 'src'), (file) => {
    if (/\.(css|tsx?|json)$/.test(file))
      sources[path.relative(root, file)] = hash(fs.readFileSync(file));
  });
  walk(outDir, (file) => {
    artifacts[path.relative(outDir, file)] = hash(fs.readFileSync(file));
  });
  fs.writeFileSync(
    path.join(outDir, 'binding.json'),
    JSON.stringify(
      {
        version: 1,
        ...identity,
        graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
        sources,
        artifacts,
      },
      null,
      2,
    ) + '\n',
  );
} finally {
  stop();
}
