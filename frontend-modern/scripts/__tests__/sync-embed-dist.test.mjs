// @vitest-environment node

import { spawnSync } from 'node:child_process';
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
