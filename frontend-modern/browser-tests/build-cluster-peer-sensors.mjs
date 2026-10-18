// Build the real adapter/drawer in the source-proof VM, not the browser gateway.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwind from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(root, '../tmp/cluster-peer-sensors/static');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const binding = { sources: {}, artifacts: {} };
try {
  await build({
    root, base: '/', configFile: false, publicDir: false,
    plugins: [
      {
        name: 'peer-sensor-source-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (file.startsWith(root + '/') && !file.includes('/node_modules/') && fs.existsSync(file) && fs.statSync(file).isFile()) {
              binding.sources[path.relative(root, file)] = hash(file);
            }
          }
          for (const required of ['src/components/Workloads/NodeDrawer.tsx', 'src/components/Workloads/NodeDrawerOverview.tsx', 'src/utils/resourceStateAdapters.ts']) {
            if (!binding.sources[required]) throw new Error(`Consumer not compiled: ${required}`);
          }
        },
      }, tailwind(), solid(),
    ],
    resolve: { alias: { '@': path.join(root, 'src') } },
    build: { target: 'esnext', outDir: output, emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/cluster-peer-sensors.html') } },
  });
  for (const file of ['package.json', 'package-lock.json', 'browser-tests/build-cluster-peer-sensors.mjs']) binding.sources[file] = hash(path.join(root, file));
  const bind = (directory) => {
    for (const entry of fs.readdirSync(directory, {withFileTypes:true})) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) bind(file);
      else binding.artifacts[path.relative(output, file)] = hash(file);
    }
  };
  bind(output);
  fs.writeFileSync(path.join(output, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
} finally { stop(); }
