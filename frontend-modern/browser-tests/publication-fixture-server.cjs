// Serve only the already-compiled, source-bound presentation fixtures. Never
// compile, proxy an API, or fall back to HTML for a missing JS/CSS/doc asset.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');

const runtimePaths = [
  'src/components/Workloads/GuestDrawerOverview.tsx',
  'src/components/Workloads/GuestRow.tsx',
  'src/components/Workloads/MetricMiniSparkline.tsx',
  'src/components/Workloads/guestDrawerModel.ts',
  'src/features/patrol/PatrolAttentionWorkbench.tsx',
];
const entryPaths = [
  'browser-tests/guest-disk-provenance.html',
  'browser-tests/guest-reading-help.html',
  'browser-tests/guest-row-memory-provenance.html',
  'browser-tests/patrol-rule-removal.html',
];
const harnessPaths = [
  'browser-tests/build-publication-fixtures.mjs',
  'browser-tests/publication-fixture-server.cjs',
  'browser-tests/guest-disk-provenance.cjs',
  'browser-tests/guest-reading-help.cjs',
  'browser-tests/guest-row-memory-provenance.cjs',
  'browser-tests/patrol-rule-removal.cjs',
  'browser-tests/patrol-rule-removal-final.cjs',
];
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.md': 'text/markdown; charset=utf-8',
};

function readRegular(root, relative) {
  assert(typeof relative === 'string' && relative.length > 0, 'missing binding path');
  assert(!relative.includes('\\') && !path.posix.isAbsolute(relative), 'absolute binding path');
  assert(
    relative.split('/').every((part) => part && part !== '.' && part !== '..'),
    'unsafe binding path',
  );
  let file = root;
  for (const part of relative.split('/')) {
    file = path.join(file, part);
    assert(!fs.lstatSync(file).isSymbolicLink(), 'symlink in binding path');
  }
  assert(fs.statSync(file).isFile(), 'binding path is not a regular file');
  return fs.readFileSync(file);
}

function loadPublicationFixture(
  root,
  compiled = path.resolve(root, '../tmp/publication-final-static'),
) {
  const binding = JSON.parse(readRegular(compiled, 'binding.json'));
  assert.equal(binding.version, 1);
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  assert.equal(
    binding.graph,
    hash(readRegular(root, 'package-lock.json')),
    'dependency graph changed',
  );
  assert.equal(binding.entries.join('\n'), entryPaths.join('\n'), 'fixture entries changed');
  for (const [file, digest] of Object.entries(binding.sources)) {
    assert.match(digest, /^[a-f0-9]{64}$/);
    assert.equal(hash(readRegular(root, file)), digest, `imported source changed: ${file}`);
  }
  for (const file of [...runtimePaths, ...harnessPaths, ...entryPaths, 'src/index.css']) {
    assert(binding.sources[file], `required source was not bound: ${file}`);
  }
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    assert.match(digest, /^[a-f0-9]{64}$/);
    const bytes = readRegular(compiled, file);
    assert.equal(hash(bytes), digest, `compiled artifact changed: ${file}`);
    // Retain verified bytes, not paths: later writes cannot change what is served.
    assets.set('/' + file, bytes);
  }
  for (const file of [...entryPaths, 'docs/VM_DISK_MONITORING.md']) {
    assert(assets.has('/' + file), `required artifact was not compiled: ${file}`);
  }
  const doc = 'public/docs/VM_DISK_MONITORING.md';
  assert(binding.sources[doc], 'shipped safety document was not bound');
  assert.equal(binding.artifacts['docs/VM_DISK_MONITORING.md'], binding.sources[doc]);
  return { binding, assets };
}

function createPublicationFixtureServer(root, port, entry, compiled) {
  assert(entryPaths.includes(entry), 'unknown fixture entry');
  assert(Number.isInteger(port) && port > 0 && port < 65536, 'invalid loopback port');
  // Fail before opening a listener when any source or served byte is stale.
  const { binding, assets } = loadPublicationFixture(root, compiled);
  const server = http.createServer((req, res) => {
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      res.writeHead(405);
      return res.end();
    }
    let pathname;
    try {
      pathname = decodeURIComponent(new URL(req.url, 'http://127.0.0.1').pathname);
    } catch {
      res.writeHead(400);
      return res.end();
    }
    // This single SPA route mounts the real Docs renderer from its own fixture.
    if (pathname === '/docs/VM_DISK_MONITORING')
      pathname = '/browser-tests/guest-reading-help.html';
    if (pathname === '/') pathname = '/' + entry;
    const bytes = assets.get(pathname);
    if (!bytes) {
      res.writeHead(404);
      return res.end();
    }
    res.setHeader('Content-Type', mime[path.extname(pathname)] || 'application/octet-stream');
    res.setHeader('Content-Length', bytes.length);
    res.setHeader('Cache-Control', 'no-store');
    res.end(req.method === 'HEAD' ? undefined : bytes);
  });
  return {
    binding,
    listen: () =>
      new Promise((resolve, reject) => {
        server.once('error', reject);
        server.listen(port, '127.0.0.1', resolve);
      }),
    close: () =>
      new Promise((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      }),
  };
}

module.exports = {
  createPublicationFixtureServer,
  loadPublicationFixture,
  runtimePaths,
  entryPaths,
  harnessPaths,
  hash,
};
