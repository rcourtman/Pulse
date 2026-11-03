import { describe, expect, it } from 'vitest';

const sourceFiles = import.meta.glob(['../../**/*.ts', '../../**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

const INLINE_TYPE_COLUMN_PATTERN =
  /\{\s*id:\s*'type',\s*label:\s*'Type'[\s\S]*?toggleable:\s*true[\s\S]*?\}/g;

describe('type column guardrails', () => {
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

    expect(typeColumnUsers).toEqual(['../Workloads/guestRowModel.tsx']);

    expect(inlineTypeColumnUsers).toEqual([]);
  });

  it('keeps type default-visibility policy out of page-level hidden-column arrays', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const hiddenTypeArrayPattern = /useColumnVisibility\([\s\S]*?\[\s*['"]type['"][\s\S]*?\]/;

    const pageLevelTypeDefaultHiddenUsers = runtimeEntries
      .filter(([, source]) => hiddenTypeArrayPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(pageLevelTypeDefaultHiddenUsers).toEqual([]);
  });

  it('limits type column helper imports to the known runtime tables', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const typeColumnDefinitionImportPattern = /from\s*['"]@\/utils\/typeColumnDefinition['"]/;

    const directHelperImportUsers = runtimeEntries
      .filter(([, source]) => typeColumnDefinitionImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(directHelperImportUsers).toEqual(['../Workloads/guestRowModel.tsx']);
  });

  it('limits direct type column contract imports to the shared helper and label presentation util', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const typeColumnContractImportPattern = /from\s*['"]@\/utils\/typeColumnContract['"]/;

    const directContractImportUsers = runtimeEntries
      .filter(([, source]) => typeColumnContractImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(directContractImportUsers).toEqual([
      '../../utils/typeColumnDefinition.ts',
      '../../utils/typeColumnPresentation.ts',
    ]);
  });
});
