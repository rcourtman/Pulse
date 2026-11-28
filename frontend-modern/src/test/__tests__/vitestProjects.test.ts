import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import {
  needsJestDomForSource,
  needsNonEnglishLocalesForSource,
  partitionVitestFiles,
} from '../../../vitest.projects';

const tempDirs: string[] = [];

function createFixtureRoot(files: Record<string, string>): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-vitest-projects-'));
  tempDirs.push(root);
  for (const [relPath, content] of Object.entries(files)) {
    const fullPath = path.join(root, relPath);
    fs.mkdirSync(path.dirname(fullPath), { recursive: true });
    fs.writeFileSync(fullPath, content, 'utf8');
  }
  return root;
}

describe('vitest project partitioning and conditional setup helpers', () => {
  afterEach(() => {
    while (tempDirs.length > 0) {
      const dir = tempDirs.pop()!;
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it('classifies pure .test.ts files as node while keeping DOM, TSX, and unresolved imports in jsdom', () => {
    const root = createFixtureRoot({
      'src/utils/pure.ts': 'export const add = (a: number, b: number) => a + b;\n',
      'src/utils/domHelper.ts':
        'export type DomState = { ready: boolean };\nexport const readTitle = () => document.title;\n',
      'src/utils/__tests__/pure.test.ts':
        "import { add } from '@/utils/pure';\nimport type { DomState } from '@/utils/domHelper';\nexport const state: DomState = { ready: add(1, 2) === 3 };\n",
      'src/utils/__tests__/directDom.test.ts': 'export const title = window.location.href;\n',
      'src/utils/__tests__/transitiveDom.test.ts':
        "import { readTitle } from '@/utils/domHelper';\nexport const value = readTitle();\n",
      'src/utils/__tests__/explicitEnv.test.ts':
        '// @vitest-environment node\nexport const ok = true;\n',
      'src/utils/__tests__/unresolvedImport.test.ts':
        "import { missing } from '@/utils/doesNotExist';\nexport const value = missing;\n",
      'src/components/__tests__/Widget.test.tsx': "export const Widget = () => 'ok';\n",
    });

    const { nodeFiles } = partitionVitestFiles(root);
    expect(nodeFiles).toEqual(['src/utils/__tests__/pure.test.ts']);
  });

  it('ignores multi-line and inline type-only edges without skipping mixed or adjacent runtime imports', () => {
    const root = createFixtureRoot({
      'src/utils/domHelper.ts':
        'export type DomState = { ready: boolean };\nexport type DomMode = "a" | "b";\nexport const readTitle = () => document.title;\n',
      'src/utils/__tests__/multilineTypeOnly.test.ts': [
        'import type {',
        '  DomMode,',
        '  DomState,',
        "} from '@/utils/domHelper';",
        'export const state: DomState = { ready: true };',
        'export const mode: DomMode = "a";',
      ].join('\n'),
      'src/utils/__tests__/inlineTypeOnly.test.ts':
        "import { type DomState, type DomMode } from '@/utils/domHelper';\nexport const ok: [DomState, DomMode] = [{ ready: true }, 'a'];\n",
      'src/utils/__tests__/mixedTypeAndRuntime.test.ts':
        "import { type DomState, readTitle } from '@/utils/domHelper';\nexport const state: DomState = { ready: Boolean(readTitle()) };\n",
      'src/utils/__tests__/unterminatedExportTypeBeforeRuntime.test.ts':
        "export type LocalFlag = boolean\nimport { readTitle } from '@/utils/domHelper'\nexport const title = readTitle()\n",
    });

    const { nodeFiles } = partitionVitestFiles(root);
    expect(nodeFiles).toEqual([
      'src/utils/__tests__/inlineTypeOnly.test.ts',
      'src/utils/__tests__/multilineTypeOnly.test.ts',
    ]);
  });

  it('skips local DOM modules replaced by top-level zero-arg factory vi.mock unless unmock/importActual is present', () => {
    const root = createFixtureRoot({
      'src/utils/domClient.ts': 'export const readCookie = () => document.cookie;\n',
      'src/api/clientConsumer.ts':
        "import { readCookie } from '@/utils/domClient';\nexport const getToken = () => readCookie();\n",
      'src/api/__tests__/factoryMocked.test.ts': [
        "import { vi } from 'vitest';",
        "vi.mock('@/utils/domClient', () => ({ readCookie: () => 'tok' }));",
        "import { getToken } from '../clientConsumer';",
        'export const value = getToken();',
      ].join('\n'),
      'src/api/__tests__/automocked.test.ts': [
        "import { vi } from 'vitest';",
        "vi.mock('@/utils/domClient');",
        "import { getToken } from '../clientConsumer';",
        'export const value = getToken();',
      ].join('\n'),
      'src/api/__tests__/partialMock.test.ts': [
        "import { vi } from 'vitest';",
        "vi.mock('@/utils/domClient', async (importOriginal) => ({ ...(await importOriginal()) }));",
        "import { getToken } from '../clientConsumer';",
        'export const value = getToken();',
      ].join('\n'),
      'src/api/__tests__/doMocked.test.ts': [
        "import { vi } from 'vitest';",
        "vi.doMock('@/utils/domClient', () => ({ readCookie: () => 'tok' }));",
        "import { getToken } from '../clientConsumer';",
        'export const value = getToken();',
      ].join('\n'),
      'src/api/__tests__/resetModulesMock.test.ts': [
        "import { vi } from 'vitest';",
        "vi.mock('@/utils/domClient', () => ({ readCookie: () => 'tok' }));",
        'vi.resetModules();',
        "import { getToken } from '../clientConsumer';",
        'export const value = getToken();',
      ].join('\n'),
    });

    const { nodeFiles } = partitionVitestFiles(root);
    expect(nodeFiles).toEqual(['src/api/__tests__/factoryMocked.test.ts']);
  });

  it('detects jest-dom matcher usage and TSX files conservatively', () => {
    expect(needsJestDomForSource('src/components/Foo.test.tsx', 'expect(1).toBe(1)')).toBe(true);
    expect(needsJestDomForSource('src/utils/bar.test.ts', 'expect(el).toBeInTheDocument()')).toBe(
      true,
    );
    expect(needsJestDomForSource('src/utils/bar.test.ts', 'expect(add(1, 2)).toBe(3)')).toBe(false);
  });

  it('detects non-English locale tests while skipping English-only tests', () => {
    expect(needsNonEnglishLocalesForSource("setActiveLocale('de-DE')")).toBe(true);
    expect(needsNonEnglishLocalesForSource("expect(t('key', {}, 'fr')).toBe('Oui')")).toBe(true);
    expect(needsNonEnglishLocalesForSource('navigator.language')).toBe(true);
    expect(needsNonEnglishLocalesForSource("expect(formatBytes(1024)).toBe('1 KB')")).toBe(false);
  });
});
