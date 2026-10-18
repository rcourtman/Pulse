// Compile the real workload-disk-metrics fixture before bounded browser execution.
// Only a bounded static preview runs in the browser tree; no live compiler or guest command.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(root, '../tmp/workload-disk-metrics-static');
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');

try {
  for (const variant of ['final']) {
    const sources = {};
    const artifacts = {};
    const outDir = path.join(output, variant);
    await build({
      root,
      // Root-relative assets keep the real shipped Docs viewer functional in a new tab.
      base: '/',
      configFile: false,
      publicDir: false,
      plugins: [
        {
          name: 'exact-workload-disk-metrics-fixture-binding',
          enforce: 'pre',
          generateBundle(_options, bundle) {
            for (const id of this.getModuleIds()) {
              if (!id.startsWith(root + '/') || id.includes('/node_modules/')) continue;
              const file = id.split('?')[0];
              if (!fs.existsSync(file) || !fs.statSync(file).isFile()) continue;
              sources[path.relative(root, file)] = hash(fs.readFileSync(file));
            }
            for (const [file, asset] of Object.entries(bundle)) {
              artifacts[file] = hash(asset.type === 'chunk' ? asset.code : asset.source);
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
        // Match the application's declared target, not Vite's older default.
        target: 'esnext',
        outDir,
        emptyOutDir: true,
        rollupOptions: { input: path.join(root, 'browser-tests/workload-disk-metrics.html') },
      },
    });
    sources['browser-tests/build-workload-disk-metrics.mjs'] = hash(
      fs.readFileSync(fileURLToPath(import.meta.url)),
    );
    const importedSources = { ...sources };
    const scanInputs = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const file = path.join(dir, entry.name);
        if (entry.isDirectory()) scanInputs(file);
        else if (/\.(tsx?|css|html)$/.test(file))
          sources[path.relative(root, file)] = hash(fs.readFileSync(file));
      }
    };
    scanInputs(path.join(root, 'src'));
    scanInputs(path.join(root, 'browser-tests'));
    // Hash the bytes actually written, including HTML post-processing.
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
      JSON.stringify(
        {
          variant,
          graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
          builder: hash(fs.readFileSync(fileURLToPath(import.meta.url))),
          sources,
          importedSources,
          artifacts,
        },
        null,
        2,
      ) + '\n',
    );
  }
} finally {
  stop();
}
