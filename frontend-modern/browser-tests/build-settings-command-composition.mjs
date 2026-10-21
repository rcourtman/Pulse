// Small static preview only. Actual tool versions and product graph are bound
// separately: this is not qualification of a missing current-graph snapshot.
import { build } from 'vite';
import solid from 'vite-plugin-solid';
import tailwind from '@tailwindcss/vite';
import { stop } from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import sourceIdentity from './publication-fixture-source.cjs';

const root = path.resolve('frontend-modern');
const output = path.resolve('tmp/settings-composition/static');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = {
  ...sourceIdentity.publicationSourceIdentity(process.argv.slice(2), () => {
    throw new Error('Supply the committed source SHA');
  }),
  sources: {},
  artifacts: {},
  product_graph: hash(fs.readFileSync(path.join(root, 'package-lock.json'))),
  preview_tools_graph: hash(fs.readFileSync('tmp/preview-tools/package-lock.json')),
  tool_versions: Object.fromEntries(
    ['vite', 'vite-plugin-solid', 'solid-js', 'esbuild', '@tailwindcss/vite'].map((name) => [
      name,
      JSON.parse(fs.readFileSync(path.join(root, 'node_modules', name, 'package.json'))).version,
    ]),
  ),
  scope:
    'Frozen offline compiler for static consumer pixels only. Not a current-graph source suite, dev server, production build or audit.',
};
try {
  await build({
    root,
    base: '/',
    configFile: false,
    publicDir: false,
    plugins: [
      {
        name: 'real-settings-consumer-binding',
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
            'src/components/Settings/InfrastructureInstallerSection.tsx',
            'src/components/Settings/NodeModalSetupGuideSection.tsx',
            'src/components/Settings/useInfrastructureInstallState.tsx',
            'src/components/Settings/useNodeModalState.ts',
          ]) {
            if (!binding.sources[file]) throw new Error(`Consumer not imported: ${file}`);
          }
        },
      },
      solid(),
      tailwind(),
    ],
    resolve: {
      alias: { '@': path.join(root, 'src') },
      conditions: ['import', 'browser', 'default'],
    },
    build: {
      target: 'esnext',
      outDir: output,
      emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/settings-command-composition.html') },
    },
  });
  const walk = (directory, visit) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(file, visit);
      else visit(file);
    }
  };
  walk(path.join(root, 'src'), (file) => {
    if (/\.(css|tsx?|json)$/.test(file))
      binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
  });
  for (const file of [
    'package.json',
    'package-lock.json',
    'browser-tests/settings-command-composition.tsx',
    'browser-tests/settings-command-composition.html',
    'browser-tests/settings-command-composition.cjs',
    'browser-tests/build-settings-command-composition.mjs',
  ])
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
  walk(output, (file) => {
    binding.artifacts[path.relative(output, file)] = hash(fs.readFileSync(file));
  });
  fs.writeFileSync(path.join(output, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
} finally {
  stop();
}
