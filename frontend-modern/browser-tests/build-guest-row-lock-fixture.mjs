// Compile the existing real-row/drawer fixture once, before bounded browser execution.
// No dev compiler, App, guest request or installer runs inside the browser tree.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(root, '../tmp/guest-row-lock-static');
const row = path.join(root, 'src/components/Workloads/GuestRow.tsx');
const parentRow = path.join(output, 'parent-row.tsx');
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const parentOnly = process.argv.includes('--with-parent');
const parentIndex = process.argv.indexOf('--parent');
const parentSha = parentIndex < 0 ? undefined : process.argv[parentIndex + 1];
if (parentOnly) {
  if (!/^[a-f0-9]{40}$/.test(parentSha ?? '')) {
    throw new Error('--with-parent requires --parent <exact 40-character parent SHA>');
  }
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(
    parentRow,
    execFileSync(
      'git',
      ['show', `${parentSha}:frontend-modern/src/components/Workloads/GuestRow.tsx`],
      { cwd: root },
    ),
  );
}

try {
  for (const variant of parentOnly ? ['parent', 'final'] : ['final']) {
    const sources = {};
    const artifacts = {};
    const outDir = path.join(output, variant);
    await build({
      root,
      // Both variants are served below their own prefix, including nested HTML.
      base: './',
      configFile: false,
      publicDir: false,
      plugins: [
        {
          name: 'exact-guest-row-fixture-binding',
          enforce: 'pre',
          load(id) {
            if (variant === 'parent' && id === row) return fs.readFileSync(parentRow, 'utf8');
          },
          generateBundle(_options, bundle) {
            for (const id of this.getModuleIds()) {
              if (!id.startsWith(root + '/') || id.includes('/node_modules/')) continue;
              const file = id.split('?')[0];
              if (!fs.existsSync(file) || !fs.statSync(file).isFile()) continue;
              sources[path.relative(root, file)] = hash(
                fs.readFileSync(variant === 'parent' && file === row ? parentRow : file),
              );
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
        rollupOptions: { input: path.join(root, 'browser-tests/guest-row-memory-provenance.html') },
      },
    });
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
          ...(variant === 'parent' ? { parent_sha: parentSha } : {}),
          graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
          builder: hash(fs.readFileSync(fileURLToPath(import.meta.url))),
          sources,
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
