// Compile a static, exact-import-bound preview before bounded browser execution.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const variant = process.argv[2];
if (!['before', 'after'].includes(variant)) throw new Error('Expected before or after');
// Supply the source owner's separately observed HEAD; isolated preview trees
// have no Git capability. Imported-file digests, not this claim, bind the bytes.
const sourceSha = process.argv[3];
if (!/^[a-f0-9]{40}$/.test(sourceSha || '')) throw new Error('Expected externally observed source SHA');
const output = path.resolve(root, '../tmp/recovery-backup-static', variant);
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const sources = {};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'recovery-source-binding',
        enforce: 'pre',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            if (!id.startsWith(root + '/') || id.includes('/node_modules/')) continue;
            const file = id.split('?')[0];
            if (fs.existsSync(file) && fs.statSync(file).isFile()) {
              sources[path.relative(root, file)] = hash(fs.readFileSync(file));
            }
          }
        },
      },
      tailwindcss(),
      solid(),
    ],
    resolve: { alias: { '@': path.join(root, 'src') }, conditions: ['import', 'browser', 'default'] },
    build: {
      target: 'esnext',
      outDir: output,
      emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/recovery-backup-polling.html') },
    },
  });
  fs.mkdirSync(path.join(output, 'docs'), { recursive: true });
  for (const doc of ['VM_DISK_MONITORING.md', 'README.md']) {
    const file = `public/docs/${doc}`;
    sources[file] = hash(fs.readFileSync(path.join(root, file)));
    fs.copyFileSync(path.join(root, file), path.join(output, 'docs', doc));
  }
  for (const file of [
    'package.json', 'package-lock.json', 'browser-tests/build-recovery-backup-polling.mjs',
    'browser-tests/recovery-backup-polling.cjs',
  ]) sources[file] = hash(fs.readFileSync(path.join(root, file)));
  const artifacts = {};
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file);
      else artifacts[path.relative(output, file)] = hash(fs.readFileSync(file));
    }
  };
  walk(output);
  fs.writeFileSync(path.join(output, 'binding.json'), JSON.stringify({
    variant, source_sha: sourceSha, source_observation: 'supplied-by-source-owner',
    sources, artifacts,
  }, null, 2) + '\n');
} finally {
  stop();
}
