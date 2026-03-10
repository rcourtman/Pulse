import { describe, expect, it } from 'vitest';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
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
    expect(alertsPageSource).toContain('getTypeColumnLabel()');
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
      '../../pages/Alerts.tsx',
      '../../pages/DashboardPanels/ProblemResourcesTable.tsx',
    ]);
  });
});
