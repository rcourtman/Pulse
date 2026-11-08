// @vitest-environment node

import { spawnSync } from 'node:child_process';
import {
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { afterEach, describe, expect, it } from 'vitest';
import { completeBuild } from './build-output-fixture.mjs';

const repository = fileURLToPath(new URL('../../../', import.meta.url));
const roots = [];
const previousFiles = completeBuild({
  'assets/previous.js': 'console.log("previous successful frontend");\n',
  'docs/previous.md': 'Previously usable help.\n',
}, { entry: 'previous.js' });
const completedFiles = completeBuild({
  'assets/current.js': 'import "./vendor.js"; import("./lazy.js"); console.log("new frontend from this compiler");\n',
  'assets/vendor.js': 'export const vendor = true;\n',
  'assets/lazy.js': 'export const lazy = true;\n',
  'docs/current.md': 'New local help.\n',
}, { entry: 'current.js', preloads: ['vendor.js'] });

function writeFiles(root, files) {
  for (const [name, bytes] of Object.entries(files)) {
    const file = path.join(root, name);
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, bytes);
  }
}

function tree(root) {
  if (!existsSync(root)) return null;
  return Object.fromEntries(
    readdirSync(root, { recursive: true, withFileTypes: true })
      .filter((entry) => entry.isFile())
      .map((entry) => {
        const file = path.join(entry.parentPath, entry.name);
        return [path.relative(root, file), readFileSync(file)];
      }),
  );
}

function fixture({ previous = true, files = completedFiles, code = 0, signal, failure } = {}) {
  const root = mkdtempSync(path.join(os.tmpdir(), 'pulse-frontend-producer-'));
  roots.push(root);
  for (const name of [
    'frontend-modern/scripts/build-embed-assets.mjs',
    'frontend-modern/scripts/sync-embed-dist.mjs',
    'frontend-modern/scripts/build-integrity.mjs',
    'frontend-modern/scripts/sync-public-docs.mjs',
    'scripts/exclusive-lock.mjs',
  ]) {
    const destination = path.join(root, name);
    mkdirSync(path.dirname(destination), { recursive: true });
    cpSync(path.join(repository, name), destination);
  }
  const modules = path.join(root, 'frontend-modern', 'node_modules');
  mkdirSync(modules, { recursive: true });
  symlinkSync(path.join(repository, 'frontend-modern', 'node_modules', 'jsdom'), path.join(modules, 'jsdom'), 'junction');
  writeFiles(
    root,
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
  const source = path.join(root, 'frontend-modern', 'dist');
  const target = path.join(root, 'internal', 'api', 'frontend-modern', 'dist');
  if (previous) {
    writeFiles(source, previousFiles);
    writeFiles(target, previousFiles);
  }
  writeFiles(root, {
    'compiler-fixture.json': JSON.stringify({ files, code, signal }),
    // Execute the real builder, copy, docs and lock. Only the compiler is a
    // fixture: emulate Vite's --outDir/default and destructive output reset.
    'frontend-modern/node_modules/vite/bin/vite.js': `
const fs = require('node:fs');
const path = require('node:path');
const config = JSON.parse(fs.readFileSync(path.resolve('..', 'compiler-fixture.json')));
if (process.argv[2] !== 'build') throw new Error('Unexpected compiler invocation');
const index = process.argv.indexOf('--outDir');
const output = path.resolve(index === -1 ? 'dist' : process.argv[index + 1]);
fs.writeFileSync(path.resolve('..', 'observed-compiler.json'), JSON.stringify({ output, args: process.argv.slice(2) }));
fs.rmSync(output, { recursive: true, force: true });
for (const [name, bytes] of Object.entries(config.files)) {
  const file = path.join(output, name);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, bytes);
}
if (config.signal) process.kill(process.pid, config.signal);
else process.exitCode = config.code;
`,
    // Inject explicit rename errors only, never fabricated success. These
    // cases establish rollback/backup custody, not real disk-fault recovery.
    'rename-failure.mjs': `
import fs from 'node:fs';
import path from 'node:path';
import { syncBuiltinESMExports } from 'node:module';
const rename = fs.promises.rename;
const failure = ${JSON.stringify(failure ?? null)};
const canonical = ${JSON.stringify(source)};
fs.promises.rename = async (from, to) => {
  const isPromotion = to === canonical && path.basename(from) === 'dist';
  const isRestore = to === canonical && path.basename(from) === 'previous-dist';
  const isBackup = from === canonical;
  if ((failure === 'backup' && isBackup) ||
      ((failure === 'promote' || failure === 'restore') && isPromotion) ||
      (failure === 'restore' && isRestore)) {
    throw Object.assign(new Error('Synthetic rename failure: ' + failure), { code: 'EACCES' });
  }
  return rename(from, to);
};
syncBuiltinESMExports();
`,
  });
  const invoke = (name, args = []) => {
    const result = spawnSync(
      process.execPath,
      [...args, path.join(root, 'frontend-modern', 'scripts', name)],
      { cwd: root, encoding: 'utf8', timeout: 15_000 },
    );
    expect(result.error).toBeUndefined();
    expect(result.signal).toBeNull();
    return result;
  };
  return {
    root,
    source,
    target,
    run: () =>
      invoke('build-embed-assets.mjs',
        failure ? ['--import', pathToFileURL(path.join(root, 'rename-failure.mjs')).href] : []),
    sync: () => invoke('sync-embed-dist.mjs'),
  };
}

function expectNoDebris(f) {
  expect(readdirSync(path.join(f.root, 'tmp')).sort()).toEqual(['locks']);
  expect(readdirSync(path.join(f.root, 'tmp', 'locks'))).toEqual([]);
  expect(readdirSync(path.dirname(f.source)).filter((name) => name.startsWith('.dist-build-'))).toEqual([]);
}

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

describe.each([true, false])('frontend producer with previous output: %s', (previous) => {
  it.each([
    { name: 'complete-looking nonzero output', code: 17, reason: /exited with code 17/ },
    { name: 'incomplete nonzero output', files: { 'assets/current.js': 'partial' }, code: 17, reason: /exited with code 17/ },
    { name: 'empty nonzero output', files: {}, code: 17, reason: /exited with code 17/ },
    { name: 'signalled complete-looking output', signal: 'SIGTERM', reason: /terminated by signal SIGTERM/ },
    { name: 'incomplete zero-exit output', files: { 'index.html': completedFiles['index.html'] }, reason: /frontend build output incomplete/ },
    ...['current.js', 'vendor.js', 'lazy.js'].map((asset) => ({
      name: `zero-exit output with stale ${asset} integrity`,
      files: { ...completedFiles, [`assets/${asset}`]: 'export const changedBytes = true;\n' },
      reason: /integrity mismatch against built asset bytes/,
    })),
    {
      name: 'zero-exit output without an import map',
      files: { ...completedFiles, 'index.html': completedFiles['index.html'].replace(/<script type="importmap">.*?<\/script>/, '') },
      reason: /importmap script block missing/,
    },
  ])('does not admit $name to canonical dist or the embed', (compiler) => {
    const f = fixture({ previous, ...compiler });
    const sourceBefore = tree(f.source);
    const embedBefore = tree(f.target);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toMatch(compiler.reason);
    expect(result.stdout).not.toContain('Synced frontend embed assets');
    expect(tree(f.source)).toEqual(sourceBefore);
    expect(tree(f.target)).toEqual(embedBefore);
    expectNoDebris(f);

    // Standalone sync is the real consumer after the failed producer releases
    // its lock. Complete-looking failed bytes must never reach it later either.
    const sync = f.sync();
    expect(sync.status, sync.stderr).toBe(previous ? 0 : 1);
    expect(tree(f.source)).toEqual(sourceBefore);
    expect(tree(f.target)).toEqual(embedBefore);
    expectNoDebris(f);
  });

  it('publishes successful output byte-exactly and removes obsolete files', () => {
    const f = fixture({ previous });
    const result = f.run();
    expect(result.status, result.stderr).toBe(0);
    expect(result.stdout).toContain('Synced frontend embed assets');
    const expected = Object.fromEntries(
      Object.entries(completedFiles).map(([name, value]) => [path.normalize(name), Buffer.from(value)]),
    );
    expect(tree(f.source)).toEqual(expected);
    expect(tree(f.target)).toEqual(expected);
    const observed = JSON.parse(readFileSync(path.join(f.root, 'observed-compiler.json'), 'utf8'));
    expect(observed.output).not.toBe(f.source);
    expect(path.basename(path.dirname(observed.output))).toMatch(/^\.dist-build-/);
    expect(path.dirname(path.dirname(observed.output))).toBe(path.dirname(f.source));
    expect(observed.args).toEqual(['build', '--outDir', observed.output, '--emptyOutDir']);
    expect(f.sync().status).toBe(0);
    expect(tree(f.target)).toEqual(expected);
    expectNoDebris(f);
  });
});

describe('producer promotion error custody', () => {
  it.each(['backup', 'promote'])('preserves the old build after an injected %s error', (failure) => {
    const f = fixture({ failure });
    const previous = tree(f.source);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain(`Synthetic rename failure: ${failure}`);
    expect(result.stdout).not.toContain('Synced frontend embed assets');
    expect(tree(f.source)).toEqual(previous);
    expect(tree(f.target)).toEqual(previous);
    expect(f.sync().status).toBe(0);
    expect(tree(f.target)).toEqual(previous);
    expectNoDebris(f);
  });

  it('does not create canonical output when first promotion fails', () => {
    const f = fixture({ previous: false, failure: 'promote' });
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('Synthetic rename failure: promote');
    expect(tree(f.source)).toBeNull();
    expect(tree(f.target)).toBeNull();
    expectNoDebris(f);
  });

  it('retains and names the last good backup if restoration also fails', () => {
    const f = fixture({ failure: 'restore' });
    const previous = tree(f.source);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('Frontend output promotion and restore failed');
    expect(result.stdout).not.toContain('Synced frontend embed assets');
    expect(tree(f.source)).toBeNull();
    expect(tree(f.target)).toEqual(previous);
    expect(readdirSync(path.join(f.root, 'tmp', 'locks'))).toEqual([]);
    const retained = readdirSync(path.dirname(f.source)).filter((name) => name.startsWith('.dist-build-'));
    expect(retained).toHaveLength(1);
    const backup = path.join(path.dirname(f.source), retained[0], 'previous-dist');
    expect(result.stderr).toContain(`previous build retained at ${backup}`);
    expect(tree(backup)).toEqual(previous);
    expect(f.sync().status).toBe(1);
    expect(tree(f.target)).toEqual(previous);
    expect(tree(backup)).toEqual(previous);
  });
});
