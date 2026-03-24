import { describe, expect, it } from 'vitest';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import problemResourcesTableSource from '@/features/dashboardOverview/ProblemResourcesTable.tsx?raw';
import { TYPE_COLUMN_LABEL } from '@/utils/typeColumnContract';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

const sourceFiles = import.meta.glob(['../../**/*.ts', '../../**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('typeColumnPresentation', () => {
  it('returns the canonical Type column label', () => {
    expect(getTypeColumnLabel()).toBe(TYPE_COLUMN_LABEL);
  });

  it('keeps fixed runtime Type headers on the shared label utility', () => {
    expect(problemResourcesTableSource).toContain('getTypeColumnLabel()');
    expect(alertHistoryTableSectionSource).toContain('getTypeColumnLabel()');
  });

  it('limits runtime Type header label helper imports to the known allowlist', () => {
    const runtimeEntries = Object.entries(sourceFiles).filter(
      ([path]) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx'),
    );
    const typeColumnPresentationImportPattern =
      /from\s*['"]@\/utils\/typeColumnPresentation['"]/;

    const directTypeLabelHelperUsers = runtimeEntries
      .filter(([, source]) => typeColumnPresentationImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(directTypeLabelHelperUsers).toEqual([
      '../../features/alerts/AlertHistoryTableSection.tsx',
      '../../features/dashboardOverview/ProblemResourcesTable.tsx',
    ]);
  });
});
