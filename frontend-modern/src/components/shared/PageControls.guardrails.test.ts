import { describe, expect, it } from 'vitest';
import pageControlsSource from '@/components/shared/PageControls.tsx?raw';
import dashboardFilterSource from '@/components/Dashboard/DashboardFilter.tsx?raw';
import recoveryPageSource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';
import infrastructurePageSurfaceSource from '@/features/infrastructure/InfrastructurePageSurface.tsx?raw';
import subtabsSource from '@/components/shared/Subtabs.tsx?raw';

const tsxSources = import.meta.glob('../../**/*.tsx', {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('page controls guardrails', () => {
  it('keeps canonical page-level controls routed through PageControls', () => {
    expect(pageControlsSource).toContain('FilterHeader');
    expect(pageControlsSource).toContain('FilterMobileToggleButton');
    expect(pageControlsSource).toContain('ColumnPicker');
    expect(pageControlsSource).toContain('searchLeading');
    expect(pageControlsSource).toContain('splitProps');
    expect(pageControlsSource).toContain('<FilterHeader');
    expect(pageControlsSource).toContain('{...divProps}');

    expect(dashboardFilterSource).toContain('PageControls');
    expect(dashboardFilterSource).not.toContain('<FilterHeader');
    expect(dashboardFilterSource).not.toContain('<ColumnPicker');

    expect(recoveryProtectedInventorySectionSource).toContain('PageControls');
    expect(recoveryProtectedInventorySectionSource).not.toContain('<FilterHeader');
    expect(recoveryProtectedInventorySectionSource).not.toContain('<ColumnPicker');

    expect(recoveryHistorySectionSource).toContain('PageControls');
    expect(recoveryHistorySectionSource).not.toContain('<FilterHeader');
    expect(recoveryHistorySectionSource).not.toContain('<ColumnPicker');

    expect(infrastructurePageSurfaceSource).toContain('PageControls');
    expect(infrastructurePageSurfaceSource).not.toContain('<FilterHeader');
  });

  it('limits raw FilterHeader and ColumnPicker usage to the known allowlist', () => {
    const runtimeEntries = Object.entries(tsxSources).filter(
      ([path]) => !path.endsWith('.test.tsx'),
    );
    const sharedToolbarImportPattern =
      /import\s*\{[\s\S]*\b(FilterHeader|FilterMobileToggleButton)\b[\s\S]*\}\s*from\s*['"]@\/components\/shared\/FilterToolbar['"]/;
    const columnPickerImportPattern =
      /import\s*\{[\s\S]*\bColumnPicker\b[\s\S]*\}\s*from\s*['"]@\/components\/shared\/ColumnPicker['"]/;
    const filterHeaderTagPattern = /<FilterHeader(?:\s|>)/;
    const columnPickerTagPattern = /<ColumnPicker(?:\s|>)/;
    const mobileToggleTagPattern = /<FilterMobileToggleButton(?:\s|>)/;

    const sharedToolbarImportUsers = runtimeEntries
      .filter(([, source]) => sharedToolbarImportPattern.test(source))
      .map(([path]) => path)
      .sort();
    const columnPickerImportUsers = runtimeEntries
      .filter(([, source]) => columnPickerImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    const filterHeaderUsers = runtimeEntries
      .filter(([, source]) => filterHeaderTagPattern.test(source))
      .map(([path]) => path)
      .sort();
    const columnPickerUsers = runtimeEntries
      .filter(([, source]) => columnPickerTagPattern.test(source))
      .map(([path]) => path)
      .sort();
    const mobileToggleUsers = runtimeEntries
      .filter(([, source]) => mobileToggleTagPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(filterHeaderUsers).toEqual([
      './PageControls.tsx',
    ]);

    expect(sharedToolbarImportUsers).toEqual([
      './PageControls.tsx',
    ]);

    expect(columnPickerUsers).toEqual([
      './PageControls.tsx',
    ]);

    expect(columnPickerImportUsers).toEqual([
      './PageControls.tsx',
    ]);

    expect(mobileToggleUsers).toEqual([
      './PageControls.tsx',
    ]);
  });

  it('keeps search-row leading content routed through PageControls rather than local FilterHeader forks', () => {
    expect(pageControlsSource).toContain('searchLeading?: JSX.Element');
    expect(pageControlsSource).toContain('searchLeading={local.searchLeading}');
    expect(pageControlsSource).toContain('{...divProps}');
    expect(recoveryProtectedInventorySectionSource).not.toContain('<FilterHeader');
    expect(recoveryHistorySectionSource).not.toContain('<FilterHeader');
  });

  it('keeps embedded workspace tabs on the shared subtabs control variant', () => {
    expect(subtabsSource).toContain("variant?: 'default' | 'control'");
    expect(subtabsSource).toContain('subtabsControlShellClass');
    expect(subtabsSource).toContain('subtabsControlListClass');
    expect(recoveryPageSource).toContain('variant="control"');
    expect(recoveryPageSource).not.toContain('listClass="gap-1 overflow-x-auto scrollbar-hide"');
    expect(recoveryPageSource).not.toContain(
      'tabClass="min-h-8 whitespace-nowrap rounded-md border border-transparent px-3 py-1.5 text-xs"',
    );
  });
});
