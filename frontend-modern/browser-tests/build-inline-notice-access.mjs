// Prepare a static production-component preview before bounded browser use.
// This is not the production build or browser proof. All served bytes and
// Tailwind inputs are bound; no compiler runs in the browser process tree.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import sourceIdentity from './publication-fixture-source.cjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const variant = process.argv[2];
if (!['parent', 'final'].includes(variant)) throw new Error('Expected parent or final');
const outDir = path.resolve(root, `../tmp/inline-notice-access/${variant}-static`);
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = {
  ...sourceIdentity.publicationSourceIdentity(process.argv.slice(3), () =>
    execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }),
  ),
  variant, graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))), sources: {}, artifacts: {},
};
try {
  await build({
    root, base: '/', configFile: false, publicDir: false,
    plugins: [{
      name: 'notice-action-source-binding', generateBundle() {
        for (const id of this.getModuleIds()) {
          const file = id.split('?')[0];
          if (file.startsWith(root + '/') && !file.includes('/node_modules/') && fs.existsSync(file) && fs.statSync(file).isFile())
            binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
        }
        for (const file of ['src/components/shared/InlineNotice.tsx', 'src/components/Workloads/GuestDrawer.tsx', 'src/components/Infrastructure/ResourceDetailDrawer.tsx', 'src/components/Settings/RolesPanel.tsx', 'src/components/Settings/UserAssignmentsPanel.tsx', 'src/pages/Docs.tsx'])
          if (!binding.sources[file]) throw new Error(`Required consumer not compiled: ${file}`);
      },
    }, tailwindcss(), solid()],
    resolve: { alias: { '@': path.join(root, 'src') }, conditions: ['import', 'browser', 'default'] },
    build: { target: 'esnext', outDir, emptyOutDir: true, rollupOptions: { input: path.join(root, 'browser-tests/inline-notice-access.html') } },
  });
  const bind = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) bind(file);
      else if (/\.(css|js|jsx|ts|tsx|html|mjs|cjs)$/.test(file)) binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
    }
  };
  bind(path.join(root, 'src')); bind(path.join(root, 'browser-tests'));
  for (const file of ['index.html', 'package.json', 'package-lock.json', 'tsconfig.json', 'public/docs/VM_DISK_MONITORING.md'])
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
  fs.mkdirSync(path.join(outDir, 'docs'), { recursive: true });
  fs.copyFileSync(path.join(root, 'public/docs/VM_DISK_MONITORING.md'), path.join(outDir, 'docs/VM_DISK_MONITORING.md'));
  const artifacts = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) artifacts(file);
      else binding.artifacts[path.relative(outDir, file)] = hash(fs.readFileSync(file));
    }
  };
  artifacts(outDir);
  fs.writeFileSync(path.join(outDir, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
  console.log(JSON.stringify({ variant, source: binding.source_sha, source_inputs: Object.keys(binding.sources).length, artifacts: Object.keys(binding.artifacts).length }));
} finally { stop(); }
