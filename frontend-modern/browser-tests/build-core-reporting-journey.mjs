// Build outside the bounded browser tree; bind real consumers and CSS inputs.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwind from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import identity from './publication-fixture-source.cjs';
import { gzipSync } from 'node:zlib';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(root, '../tmp/core-reporting-journey/static');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = {
  ...identity.publicationSourceIdentity(process.argv.slice(2), () =>
    execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }),
  ),
  sources: {},
  artifacts: {},
};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'core-reporting-journey-consumer-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (
              file.startsWith(root + '/') &&
              !file.includes('/node_modules/') &&
              fs.existsSync(file) &&
              fs.statSync(file).isFile()
            )
              binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
          }
          for (const file of [
            'src/hooks/useUnifiedResources.ts',
            'src/features/truenas/TrueNASSystemsTable.tsx',
            'src/components/Infrastructure/ResourceDetailDrawer.tsx',
            'src/components/Workloads/GuestDrawerHistory.tsx',
            'src/api/charts.ts',
            'src/utils/apiClient.ts',
          ])
            if (!binding.sources[file]) throw new Error(`Real consumer not compiled: ${file}`);
        },
      },
      tailwind(),
      solid(),
    ],
    resolve: { alias: { '@': path.join(root, 'src') } },
    build: {
      target: 'esnext',
      outDir: output,
      emptyOutDir: true,
      rollupOptions: {
        input: [path.join(root, 'browser-tests/core-reporting-journey.html')],
      },
    },
  });
  const bind = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) bind(file);
      else if (/\.(css|js|jsx|ts|tsx|html|mjs|cjs)$/.test(file))
        binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
    }
  };
  bind(path.join(root, 'src'));
  bind(path.join(root, 'browser-tests'));
  for (const file of ['package.json', 'package-lock.json', 'tsconfig.json', 'index.html'])
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
  const artifacts = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) artifacts(file);
      else binding.artifacts[path.relative(output, file)] = hash(fs.readFileSync(file));
    }
  };
  artifacts(output);
  fs.writeFileSync(path.join(output, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
  const files = { 'binding.json': Buffer.from(JSON.stringify(binding)).toString('base64') };
  for (const file of Object.keys(binding.artifacts))
    files[file] = fs.readFileSync(path.join(output, file)).toString('base64');
  console.log('CORE_CONSUMER_ASSETS ' + gzipSync(Buffer.from(JSON.stringify(files))).toString('base64'));

} finally {
  stop();
}
