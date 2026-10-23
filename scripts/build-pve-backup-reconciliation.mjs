// Compile the production consumer outside the bounded browser process tree.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../frontend-modern/package.json', import.meta.url));
const { build } = await import(require.resolve('vite'));
const solid = (await import(require.resolve('vite-plugin-solid'))).default;
const tailwindcss = (await import(require.resolve('@tailwindcss/vite'))).default;
const { stop } = await import(require.resolve('esbuild'));
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../frontend-modern');
const outDir = path.resolve(root, '../tmp/pve-reconciliation-static');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const sources = {};
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
        name: 'bind-pve-reconciliation-consumer',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (
              file.startsWith(root + '/') &&
              !file.includes('/node_modules/') &&
              fs.existsSync(file) &&
              fs.statSync(file).isFile()
            ) {
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
      rollupOptions: { input: path.join(root, 'browser-tests/backups.html') },
    },
  });
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file);
      else artifacts[path.relative(outDir, file)] = hash(fs.readFileSync(file));
    }
  };
  walk(outDir);
  fs.writeFileSync(
    path.join(outDir, 'binding.json'),
    JSON.stringify({
      graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
      builder: hash(fs.readFileSync(fileURLToPath(import.meta.url))),
      sources,
      artifacts,
    }, null, 2) + '\n',
  );
} finally {
  stop();
}
