import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

const auditPath = path.join(process.cwd(), 'scripts', 'shared-template-audit.mjs');
const temporaryRoots = [];

const writeFile = (root, relativePath, content) => {
  const fullPath = path.join(root, ...relativePath.split('/'));
  fs.mkdirSync(path.dirname(fullPath), { recursive: true });
  fs.writeFileSync(fullPath, content);
};

const makeFixture = (registry, files) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-shared-template-audit-'));
  temporaryRoots.push(root);
  writeFile(root, 'scripts/shared-template-registry.json', JSON.stringify(registry, null, 2));
  for (const [relativePath, content] of Object.entries(files)) {
    writeFile(root, relativePath, content);
  }
  return {
    root,
    run: () =>
      spawnSync(process.execPath, [auditPath], {
        cwd: root,
        env: { ...process.env, PULSE_SHARED_TEMPLATE_AUDIT_ROOT: root },
        encoding: 'utf8',
      }),
  };
};

afterEach(() => {
  while (temporaryRoots.length > 0) {
    fs.rmSync(temporaryRoots.pop(), { force: true, recursive: true });
  }
});

describe('shared template audit', () => {
  it('detects violations across overlapping cached scopes and guards without cross-contamination', () => {
    const fixture = makeFixture(
      {
        rules: [
          {
            id: 'canonical-button',
            summary: 'Use shared Button',
            category: 'primitives',
            canonical: { path: 'src/shared/Button.tsx' },
            requiredConsumers: [{ path: 'src/features/GoodConsumer.tsx' }],
          },
        ],
        patternGuards: [
          {
            id: 'no-raw-search',
            summary: 'Use SearchField',
            category: 'primitives',
            canonical: { path: 'src/shared/SearchField.tsx' },
            scopes: ['src/features'],
            allPatterns: ['type="search"'],
            allowedPaths: ['src/features/StaleAllowed.tsx'],
          },
          {
            id: 'no-raw-dialog',
            summary: 'Use Dialog',
            category: 'primitives',
            canonical: { path: 'src/shared/Dialog.tsx' },
            scopes: ['src/features'],
            allPatterns: ['role="dialog"', 'fixed inset-0'],
          },
        ],
        requiredPatternGuards: [
          {
            id: 'missing-scope-check',
            summary: 'Missing scope fails closed',
            category: 'primitives',
            canonical: { path: 'src/shared/SearchField.tsx' },
            scopes: ['src/missing-dir'],
            triggerPatterns: ['placeholder='],
            requiredPatterns: ['SearchField'],
          },
        ],
      },
      {
        'src/shared/Button.tsx': 'export function Button() { return null; }\n',
        'src/shared/SearchField.tsx': 'export function SearchField() { return null; }\n',
        'src/shared/Dialog.tsx': 'export function Dialog() { return null; }\n',
        'src/features/GoodConsumer.tsx':
          'import { Button } from "../shared/Button"; export const A = () => <Button />;\n',
        'src/features/StaleAllowed.tsx': 'export const B = () => <input type="text" />;\n',
        'src/features/Violating.tsx':
          'export const C = () => <div role="dialog" class="fixed inset-0"><input type="search" /></div>;\n',
      },
    );

    const result = fixture.run();
    expect(result.status).toBe(1);
    expect(result.stderr).toContain(
      'no-raw-search: src/features/Violating.tsx recreates the shared template',
    );
    expect(result.stderr).toContain(
      'no-raw-search: allowed path src/features/StaleAllowed.tsx is stale',
    );
    expect(result.stderr).toContain(
      'no-raw-dialog: src/features/Violating.tsx recreates the shared template',
    );
    expect(result.stderr).toContain('pattern guard scope src/missing-dir does not exist');
  });

  it('passes when all rules and cached pattern guards are satisfied', () => {
    const fixture = makeFixture(
      {
        rules: [
          {
            id: 'canonical-button',
            summary: 'Use shared Button',
            category: 'primitives',
            canonical: { path: 'src/shared/Button.tsx' },
            requiredConsumers: [{ path: 'src/features/GoodConsumer.tsx' }],
          },
        ],
        patternGuards: [
          {
            id: 'no-raw-search',
            summary: 'Use SearchField',
            category: 'primitives',
            canonical: { path: 'src/shared/SearchField.tsx' },
            scopes: ['src/features'],
            allPatterns: ['type="search"'],
          },
        ],
        requiredPatternGuards: [
          {
            id: 'require-search-field',
            summary: 'Compose SearchField when searching',
            category: 'primitives',
            canonical: { path: 'src/shared/SearchField.tsx' },
            scopes: ['src/features'],
            triggerPatterns: ['role="searchbox"'],
            requiredPatterns: ['SearchField'],
          },
        ],
      },
      {
        'src/shared/Button.tsx': 'export function Button() { return null; }\n',
        'src/shared/SearchField.tsx': 'export function SearchField() { return null; }\n',
        'src/features/GoodConsumer.tsx':
          'import { Button } from "../shared/Button"; import { SearchField } from "../shared/SearchField"; export const A = () => <><Button /><SearchField role="searchbox" /></>;\n',
      },
    );

    const result = fixture.run();
    expect(result.status).toBe(0);
    expect(result.stderr).toBe('');
    expect(result.stdout).toContain(
      'Shared template audit passed (1 rule, 1 pattern guard, 1 required pattern guard',
    );
  });
});
