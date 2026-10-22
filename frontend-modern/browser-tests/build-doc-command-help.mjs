// Compile in the source-proof VM, not the bounded browser process tree.
// This is a consuming preview, not the production build or a native agent.
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
const output = path.resolve(root, '../tmp/doc-command-help/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = {
  ...identity.publicationSourceIdentity(process.argv.slice(2), () =>
    execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' })),
  sources: {}, artifacts: {},
};
try {
  await build({
    root, base: '/', configFile: false, publicDir: false,
    plugins: [{
      name: 'real-docs-consumer-binding',
      generateBundle() {
        for (const id of this.getModuleIds()) {
          const file = id.split('?')[0];
          if (file.startsWith(root + '/') && !file.includes('/node_modules/') &&
              fs.existsSync(file) && fs.statSync(file).isFile())
            binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
        }
        for (const file of ['src/pages/Docs.tsx', 'src/features/docs/docMarkdown.ts',
          'src/components/shared/PageHeader.tsx', 'browser-tests/doc-command-help.tsx'])
          if (!binding.sources[file]) throw new Error(`Consumer not compiled: ${file}`);
      },
    }, tailwind(), solid()],
    resolve: { alias: { '@': path.join(root, 'src') } },
    // This fixture targets the installed current browser, not a legacy-engine
    // qualification. Match the other consuming preview's non-lowering target.
    build: { target: 'esnext', outDir: output, emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/doc-command-help.html') } },
  });
  const bind = directory => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) bind(file);
      else if (/\.(css|js|jsx|ts|tsx|html|mjs|cjs)$/.test(file))
        binding.sources[path.relative(root, file)] = hash(fs.readFileSync(file));
    }
  };
  // Tailwind scans source beyond the consumer graph. Bind those inputs too.
  bind(path.join(root, 'src')); bind(path.join(root, 'browser-tests'));
  for (const file of ['package.json', 'package-lock.json', 'tsconfig.json', 'index.html'])
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
  fs.mkdirSync(path.join(output, 'docs'), { recursive: true });
  for (const name of ['README', 'FAQ', 'UNIFIED_AGENT', 'TROUBLESHOOTING', 'DOCKER', 'PBS', 'MAIL_GATEWAY', 'AGENT_SECURITY',
    'RELEASE_PROCESS', 'AUTO_UPDATE', 'MIGRATION', 'DEPLOYMENT_MODELS', 'TRUENAS', 'VM_DISK_MONITORING', 'TEMPERATURE_MONITORING', 'PROXY_AUTH',
    'CLOUD', 'PULSE_PRO', 'AI_TRANSPARENCY', 'AI', 'AI_AUTONOMY', 'ASSISTANT_SAFETY', 'PRIVACY', 'ISSUE_TRIAGE', 'SECURITY', 'API', 'CONFIGURATION', 'INSTALL', 'KUBERNETES', 'OIDC', 'REVERSE_PROXY', 'CONTRIBUTING']) {
    const file = `public/docs/${name}.md`;
    binding.sources[file] = hash(fs.readFileSync(path.join(root, file)));
    fs.copyFileSync(path.join(root, file), path.join(output, 'docs', `${name}.md`));
  }
  const artifacts = directory => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) artifacts(file);
      else binding.artifacts[path.relative(output, file)] = hash(fs.readFileSync(file));
    }
  };
  artifacts(output);
  fs.writeFileSync(path.join(output, 'binding.json'), JSON.stringify(binding, null, 2) + '\n');
  console.log(JSON.stringify({ source: binding.source_sha,
    source_inputs: Object.keys(binding.sources).length, artifacts: Object.keys(binding.artifacts).length }));
} finally { stop(); }
