import { describe, expect, it } from 'vitest';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import typeColumnDefinitionSource from '@/utils/typeColumnDefinition.ts?raw';

const tsxSources = import.meta.glob('../../**/*.tsx', {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

const INLINE_TYPE_COLUMN_PATTERN = /\{\s*id:\s*'type',\s*label:\s*'Type'[\s\S]*?\}/g;

describe('type column guardrails', () => {
  it('keeps the canonical Type column definition in the shared helper', () => {
    expect(typeColumnDefinitionSource).toContain("id: 'type'");
    expect(typeColumnDefinitionSource).toContain("label: 'Type'");
    expect(typeColumnDefinitionSource).toContain('toggleable: true');
  });

  it('routes runtime Type columns through the shared helper', () => {
    expect(guestRowSource).toContain('createCanonicalTypeColumn');
    expect(guestRowSource).not.toMatch(INLINE_TYPE_COLUMN_PATTERN);

    expect(recoverySource).toContain('createCanonicalTypeColumn');
    expect(recoverySource).not.toMatch(INLINE_TYPE_COLUMN_PATTERN);
  });

  it('limits runtime Type columns to the known allowlist', () => {
    const runtimeEntries = Object.entries(tsxSources).filter(
      ([path]) => !path.endsWith('.test.tsx'),
    );

    const typeColumnUsers = runtimeEntries
      .filter(([, source]) => {
        return source.includes('createCanonicalTypeColumn(');
      })
      .map(([path]) => path)
      .sort();

    expect(typeColumnUsers).toEqual([
      '../Dashboard/GuestRow.tsx',
      '../Recovery/Recovery.tsx',
    ]);
  });
});
