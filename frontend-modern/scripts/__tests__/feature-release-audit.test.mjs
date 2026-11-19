import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

const auditPath = path.join(process.cwd(), 'scripts', 'feature-release-audit.mjs');
const temporaryRoots = [];

function writeFile(file, content) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
}

function git(repoRoot, ...args) {
  const result = spawnSync('git', ['-C', repoRoot, ...args], { encoding: 'utf8' });
  if (result.status !== 0) {
    throw new Error(`git ${args.join(' ')} failed: ${result.stderr}`);
  }
}

function makeRepo(registry, documents) {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-feature-release-audit-'));
  temporaryRoots.push(repoRoot);
  const frontendRoot = path.join(repoRoot, 'frontend-modern');
  const registryPath = path.join(frontendRoot, 'src', 'utils', 'featureReleases.json');
  writeFile(registryPath, JSON.stringify(registry, null, 2));
  for (const [relative, content] of Object.entries(documents)) {
    writeFile(path.join(repoRoot, ...relative.split('/')), content);
  }
  git(repoRoot, 'init', '--quiet');
  git(repoRoot, 'add', '.');
  return {
    run: () => spawnSync(process.execPath, [auditPath], { cwd: frontendRoot, encoding: 'utf8' }),
    write: (relative, content) => writeFile(path.join(repoRoot, ...relative.split('/')), content),
    setRegistry: (next) => writeFile(registryPath, JSON.stringify(next, null, 2)),
  };
}

const map = (overrides = {}) => ({
  infrastructureMap: { name: 'Map', stage: 'build', guides: ['MAP.md'], ...overrides },
});

const cleanDocuments = {
  'docs/MAP.md': '# Map\n',
  'docs/README.md': '# Guides\n\n- [Roadmap](ROADMAP.md)\n- [Mapping](docs/MAPPING)\n',
  'docs/ROADMAP.md': '# Roadmap\n',
  'frontend-modern/public/docs/MAP.md': '# Map\n\nSee [the map guide](MAP.md#tiles).\n',
  'frontend-modern/public/docs/README.md': '# Guides\n',
  'docs/release-control/v6/internal/subsystems/unified-resources.md':
    'The map guide is `docs/MAP.md`.\n',
};

afterEach(() => {
  while (temporaryRoots.length > 0) {
    fs.rmSync(temporaryRoots.pop(), { force: true, recursive: true });
  }
});

describe('feature release audit', () => {
  it('passes while a held feature is linked from nowhere that ships', () => {
    const repo = makeRepo(map(), cleanDocuments);
    const result = repo.run();
    expect(result.stderr).toBe('');
    expect(result.status).toBe(0);
    expect(result.stdout).toContain('1 feature(s) listed, 1 guide(s) held back and unlinked');
  });

  it('refuses a link to a held guide from a guide and from the in-app copy', () => {
    const repo = makeRepo(map(), cleanDocuments);
    repo.write('docs/README.md', '# Guides\n\n- [Map](MAP.md)\n');
    repo.write('frontend-modern/public/docs/README.md', 'Open [the map guide](/docs/MAP).\n');
    // A file nobody has added to Git is a local note, not a shipped guide.
    repo.write('docs/SCRATCH.md', 'See [the map](MAP.md).\n');

    const result = repo.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('docs/README.md:3 points to MAP.md');
    expect(result.stderr).toContain('frontend-modern/public/docs/README.md:1 points to MAP.md');
    expect(result.stderr).not.toContain('docs/SCRATCH.md');
  });

  it('catches the other spellings the docs page resolves, and a mention that is not a link', () => {
    for (const [line, why] of [
      ['[Map](/docs/./MAP)', 'a dot segment'],
      ['[Map](MAP.MD)', 'another case'],
      ['[Map](../docs/map.md#tiles)', 'lower case with a fragment'],
      ['Read `MAP.md` for how tiles are drawn.', 'a plain mention'],
    ]) {
      const repo = makeRepo(map(), {
        ...cleanDocuments,
        'docs/README.md': `# Guides\n\n${line}\n`,
      });
      const result = repo.run();
      expect(result.status, why).toBe(1);
      expect(result.stderr, why).toContain('docs/README.md:3 points to MAP.md');
    }
  });

  it('reads tracked release notes too', () => {
    const repo = makeRepo(map(), {
      ...cleanDocuments,
      'docs/releases/NOTES.md': 'New: [the map](../MAP.md#tiles).\n',
    });
    const result = repo.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('docs/releases/NOTES.md:1 points to MAP.md');
    expect(result.stderr).toContain('which is not released');
  });

  it('keeps refusing the link while the feature is in preview and allows it once released', () => {
    const linked = { ...cleanDocuments, 'docs/README.md': '# Guides\n\n- [Map](MAP.md)\n' };
    const repo = makeRepo(map({ stage: 'preview', approval: 'Richard, 1 December 2026' }), linked);
    expect(repo.run().status).toBe(1);

    repo.setRegistry(map({ stage: 'released', approval: 'Richard, 8 December 2026' }));
    const released = repo.run();
    expect(released.stderr).toBe('');
    expect(released.status).toBe(0);
  });

  it('refuses a stage past build with no approval line', () => {
    const repo = makeRepo(map({ stage: 'preview' }), cleanDocuments);
    const preview = repo.run();
    expect(preview.status).toBe(1);
    expect(preview.stderr).toContain('stage "preview" needs an "approval" line');

    repo.setRegistry(map({ stage: 'released', approval: '  ' }));
    const released = repo.run();
    expect(released.status).toBe(1);
    expect(released.stderr).toContain('stage "released" needs an "approval" line');
  });

  it('refuses an entry the app would not read as written', () => {
    const repo = makeRepo(
      {
        one: { name: 'One', stage: 'Released', guides: [] },
        two: { name: '', stage: 'build', guides: ['../MAP.md'] },
        three: 'build',
      },
      cleanDocuments,
    );
    const result = repo.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('one: "stage" must be one of build, preview, released');
    expect(result.stderr).toContain('two: "name" must say what users call the feature');
    expect(result.stderr).toContain('two: "guides" must list file names under docs/');
    expect(result.stderr).toContain('three: the entry must be an object');
  });

  it('refuses a guide that does not exist', () => {
    const repo = makeRepo(map({ guides: ['MAPS.md'] }), cleanDocuments);
    const result = repo.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('guide docs/MAPS.md is not a tracked file');
  });
});
