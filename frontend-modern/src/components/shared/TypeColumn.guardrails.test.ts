import { describe, expect, it } from 'vitest';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import responsiveSource from '@/types/responsive.ts?raw';
import typeColumnDefinitionSource from '@/utils/typeColumnDefinition.ts?raw';
import typeColumnContractSource from '@/utils/typeColumnContract.ts?raw';

const sourceFiles = import.meta.glob(['../../**/*.ts', '../../**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

const INLINE_TYPE_COLUMN_PATTERN =
  /\{\s*id:\s*'type',\s*label:\s*'Type'[\s\S]*?toggleable:\s*true[\s\S]*?\}/g;

describe('type column guardrails', () => {
  it('keeps the canonical Type column definition in the shared helper', () => {
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_ID = 'type'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_LABEL = 'Type'");
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_ID');
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_LABEL');
    expect(typeColumnDefinitionSource).toContain('toggleable: true');
    expect(typeColumnDefinitionSource).not.toContain('export const createCanonicalTypeColumn');
    expect(responsiveSource).toContain('TYPE_COLUMN_ID');
    expect(responsiveSource).toContain('TYPE_COLUMN_LABEL');
  });

  it('routes runtime Type columns through the shared helper', () => {
    expect(guestRowSource).toContain('createVisibleCanonicalTypeColumn');
    expect(guestRowSource).not.toContain('createCanonicalTypeColumn');
    expect(guestRowSource).not.toMatch(INLINE_TYPE_COLUMN_PATTERN);
    expect(guestRowSource).not.toContain("defaultVisibility:");

    expect(recoverySource).toContain('createHiddenCanonicalTypeColumn');
    expect(recoverySource).not.toContain('createCanonicalTypeColumn');
    expect(recoverySource).not.toMatch(INLINE_TYPE_COLUMN_PATTERN);
    expect(recoverySource).not.toContain("defaultVisibility:");
  });

  it('limits runtime Type columns to the known allowlist', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );

    const typeColumnUsers = runtimeEntries
      .filter(([, source]) => {
        return (
          source.includes('createVisibleCanonicalTypeColumn(') ||
          source.includes('createHiddenCanonicalTypeColumn(')
        );
      })
      .map(([path]) => path)
      .sort();

    const inlineTypeColumnUsers = runtimeEntries
      .filter(([, source]) => INLINE_TYPE_COLUMN_PATTERN.test(source))
      .map(([path]) => path)
      .sort();

    expect(typeColumnUsers).toEqual([
      '../Dashboard/GuestRow.tsx',
      '../Recovery/Recovery.tsx',
    ]);

    expect(inlineTypeColumnUsers).toEqual([]);
  });

  it('keeps type default-visibility policy out of page-level hidden-column arrays', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const hiddenTypeArrayPattern =
      /useColumnVisibility\([\s\S]*?\[\s*['"]type['"][\s\S]*?\]/;

    const pageLevelTypeDefaultHiddenUsers = runtimeEntries
      .filter(([, source]) => hiddenTypeArrayPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(pageLevelTypeDefaultHiddenUsers).toEqual([]);
  });
});
