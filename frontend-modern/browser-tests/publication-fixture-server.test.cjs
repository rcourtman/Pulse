const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { afterEach, test } = require('node:test');
const {
  createPublicationFixtureServer,
  loadPublicationFixture,
  runtimePaths,
  entryPaths,
  harnessPaths,
  hash,
} = require('./publication-fixture-server.cjs');

const temporary = [];
afterEach(() => {
  for (const directory of temporary.splice(0)) fs.rmSync(directory, { recursive: true });
});

function fixture() {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'publication-fixture-'));
  temporary.push(directory);
  const root = path.join(directory, 'frontend');
  const compiled = path.join(directory, 'compiled');
  const write = (base, file, content) => {
    fs.mkdirSync(path.dirname(path.join(base, file)), { recursive: true });
    fs.writeFileSync(path.join(base, file), content);
  };
  write(root, 'package-lock.json', '{}');
  const binding = {
    version: 1,
    source_sha: '1'.repeat(40),
    graph: hash('{}'),
    entries: entryPaths,
    sources: {},
    artifacts: {},
  };
  for (const file of [
    ...runtimePaths,
    ...entryPaths,
    ...harnessPaths,
    'src/index.css',
    'public/docs/VM_DISK_MONITORING.md',
  ]) {
    write(root, file, file);
    binding.sources[file] = hash(file);
  }
  for (const file of [...entryPaths, 'assets/fixture.js', 'assets/fixture.css']) {
    write(compiled, file, file);
    binding.artifacts[file] = hash(file);
  }
  write(compiled, 'docs/VM_DISK_MONITORING.md', 'public/docs/VM_DISK_MONITORING.md');
  binding.artifacts['docs/VM_DISK_MONITORING.md'] =
    binding.sources['public/docs/VM_DISK_MONITORING.md'];
  const save = () => write(compiled, 'binding.json', JSON.stringify(binding));
  save();
  return { root, compiled, binding, write, save };
}

test('loads complete source-bound fixture bytes', () => {
  const f = fixture();
  const loaded = loadPublicationFixture(f.root, f.compiled);
  assert.equal(loaded.binding.source_sha, f.binding.source_sha);
  assert.equal(loaded.assets.get('/assets/fixture.js').toString(), 'assets/fixture.js');
});

for (const file of [
  ...runtimePaths,
  ...harnessPaths,
  'src/index.css',
  'public/docs/VM_DISK_MONITORING.md',
]) {
  test(`rejects stale imported/bound source: ${file}`, () => {
    const f = fixture();
    f.write(f.root, file, 'changed');
    assert.throws(() => loadPublicationFixture(f.root, f.compiled), /imported source changed/);
  });
}

test('rejects a changed dependency lock', () => {
  const f = fixture();
  f.write(f.root, 'package-lock.json', '{"changed":true}');
  assert.throws(() => loadPublicationFixture(f.root, f.compiled), /dependency graph changed/);
});

test('rejects a required runtime missing from the manifest', () => {
  const f = fixture();
  delete f.binding.sources[runtimePaths[0]];
  f.save();
  assert.throws(() => loadPublicationFixture(f.root, f.compiled), /required source was not bound/);
});

for (const file of [
  'assets/fixture.js',
  'assets/fixture.css',
  entryPaths[0],
  'docs/VM_DISK_MONITORING.md',
]) {
  test(`rejects altered compiled bytes: ${file}`, () => {
    const f = fixture();
    f.write(f.compiled, file, 'changed');
    assert.throws(() => loadPublicationFixture(f.root, f.compiled), /compiled artifact changed/);
  });
}

test('rejects missing compiled entry and a substituted safety doc', () => {
  const f = fixture();
  delete f.binding.artifacts[entryPaths[0]];
  f.save();
  assert.throws(
    () => loadPublicationFixture(f.root, f.compiled),
    /required artifact was not compiled/,
  );
  f.binding.artifacts[entryPaths[0]] = hash(entryPaths[0]);
  f.binding.artifacts['docs/VM_DISK_MONITORING.md'] = hash('different guide');
  f.write(f.compiled, 'docs/VM_DISK_MONITORING.md', 'different guide');
  f.save();
  assert.throws(() => loadPublicationFixture(f.root, f.compiled), /strictly equal/);
});

for (const unsafe of ['../outside', '/absolute', 'assets/../outside', 'assets\\outside']) {
  test(`rejects unsafe manifest path: ${unsafe}`, () => {
    const f = fixture();
    f.binding.artifacts[unsafe] = hash('outside');
    f.save();
    assert.throws(() => loadPublicationFixture(f.root, f.compiled), /binding path/);
  });
}

test('rejects symlinked compiled files and source directories', () => {
  const f = fixture();
  fs.renameSync(path.join(f.compiled, 'assets/fixture.js'), path.join(f.compiled, 'actual.js'));
  fs.symlinkSync('../actual.js', path.join(f.compiled, 'assets/fixture.js'));
  assert.throws(() => loadPublicationFixture(f.root, f.compiled), /symlink/);
  fs.unlinkSync(path.join(f.compiled, 'assets/fixture.js'));
  fs.renameSync(path.join(f.compiled, 'actual.js'), path.join(f.compiled, 'assets/fixture.js'));
  fs.renameSync(path.join(f.root, 'src'), path.join(f.root, 'actual-src'));
  fs.symlinkSync('actual-src', path.join(f.root, 'src'));
  assert.throws(() => loadPublicationFixture(f.root, f.compiled), /symlink/);
});

test('refuses stale content before creating a listener', () => {
  const f = fixture();
  f.write(f.root, runtimePaths[0], 'changed');
  assert.throws(
    () => createPublicationFixtureServer(f.root, 5349, entryPaths[0], f.compiled),
    /source changed/,
  );
});

test('rejects unknown fixture entries and invalid ports', () => {
  const f = fixture();
  assert.throws(
    () => createPublicationFixtureServer(f.root, 5349, 'index.html', f.compiled),
    /unknown fixture/,
  );
  assert.throws(
    () => createPublicationFixtureServer(f.root, 0, entryPaths[0], f.compiled),
    /invalid loopback/,
  );
});

test('serves only verified bytes, explicit routes and GET/HEAD; closes the listener', async () => {
  const f = fixture();
  const server = createPublicationFixtureServer(f.root, 5349, entryPaths[0], f.compiled);
  f.write(f.compiled, 'assets/fixture.js', 'later changed on disk');
  f.write(f.compiled, 'unlisted.js', 'must not serve');
  const request = (url, method = 'GET') =>
    new Promise((resolve, reject) => {
      const req = http.request(
        { host: '127.0.0.1', port: 5349, path: url, method, agent: false },
        (res) => {
          const data = [];
          res.on('data', (chunk) => data.push(chunk));
          res.on('end', () =>
            resolve({
              status: res.statusCode,
              type: res.headers['content-type'],
              body: Buffer.concat(data).toString(),
            }),
          );
        },
      );
      req.once('error', reject);
      req.end();
    });
  await server.listen();
  try {
    assert.equal((await request('/')).body, entryPaths[0]);
    const js = await request('/assets/fixture.js?cache=1');
    assert.equal(js.body, 'assets/fixture.js');
    assert.match(js.type, /javascript/);
    assert.equal((await request('/assets/fixture.js', 'HEAD')).body, '');
    assert.equal((await request('/docs/VM_DISK_MONITORING')).body, entryPaths[1]);
    assert.match((await request('/docs/VM_DISK_MONITORING.md')).type, /markdown/);
    for (const url of [
      '/missing.js',
      '/missing.css',
      '/docs/missing.md',
      '/unlisted.js',
      '/binding.json',
      '/api/guests',
      '/%ZZ',
      '/%2e%2e%2foutside',
    ]) {
      assert((await request(url)).status >= 400, url);
    }
    assert.equal((await request('/assets/fixture.js', 'POST')).status, 405);
    assert.equal((await request('/api/guests', 'DELETE')).status, 405);
  } finally {
    await server.close();
  }
  await assert.rejects(request('/'), /ECONNREFUSED/);
});
