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

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(root, '../tmp/installer-transport/static');
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
        name: 'installer-transport-consumer-binding',
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
            'src/components/Settings/useInfrastructureInstallState.tsx',
            'src/components/Settings/useInfrastructureOperationsState.tsx',
            'src/components/Settings/infrastructureOperationsModel.tsx',
            'src/utils/agentInstallCommand.ts',
            'src/api/security.ts',
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
        input: [path.join(root, 'browser-tests/installer-transport.html')],
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
  console.log(
    JSON.stringify({
      source: binding.source_sha,
      inputs: Object.keys(binding.sources).length,
      artifacts: Object.keys(binding.artifacts).length,
    }),
  );
} finally {
  stop();
}
