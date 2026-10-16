// Compile before bounded browser execution, and bind imported and served bytes.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = process.argv[2];
if (!/^[a-f0-9]{40}$/.test(source || ''))
  throw new Error('Expected source owner observed full SHA');
const output = path.resolve(root, '../tmp/backup-point-state/static');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const sources = {},
  artifacts = {};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'backup-point-source-binding',
        enforce: 'pre',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            if (!id.startsWith(root + '/') || id.includes('/node_modules/')) continue;
            const file = id.split('?')[0];
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
      outDir: output,
      emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/backup-point-state.html') },
    },
  });
  fs.mkdirSync(path.join(output, 'docs'), { recursive: true });
  for (const doc of ['PBS.md', 'README.md']) {
    const file = `public/docs/${doc}`;
    sources[file] = hash(fs.readFileSync(path.join(root, file)));
    fs.copyFileSync(path.join(root, file), path.join(output, 'docs', doc));
  }
  for (const file of [
    'package.json',
    'package-lock.json',
    'browser-tests/backup-point-state.cjs',
    'browser-tests/backup-location-access.cjs',
    'browser-tests/backup-provider-evidence.cjs',
    'browser-tests/backup-job-outcomes.cjs',
    'browser-tests/build-backup-point-state.mjs',
  ])
    sources[file] = hash(fs.readFileSync(path.join(root, file)));
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file);
      else artifacts[path.relative(output, file)] = hash(fs.readFileSync(file));
    }
  };
  walk(output);
  fs.writeFileSync(
    path.join(output, 'binding.json'),
    JSON.stringify(
      {
        source_sha: source,
        source_observation: 'supplied-by-source-owner',
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
