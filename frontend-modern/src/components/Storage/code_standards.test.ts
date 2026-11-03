import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const storageDir = path.dirname(fileURLToPath(import.meta.url));

const STORAGE_SHELL_FILES = ['Storage.tsx'] as const;
const FORBIDDEN_STORAGE_HELPERS = [
  'isCephType',
  'getCephHealthLabel',
  'getCephHealthStyles',
] as const;

const findInlineDefinitionLine = (source: string, symbol: string): number | null => {
  const patterns = [
    new RegExp(`^\\s*(?:export\\s+)?function\\s+${symbol}\\b`, 'm'),
    new RegExp(`^\\s*(?:export\\s+)?(?:const|let|var)\\s+${symbol}\\b`, 'm'),
  ];

  for (const pattern of patterns) {
    const match = pattern.exec(source);
    if (!match || match.index < 0) continue;
    return source.slice(0, match.index).split('\n').length;
  }

  return null;
};

describe('Storage code standards guardrails', () => {
  it('keeps Ceph helper definitions in storageDomain.ts and out of storage shells', () => {
    const violations: string[] = [];

    for (const fileName of STORAGE_SHELL_FILES) {
      const filePath = path.resolve(storageDir, fileName);
      const source = readFileSync(filePath, 'utf-8');

      for (const helper of FORBIDDEN_STORAGE_HELPERS) {
        const line = findInlineDefinitionLine(source, helper);
        if (line === null) continue;
        violations.push(
          `${fileName}:${line} inline ${helper} definition detected; define it in src/features/storageBackups/storageDomain.ts`,
        );
      }
    }

    expect(violations).toEqual([]);
  });
});
