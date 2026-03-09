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
const RESPONSIVE_TYPE_BLOCK_PATTERN = /type:\s*\{[\s\S]*?\n\s*\},/;

describe('type column guardrails', () => {
  it('keeps the canonical Type column definition in the shared helper', () => {
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_ID = 'type'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_LABEL = 'Type'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_SORT_KEY = 'type'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_SORTABLE = true");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_PRIORITY = 'essential'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_WIDTH = '60px'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_MIN_WIDTH = '60px'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_MAX_WIDTH = '80px'");
    expect(typeColumnContractSource).toContain("TYPE_COLUMN_ALIGN = 'center'");
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_ID');
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_LABEL');
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_SORT_KEY');
    expect(typeColumnDefinitionSource).toContain('TYPE_COLUMN_WIDTH');
    expect(typeColumnDefinitionSource).toContain('toggleable: true');
    expect(typeColumnDefinitionSource).not.toContain('export const createCanonicalTypeColumn');
    expect(typeColumnDefinitionSource).not.toContain("id: 'type'");
    expect(typeColumnDefinitionSource).not.toContain("label: 'Type'");
    expect(typeColumnDefinitionSource).not.toContain("width: '60px'");
    expect(typeColumnDefinitionSource).not.toContain("sortKey: 'type'");
    expect(responsiveSource).toContain('TYPE_COLUMN_ID');
    expect(responsiveSource).toContain('TYPE_COLUMN_LABEL');
    expect(responsiveSource).toContain('TYPE_COLUMN_PRIORITY');
    expect(responsiveSource).toContain('TYPE_COLUMN_SORTABLE');
    expect(responsiveSource).toContain('TYPE_COLUMN_MIN_WIDTH');
    expect(responsiveSource).toContain('TYPE_COLUMN_MAX_WIDTH');
    expect(responsiveSource).toContain('TYPE_COLUMN_ALIGN');
    const responsiveTypeBlock = responsiveSource.match(RESPONSIVE_TYPE_BLOCK_PATTERN)?.[0] ?? '';
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_ID');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_LABEL');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_PRIORITY');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_SORTABLE');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_MIN_WIDTH');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_MAX_WIDTH');
    expect(responsiveTypeBlock).toContain('TYPE_COLUMN_ALIGN');
    expect(responsiveTypeBlock).not.toContain("id: 'type'");
    expect(responsiveTypeBlock).not.toContain("label: 'Type'");
    expect(responsiveTypeBlock).not.toContain("priority: 'essential'");
    expect(responsiveTypeBlock).not.toContain('sortable: true');
    expect(responsiveTypeBlock).not.toContain("minWidth: '60px'");
    expect(responsiveTypeBlock).not.toContain("maxWidth: '80px'");
  });

  it('routes runtime Type columns through the shared helper', () => {
    expect(guestRowSource).toContain('createVisibleCanonicalTypeColumn()');
    expect(guestRowSource).not.toContain('createCanonicalTypeColumn');
    expect(guestRowSource).not.toMatch(INLINE_TYPE_COLUMN_PATTERN);
    expect(guestRowSource).not.toContain("defaultVisibility:");

    expect(recoverySource).toContain('createHiddenCanonicalTypeColumn()');
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

  it('limits type column helper imports to the known runtime tables', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const typeColumnDefinitionImportPattern =
      /from\s*['"]@\/utils\/typeColumnDefinition['"]/;

    const directHelperImportUsers = runtimeEntries
      .filter(([, source]) => typeColumnDefinitionImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(directHelperImportUsers).toEqual([
      '../Dashboard/GuestRow.tsx',
      '../Recovery/Recovery.tsx',
    ]);
  });

  it('limits direct type column contract imports to the shared helper and responsive schema', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const typeColumnContractImportPattern =
      /from\s*['"]@\/utils\/typeColumnContract['"]/;

    const directContractImportUsers = runtimeEntries
      .filter(([, source]) => typeColumnContractImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(directContractImportUsers).toEqual([
      '../../types/responsive.ts',
      '../../utils/typeColumnDefinition.ts',
    ]);
  });
});
