import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.resolve(root, '../tmp/publication-reconciliation-static');
const hash = (b) => crypto.createHash('sha256').update(b).digest('hex');
const binding = { sources: {}, artifacts: {} };
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      solid(),
      tailwindcss(),
      {
        name: 'bind-compiled-source',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const f = id.split('?')[0];
            if (
              f.startsWith(root + '/') &&
              !f.includes('/node_modules/') &&
              fs.existsSync(f) &&
              fs.statSync(f).isFile()
            )
              binding.sources[path.relative(root, f)] = hash(fs.readFileSync(f));
          }
        },
      },
    ],
    resolve: { alias: { '@': path.join(root, 'src') } },
    build: {
      target: 'esnext',
      outDir,
      emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/publication-reconciliation.html') },
    },
  });
  const walk = (d) => {
    for (const f of fs.readdirSync(d)) {
      const p = path.join(d, f);
      if (fs.statSync(p).isDirectory()) walk(p);
      else binding.artifacts[path.relative(outDir, p)] = hash(fs.readFileSync(p));
    }
  };
  walk(outDir);
  fs.writeFileSync(path.join(outDir, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
} finally {
  stop();
}
