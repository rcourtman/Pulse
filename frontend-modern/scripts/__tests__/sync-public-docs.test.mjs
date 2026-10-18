import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { afterEach, describe, expect, it } from 'vitest';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const repoRoot = path.dirname(frontendRoot);
const scriptPath = path.join(frontendRoot, 'scripts', 'sync-public-docs.mjs');
const rootDocs = new Set(['SECURITY.md', 'TERMS.md', 'ARCHITECTURE.md', 'CONTRIBUTING.md']);
const temporaryRoots = [];
const legacyDocs = [
  'README.md',
  'MIGRATION_UNIFIED_NAV.md',
  'PRIVACY.md',
  'CONFIGURATION.md',
  'UNIFIED_AGENT.md',
  'AGENT_SECURITY.md',
  'PROXY_AUTH.md',
  'TRUENAS.md',
  'SECURITY.md',
  'TERMS.md',
];

function write(root, relative, content) {
  const file = path.join(root, relative);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
  return file;
}

function mirror(relative) {
  return path.join('frontend-modern', 'public', 'docs', relative);
}

function source(relative) {
  return rootDocs.has(relative) ? relative : path.join('docs', relative);
}

function fixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-shipped-docs-'));
  temporaryRoots.push(root);
  write(root, 'frontend-modern/scripts/sync-public-docs.mjs', fs.readFileSync(scriptPath));
  // Keep the old ten-guide route viable too, so omissions fail on stale
  // content rather than an unrelated missing legacy fixture.
  for (const relative of legacyDocs) {
    write(root, source(relative), `# Current ${relative}\n`);
    write(root, mirror(relative), `# Current ${relative}\n`);
  }
  return root;
}

function sync(root) {
  return spawnSync(
    process.execPath,
    [path.join(root, 'frontend-modern/scripts/sync-public-docs.mjs')],
    {
      cwd: root,
      encoding: 'utf8',
      timeout: 10_000,
    },
  );
}

function bytes(root, relative) {
  return fs.readFileSync(path.join(root, relative));
}

afterEach(() => {
  while (temporaryRoots.length > 0) {
    fs.rmSync(temporaryRoots.pop(), { recursive: true, force: true });
  }
});

describe('shipped documentation build sync', () => {
  it.each(['VM_DISK_MONITORING.md', 'PBS.md', 'RECOVERY.md'])(
    'refreshes the selected %s guide, not only the legacy ten',
    (relative) => {
      const root = fixture();
      write(root, source(relative), '# Current safe guidance\n');
      write(root, mirror(relative), '# Superseded guidance\n');

      const result = sync(root);

      expect(result.status, result.stderr).toBe(0);
      expect(bytes(root, mirror(relative))).toEqual(bytes(root, source(relative)));
    },
  );

  it('refreshes nested selected guides while keeping the root mapping top-level only', () => {
    const root = fixture();
    for (const relative of ['i18n/de/README.md', 'nested/SECURITY.md']) {
      write(root, source(relative), `# Current nested ${relative}\n`);
      write(root, mirror(relative), '# Stale nested guide\n');
    }
    write(root, 'SECURITY.md', '# Root security\n');

    const result = sync(root);

    expect(result.status, result.stderr).toBe(0);
    for (const relative of ['i18n/de/README.md', 'nested/SECURITY.md', 'SECURITY.md']) {
      expect(bytes(root, mirror(relative))).toEqual(bytes(root, source(relative)));
    }
  });

  it.each(['ARCHITECTURE.md', 'CONTRIBUTING.md'])(
    'takes selected %s from the repository root, not docs/',
    (relative) => {
      const root = fixture();
      write(root, relative, '# Correct root document\n');
      write(root, path.join('docs', relative), '# Not the source\n');
      write(root, mirror(relative), '# Stale copy\n');

      const result = sync(root);

      expect(result.status, result.stderr).toBe(0);
      expect(bytes(root, mirror(relative))).toEqual(bytes(root, relative));
    },
  );

  it('does not publish unselected source documents or change assets', () => {
    const root = fixture();
    write(root, 'docs/INTERNAL_ONLY.md', '# Not selected for publication\n');
    write(root, 'docs/release-control/internal/RECORD.md', '# Internal record\n');
    write(root, mirror('asset.svg'), '<svg><!-- existing asset --></svg>');
    const before = fs.readdirSync(path.join(root, 'frontend-modern/public/docs')).sort();

    const result = sync(root);

    expect(result.status, result.stderr).toBe(0);
    expect(fs.readdirSync(path.join(root, 'frontend-modern/public/docs')).sort()).toEqual(before);
    expect(bytes(root, mirror('asset.svg')).toString()).toBe('<svg><!-- existing asset --></svg>');
    expect(fs.existsSync(path.join(root, mirror('INTERNAL_ONLY.md')))).toBe(false);
    expect(fs.existsSync(path.join(root, mirror('release-control')))).toBe(false);
  });

  it('preserves source bytes and is idempotent', () => {
    const root = fixture();
    const relative = 'PBS.md';
    const content = Buffer.from('\uFEFF# Sicher — sauvegarde\r\n\r\nNo final newline');
    write(root, source(relative), content);
    write(root, mirror(relative), '# Old\n');

    expect(sync(root).status).toBe(0);
    expect(bytes(root, mirror(relative))).toEqual(content);
    const before = fs.statSync(path.join(root, mirror(relative))).mtimeMs;
    expect(sync(root).status).toBe(0);
    expect(bytes(root, mirror(relative))).toEqual(content);
    expect(fs.statSync(path.join(root, mirror(relative))).mtimeMs).toBe(before);
  });

  it.each(['missing', 'directory', 'symlink', 'symlinked-parent'])(
    'rejects a %s source before changing even an earlier valid guide',
    (kind) => {
      const root = fixture();
      write(root, 'docs/AA_VALID.md', '# New valid guidance\n');
      write(root, mirror('AA_VALID.md'), '# Keep until all sources are validated\n');
      const relative = kind === 'symlinked-parent' ? 'ZZ_DIRECTORY/GUIDE.md' : 'ZZ_INVALID.md';
      write(root, mirror(relative), '# Original invalid-guide copy\n');
      if (kind === 'directory') fs.mkdirSync(path.join(root, source(relative)));
      if (kind === 'symlink') {
        const other = write(root, 'private-placeholder.md', '# Must not be copied\n');
        fs.symlinkSync(other, path.join(root, source(relative)));
      }
      if (kind === 'symlinked-parent') {
        write(root, 'private-placeholder/GUIDE.md', '# Must not be copied\n');
        fs.symlinkSync(
          path.join(root, 'private-placeholder'),
          path.join(root, 'docs/ZZ_DIRECTORY'),
        );
      }

      const result = sync(root);

      expect(result.status).toBe(1);
      expect(result.stdout).not.toContain('Synced');
      expect(bytes(root, mirror('AA_VALID.md')).toString()).toBe(
        '# Keep until all sources are validated\n',
      );
      expect(bytes(root, mirror(relative)).toString()).toBe('# Original invalid-guide copy\n');
    },
  );

  it.each(['file', 'directory', 'docs-root'])(
    'refuses a symlinked destination %s without touching its target',
    (kind) => {
      const root = fixture();
      const external = write(root, 'private-placeholder/GUIDE.md', '# Untouched\n');
      if (kind === 'file') {
        write(root, 'docs/GUIDE.md', '# Must not overwrite a link\n');
        fs.symlinkSync(external, path.join(root, mirror('GUIDE.md')));
      } else if (kind === 'directory') {
        write(root, 'docs/linked/GUIDE.md', '# Must not overwrite through a directory\n');
        fs.symlinkSync(path.dirname(external), path.join(root, mirror('linked')));
      } else {
        fs.renameSync(
          path.join(root, 'frontend-modern/public/docs'),
          path.join(root, 'old-public-docs'),
        );
        fs.symlinkSync(path.dirname(external), path.join(root, 'frontend-modern/public/docs'));
      }

      const result = sync(root);

      expect(result.status).toBe(1);
      expect(result.stdout).not.toContain('Synced');
      expect(bytes(root, 'private-placeholder/GUIDE.md').toString()).toBe('# Untouched\n');
    },
  );

  it.each(['empty', 'missing'])(
    'fails closed on an %s publication selection instead of rebuilding a partial one',
    (kind) => {
      const root = fixture();
      fs.renameSync(
        path.join(root, 'frontend-modern/public/docs'),
        path.join(root, 'old-public-docs'),
      );
      if (kind === 'empty') fs.mkdirSync(path.join(root, 'frontend-modern/public/docs'));

      const result = sync(root);

      expect(result.status).toBe(1);
      expect(result.stdout).not.toContain('Synced');
      if (kind === 'empty')
        expect(fs.readdirSync(path.join(root, 'frontend-modern/public/docs'))).toEqual([]);
      else expect(fs.existsSync(path.join(root, 'frontend-modern/public/docs'))).toBe(false);
    },
  );

  it('refreshes the complete real shipped selection without adding or losing a guide', () => {
    const root = fixture();
    const shipped = path.join(frontendRoot, 'public/docs');
    const selected = fs
      .readdirSync(shipped, { recursive: true })
      .filter((name) => name.endsWith('.md'));
    expect(selected).toContain('VM_DISK_MONITORING.md');
    expect(selected).toContain('PBS.md');
    expect(selected.length).toBeGreaterThan(legacyDocs.length);
    for (const relative of selected) {
      write(root, source(relative), fs.readFileSync(path.join(repoRoot, source(relative))));
      write(root, mirror(relative), '# Deliberately stale fixture\n');
    }

    const result = sync(root);

    expect(result.status, result.stderr).toBe(0);
    const actual = fs
      .readdirSync(path.join(root, 'frontend-modern/public/docs'), { recursive: true })
      .filter((name) => name.endsWith('.md'))
      .sort();
    expect(actual).toEqual([...selected].sort());
    for (const relative of selected) {
      // Compare every byte without making the assertion library enumerate
      // large Buffer objects as individual JS properties.
      expect(bytes(root, mirror(relative)).equals(bytes(root, source(relative))), relative).toBe(
        true,
      );
    }
  });
});
