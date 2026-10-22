// Compile only the real existing SystemLogsPanel; the fixture creates no route
// or feature in the product. Run in the selected offline frontend proof module.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { createRequire } = require('node:module');
const { pathToFileURL } = require('node:url');
const { execFileSync } = require('node:child_process');
const assert = require('node:assert/strict');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  const root = process.cwd();
  const source = process.argv[2];
  assert.match(source || '', /^[0-9a-f]{40}$/);
  assert(fs.existsSync(path.join(root, 'src/components/Settings/SystemLogsPanel.tsx')));
  const frontendRequire = createRequire(path.join(root, 'package.json'));
  const { build } = await import(pathToFileURL(frontendRequire.resolve('vite')).href);
  const solid = frontendRequire('vite-plugin-solid');
  const tailwind = (await import(pathToFileURL(frontendRequire.resolve('@tailwindcss/vite')).href)).default;
  const out = path.join(root, 'tmp/pbs-status-logs');
  fs.mkdirSync(out, { recursive: true });
  fs.writeFileSync(path.join(out, 'main.tsx'), `import { render } from 'solid-js/web';
import { SystemLogsPanel } from '../../src/components/Settings/SystemLogsPanel';
import '../../src/index.css';
render(() => <main style={{ padding: '8px', 'max-width': '1200px', margin: 'auto' }}><SystemLogsPanel /></main>, document.getElementById('root')!);
`);
  fs.writeFileSync(path.join(out, 'index.html'), '<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><title>PBS status diagnostics</title></head><body><div id="root"></div><script type="module" src="/tmp/pbs-status-logs/main.tsx"></script></body></html>');
  const compiledSources = {};
  const compiled = path.join(out, 'static');
  try {
    await build({ root, configFile: false, publicDir: false, logLevel: 'error',
      resolve: { alias: { '@': path.join(root, 'src') }, conditions: ['import', 'browser', 'default'] },
      plugins: [tailwind(), solid(), { name: 'pbs-status-logs-source-binding', generateBundle() {
        for (const id of this.getModuleIds()) {
          const file = id.split('?')[0];
          if (file.startsWith(root + '/') && !file.includes('/node_modules/') && fs.existsSync(file) && fs.statSync(file).isFile())
            compiledSources[path.relative(root, file)] = hash(fs.readFileSync(file));
        }
      } }],
      build: { target: 'esnext', outDir: compiled, emptyOutDir: true, rollupOptions: { input: path.join(out, 'index.html') } },
    });
    const artifacts = {};
    const walk = dir => { for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, e.name);
      if (e.isDirectory()) walk(file); else artifacts[path.relative(compiled, file)] = hash(fs.readFileSync(file));
    } };
    walk(compiled);
    for (const file of ['package.json', 'package-lock.json']) compiledSources[file] = hash(fs.readFileSync(path.join(root, file)));
    fs.writeFileSync(path.join(compiled, 'binding.json'), JSON.stringify({ source, compiledSources, artifacts,
      helperSHA256: hash(fs.readFileSync(__filename)), generatedFixture: { entry: hash(fs.readFileSync(path.join(out, 'main.tsx'))), html: hash(fs.readFileSync(path.join(out, 'index.html'))) } }, null, 2) + '\n');
    const archive = path.join(out, 'compiled.tar.xz');
    execFileSync('tar', ['-cJf', archive, '-C', compiled, '.']);
    const bytes = fs.readFileSync(archive);
    assert(bytes.length < 1024 * 1024, 'return only a bounded compiled panel');
    console.log('PBS_LOGS_COMPILED_METADATA ' + JSON.stringify({ source, sha256: hash(bytes), bytes: bytes.length, artifacts, compiledSources }));
    console.log('PBS_LOGS_COMPILED_TAR_XZ ' + bytes.toString('base64'));
  } finally {
    frontendRequire('esbuild').stop();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
