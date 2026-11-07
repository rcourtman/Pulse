// @vitest-environment node

import { spawn, spawnSync } from 'node:child_process';
import {
  chmodSync,
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  readlinkSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { afterEach, describe, expect, it } from 'vitest';

const repository = fileURLToPath(new URL('../../../', import.meta.url));
const roots = [];
const oldFiles = {
  'index.html': '<!doctype html><script src="/assets/previous.js"></script>\n',
  'assets/previous.js': 'console.log("previous frontend");\n',
  'docs/retained.md': 'Previously usable help.\n',
};
const validFiles = {
  'index.html':
    '<!doctype html><script type="module" src="/assets/app.js"></script>\n<link rel="stylesheet" href="/assets/app.css">\n<link rel="modulepreload" href="/assets/vendor.js">\n<script type="importmap">{"integrity":{"/assets/lazy.js":"synthetic-integrity"}}</script>\n',
  'assets/app.js': 'console.log("current frontend");\n',
  'assets/vendor.js': 'export const vendor = true;\n',
  'assets/lazy.js': 'export const lazy = true;\n',
  'assets/app.css': 'body { color: black; }\n',
  'docs/current.md': 'Current help.\n',
};

function writeFiles(directory, files) {
  for (const [name, content] of Object.entries(files)) {
    const file = path.join(directory, name);
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, content);
  }
}

function tree(directory, prefix = '') {
  if (!existsSync(directory)) return null;
  const files = {};
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const relative = path.join(prefix, entry.name);
    if (entry.isDirectory()) Object.assign(files, tree(path.join(directory, entry.name), relative));
    else if (entry.isSymbolicLink())
      files[relative] = { link: readlinkSync(path.join(directory, entry.name)) };
    else files[relative] = readFileSync(path.join(directory, entry.name));
  }
  return files;
}

function fixture(files, { previous = true } = {}) {
  const root = mkdtempSync(path.join(os.tmpdir(), 'pulse-embed-sync-'));
  roots.push(root);
  // Execute the unmodified production CLI/helper at their real relative paths,
  // but own every file it can replace inside this disposable fixture repository.
  for (const name of [
    'frontend-modern/scripts/sync-embed-dist.mjs',
    'scripts/exclusive-lock.mjs',
  ]) {
    const destination = path.join(root, name);
    mkdirSync(path.dirname(destination), { recursive: true });
    cpSync(path.join(repository, name), destination);
  }
  const source = path.join(root, 'frontend-modern', 'dist');
  const target = path.join(root, 'internal', 'api', 'frontend-modern', 'dist');
  mkdirSync(source, { recursive: true });
  writeFiles(source, files);
  if (previous) writeFiles(target, oldFiles);
  const script = path.join(root, 'frontend-modern', 'scripts', 'sync-embed-dist.mjs');
  const run = (mode = 'CLI') =>
    spawnSync(
      process.execPath,
      mode === 'CLI'
        ? [script]
        : [
            '--input-type=module',
            '-e',
            `import { syncEmbedDir } from ${JSON.stringify(pathToFileURL(script).href)}; await syncEmbedDir({ lock: false });`,
          ],
      { cwd: root, encoding: 'utf8', timeout: 15_000 },
    );
  return { root, source, target, run };
}

function expectNoDebris(f) {
  expect(readdirSync(path.dirname(f.target)).sort()).toEqual(existsSync(f.target) ? ['dist'] : []);
  expect(existsSync(path.join(f.root, 'tmp', 'locks', 'frontend-embed-sync.lock'))).toBe(false);
  expect(existsSync(path.join(f.root, 'tmp', 'locks', 'frontend-embed-build.lock'))).toBe(false);
}

function startNode(root, args) {
  const child = spawn(process.execPath, args, { cwd: root, stdio: ['ignore', 'pipe', 'pipe'] });
  const result = { code: null, signal: null, error: null, stdout: '', stderr: '' };
  let settled = false;
  child.stdout.on('data', (data) => (result.stdout += data));
  child.stderr.on('data', (data) => (result.stderr += data));
  const done = new Promise((resolve) => {
    child.on('error', (error) => (result.error = error));
    child.on('close', (code, signal) => {
      clearTimeout(timeout);
      settled = true;
      Object.assign(result, { code, signal });
      resolve(result);
    });
  });
  const timeout = setTimeout(() => child.kill('SIGKILL'), 15_000);
  return { done, result, settled: () => settled };
}

async function waitForMarker(file, command) {
  const deadline = Date.now() + 5_000;
  while (!existsSync(file) && !command.settled() && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  expect(command.result.error).toBeNull();
  expect(existsSync(file), JSON.stringify(command.result)).toBe(true);
}

function prepareGatedCompiler(f, { initial, fail }) {
  for (const name of ['build-embed-assets.mjs', 'sync-public-docs.mjs']) {
    cpSync(
      path.join(repository, 'frontend-modern', 'scripts', name),
      path.join(f.root, 'frontend-modern', 'scripts', name),
    );
  }
  // Use the real docs producer too. Only Vite is replaced: a gated compiler
  // makes the normally fleeting, partially written output deterministic.
  writeFiles(
    f.root,
    Object.fromEntries(
      [
        'docs/README.md',
        'docs/MIGRATION_UNIFIED_NAV.md',
        'docs/PRIVACY.md',
        'docs/CONFIGURATION.md',
        'docs/UNIFIED_AGENT.md',
        'docs/AGENT_SECURITY.md',
        'docs/PROXY_AUTH.md',
        'docs/TRUENAS.md',
        'SECURITY.md',
        'TERMS.md',
      ].map((name) => [name, `Synthetic local help for ${name}\n`]),
    ),
  );
  const started = path.join(f.root, 'compiler-started');
  const release = path.join(f.root, 'compiler-continue');
  const blocked = path.join(f.root, 'sync-observed-build-lock');
  const partialFiles = { ...validFiles, 'assets/app.js': 'console.log("unfinished compiler");\n' };
  const finalFiles = fail
    ? { 'assets/app.js': 'console.log("failed compiler without an entry");\n' }
    : validFiles;
  writeFiles(f.root, {
    'compiler-fixture.json': JSON.stringify({
      source: f.source,
      started,
      release,
      initialFiles: initial === 'present' ? partialFiles : {},
      finalFiles,
      fail,
    }),
    'frontend-modern/node_modules/vite/bin/vite.js': `
const fs = require('node:fs');
const path = require('node:path');
const config = JSON.parse(fs.readFileSync(path.join(process.cwd(), '..', 'compiler-fixture.json')));
const write = (files) => {
  fs.rmSync(config.source, { recursive: true, force: true });
  for (const [name, content] of Object.entries(files)) {
    const file = path.join(config.source, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content);
  }
};
(async () => {
  write(config.initialFiles);
  fs.writeFileSync(config.started, 'compiler holds the production build lock');
  const deadline = Date.now() + 10000;
  while (!fs.existsSync(config.release)) {
    if (Date.now() > deadline) throw new Error('Fixture compiler gate timed out');
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  write(config.finalFiles);
  process.exitCode = config.fail ? 17 : 0;
})();
`,
    // Observe the real filesystem refusal; never supply a fake lock result
    // or run a second selection algorithm. --import leaves the actual CLI
    // entry point, process.argv and its lock helper unchanged.
    'observe-build-lock.mjs': `
import fs from 'node:fs';
import { syncBuiltinESMExports } from 'node:module';
const mkdir = fs.promises.mkdir;
fs.promises.mkdir = async (...args) => {
  try { return await mkdir(...args); }
  catch (error) {
    if (args[0] === ${JSON.stringify(path.join(f.root, 'tmp', 'locks', 'frontend-embed-build.lock'))}
        && error.code === 'EEXIST') {
      fs.writeFileSync(${JSON.stringify(blocked)}, 'observed EEXIST from the occupied production build lock');
    }
    throw error;
  }
};
syncBuiltinESMExports();
`,
  });
  return {
    started,
    release,
    blocked,
    builder: path.join(f.root, 'frontend-modern', 'scripts', 'build-embed-assets.mjs'),
    sync: path.join(f.root, 'frontend-modern', 'scripts', 'sync-embed-dist.mjs'),
    observer: path.join(f.root, 'observe-build-lock.mjs'),
  };
}

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

const invalidBuilds = [
  { name: 'empty directory', files: {}, reason: /index\.html/ },
  {
    name: 'missing index',
    files: { 'assets/app.js': validFiles['assets/app.js'] },
    reason: /index\.html/,
  },
  { name: 'empty index', files: { ...validFiles, 'index.html': '' }, reason: /index\.html/ },
  {
    name: 'index directory',
    files: { 'index.html/child': 'not an index', 'assets/app.js': validFiles['assets/app.js'] },
    reason: /index\.html/,
  },
  { name: 'missing assets', files: { 'index.html': validFiles['index.html'] }, reason: /assets/ },
  {
    name: 'assets file',
    files: { 'index.html': validFiles['index.html'], assets: 'not a directory' },
    reason: /assets/,
  },
  {
    name: 'CSS without a script reference',
    files: {
      'index.html': '<link href="/assets/app.css">',
      'assets/app.css': validFiles['assets/app.css'],
    },
    reason: /JavaScript/,
  },
  {
    name: 'unreferenced JavaScript',
    files: { ...validFiles, 'index.html': '<!doctype html><p>No entry script</p>' },
    reason: /JavaScript/,
  },
  {
    name: 'empty JavaScript',
    files: { ...validFiles, 'assets/app.js': '' },
    reason: /assets\/app\.js/,
  },
  {
    name: 'JavaScript directory',
    files: { 'index.html': validFiles['index.html'], 'assets/app.js/child': 'not JavaScript' },
    reason: /assets\/app\.js/,
  },
  ...['app.css', 'vendor.js', 'lazy.js'].map((asset) => ({
    name: `missing advertised ${asset}`,
    files: Object.fromEntries(
      Object.entries(validFiles).filter(([name]) => name !== `assets/${asset}`),
    ),
    reason: new RegExp(asset.replace('.', '\\.')),
  })),
  ...['../outside.js', 'nested/../../outside.js', 'nested\\outside.js'].map((asset) => ({
    name: `invalid asset path ${asset}`,
    files: { ...validFiles, 'index.html': `<script src="/assets/${asset}"></script>` },
    reason: /invalid \/assets\/ reference/,
  })),
];

describe.each(['CLI', 'builder'])('%s embed publication preserves usable output', (mode) => {
  it.each(invalidBuilds)(
    'rejects $name without replacing or leaving staging files',
    ({ files, reason }) => {
      const f = fixture(files);
      const previous = tree(f.target);
      const source = tree(f.source);
      const result = f.run(mode);
      expect(result.error).toBeUndefined();
      expect(result.status).toBe(1);
      expect(result.stderr).toContain('frontend build output incomplete');
      expect(result.stderr).toMatch(reason);
      expect(result.stdout).not.toContain('Synced frontend embed assets');
      expect(tree(f.target)).toEqual(previous);
      expect(tree(f.source)).toEqual(source);
      expectNoDebris(f);
    },
  );

  it('replaces a valid tree byte-exactly, including help, and removes obsolete files', () => {
    const f = fixture(validFiles);
    const source = tree(f.source);
    const result = f.run(mode);
    expect(result.error).toBeUndefined();
    expect(result.status, result.stderr).toBe(0);
    expect(result.stdout).toContain('Synced frontend embed assets');
    expect(tree(f.target)).toEqual(source);
    expect(tree(f.source)).toEqual(source);
    expectNoDebris(f);
  });

  it('accepts single-quoted and nested generated asset references', () => {
    const f = fixture({
      'index.html': "<script src='/assets/nested/entry.js'></script>",
      'assets/nested/entry.js': validFiles['assets/app.js'],
    });
    const result = f.run(mode);
    expect(result.status, result.stderr).toBe(0);
    expect(tree(f.target)).toEqual(tree(f.source));
    expectNoDebris(f);
  });

  it('creates the first valid embed but never creates an incomplete one', () => {
    const rejected = fixture({}, { previous: false });
    expect(rejected.run(mode).status).toBe(1);
    expect(tree(rejected.target)).toBeNull();
    expectNoDebris(rejected);
    const accepted = fixture(validFiles, { previous: false });
    expect(accepted.run(mode).status).toBe(0);
    expect(tree(accepted.target)).toEqual(tree(accepted.source));
    expectNoDebris(accepted);
  });
});

describe('copy failures and irregular staged output', () => {
  it.skipIf(process.platform === 'win32' || process.getuid?.() === 0)(
    'cleans its own partial copy without changing the previous target',
    () => {
      const f = fixture(validFiles);
      const previous = tree(f.target);
      const unreadable = path.join(f.source, 'assets', 'unreadable.js');
      writeFileSync(unreadable, 'synthetic unreadable asset');
      chmodSync(unreadable, 0o000);
      try {
        const result = f.run();
        expect(result.error).toBeUndefined();
        expect(result.status).toBe(1);
        expect(result.stderr).toContain('EACCES');
        expect(tree(f.target)).toEqual(previous);
        expectNoDebris(f);
      } finally {
        chmodSync(unreadable, 0o600);
      }
    },
  );

  it.skipIf(process.platform === 'win32').each(['index', 'asset', 'asset directory'])(
    'rejects a symbolic %s rather than following it out of its stage',
    (kind) => {
      const f = fixture(
        kind === 'index'
          ? { 'assets/app.js': validFiles['assets/app.js'] }
          : { 'index.html': '<script src="/assets/app.js"></script>' },
      );
      const previous = tree(f.target);
      const outside = path.join(f.root, 'fixture-outside-stage');
      if (kind === 'asset directory') {
        mkdirSync(outside);
        writeFileSync(path.join(outside, 'app.js'), validFiles['assets/app.js']);
        symlinkSync(outside, path.join(f.source, 'assets'), 'dir');
      } else {
        writeFileSync(
          outside,
          kind === 'index' ? validFiles['index.html'] : validFiles['assets/app.js'],
        );
        const link =
          kind === 'index'
            ? path.join(f.source, 'index.html')
            : path.join(f.source, 'assets', 'app.js');
        mkdirSync(path.dirname(link), { recursive: true });
        symlinkSync(outside, link);
      }
      const source = tree(f.source);
      const result = f.run();
      expect(result.status).toBe(1);
      expect(result.stderr).toContain('frontend build output incomplete');
      expect(tree(f.target)).toEqual(previous);
      expect(tree(f.source)).toEqual(source);
      expectNoDebris(f);
    },
  );
});

describe('standalone sync waits for its actual build producer', () => {
  it.each(['present', 'absent'])(
    'does not inspect or publish %s intermediate output while Vite owns the build lock',
    async (initial) => {
      const f = fixture(validFiles);
      const previous = tree(f.target);
      const gate = prepareGatedCompiler(f, { initial, fail: false });
      const build = startNode(f.root, [gate.builder]);
      let sync;
      try {
        await waitForMarker(gate.started, build);
        expect(existsSync(f.source)).toBe(initial === 'present');
        sync = startNode(f.root, ['--import', pathToFileURL(gate.observer).href, gate.sync]);
        await waitForMarker(gate.blocked, sync);
        expect(sync.settled()).toBe(false);
        expect(tree(f.target)).toEqual(previous);
        expect(readdirSync(path.dirname(f.target))).toEqual(['dist']);
      } finally {
        writeFileSync(gate.release, 'finish the owned fixture compiler');
        await Promise.all([build.done, sync?.done]);
      }
      expect(build.result).toMatchObject({ code: 0, signal: null, error: null });
      expect(sync.result).toMatchObject({ code: 0, signal: null, error: null });
      expect(build.result.stdout).toContain('Synced frontend embed assets');
      expect(sync.result.stdout).toContain('Synced frontend embed assets');
      expect(tree(f.target)).toEqual(tree(f.source));
      expect(readFileSync(path.join(f.target, 'assets', 'app.js'), 'utf8')).toBe(
        validFiles['assets/app.js'],
      );
      expectNoDebris(f);
    },
    20_000,
  );

  it('retains the usable embed when the held compiler fails and leaves incomplete output', async () => {
    const f = fixture(validFiles);
    const previous = tree(f.target);
    const gate = prepareGatedCompiler(f, { initial: 'present', fail: true });
    const build = startNode(f.root, [gate.builder]);
    let sync;
    try {
      await waitForMarker(gate.started, build);
      sync = startNode(f.root, ['--import', pathToFileURL(gate.observer).href, gate.sync]);
      await waitForMarker(gate.blocked, sync);
      expect(sync.settled()).toBe(false);
      expect(tree(f.target)).toEqual(previous);
    } finally {
      writeFileSync(gate.release, 'finish the failing owned fixture compiler');
      await Promise.all([build.done, sync?.done]);
    }
    expect(build.result).toMatchObject({ code: 1, signal: null, error: null });
    expect(build.result.stderr).toContain('exited with code 17');
    expect(sync.result).toMatchObject({ code: 1, signal: null, error: null });
    expect(sync.result.stderr).toContain('frontend build output incomplete');
    expect(sync.result.stderr).toContain('index.html');
    expect(build.result.stdout).not.toContain('Synced frontend embed assets');
    expect(sync.result.stdout).not.toContain('Synced frontend embed assets');
    expect(tree(f.target)).toEqual(previous);
    expectNoDebris(f);
  }, 20_000);
});
