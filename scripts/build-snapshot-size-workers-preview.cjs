// Precompile the existing Backups and Alerts consumers outside browser time.
// Run in the selected offline frontend proof module; no product route is added.
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
  assert(fs.existsSync(path.join(root, 'browser-tests/backups.html')));
  const frontendRequire = createRequire(path.join(root, 'package.json'));
  const { build } = await import(pathToFileURL(frontendRequire.resolve('vite')).href);
  const solid = frontendRequire('vite-plugin-solid');
  const tailwind = (await import(pathToFileURL(frontendRequire.resolve('@tailwindcss/vite')).href)).default;
  const out = path.join(root, 'tmp/snapshot-size-workers');
  fs.mkdirSync(out, { recursive: true });
  const compiledSources = {};
  const compiled = path.join(out, 'static');
  try {
    await build({ root, configFile: false, publicDir: false, logLevel: 'error',
      resolve: { alias: { '@': path.join(root, 'src') }, conditions: ['import', 'browser', 'default'] },
      plugins: [tailwind(), solid(), { name: 'snapshot-size-consumers-source-binding', generateBundle() {
        for (const id of this.getModuleIds()) {
          const file = id.split('?')[0];
          if (file.startsWith(root + '/') && !file.includes('/node_modules/') && fs.existsSync(file) && fs.statSync(file).isFile())
            compiledSources[path.relative(root, file)] = hash(fs.readFileSync(file));
        }
      } }],
      build: { target: 'esnext', outDir: compiled, emptyOutDir: true, rollupOptions: { input: ['backups','alert-delivery-freshness'].map(name => path.join(root, `browser-tests/${name}.html`)) } },
    });
    const artifacts = {};
    const walk = dir => { for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, e.name);
      if (e.isDirectory()) walk(file); else artifacts[path.relative(compiled, file)] = hash(fs.readFileSync(file));
    } };
    walk(compiled);
    for (const file of ['package.json', 'package-lock.json']) compiledSources[file] = hash(fs.readFileSync(path.join(root, file)));
    fs.writeFileSync(path.join(compiled, 'binding.json'), JSON.stringify({ source, compiledSources, artifacts,
      helperSHA256: hash(fs.readFileSync(__filename)) }, null, 2) + '\n');
    const archive = path.join(out, 'compiled.tar.xz');
    execFileSync('tar', ['-cJf', archive, '-C', compiled, '.']);
    const bytes = fs.readFileSync(archive);
    assert(bytes.length < 1024 * 1024, 'return only bounded compiled consumers');
    console.log('SNAPSHOT_CONSUMERS_COMPILED_METADATA ' + JSON.stringify({ source, sha256: hash(bytes), bytes: bytes.length, artifacts, compiledSources }));
    console.log('SNAPSHOT_CONSUMERS_COMPILED_TAR_XZ ' + bytes.toString('base64'));
  } finally {
    frontendRequire('esbuild').stop();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
