import { describe, expect, it } from 'vitest';
import pageControlsSource from '@/components/shared/PageControls.tsx?raw';
import filterToolbarSource from '@/components/shared/FilterToolbar.tsx?raw';
import workloadsFilterSource from '@/components/Workloads/WorkloadsFilter.tsx?raw';
import storageFilterSource from '@/components/Storage/StorageFilter.tsx?raw';
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
    expect(pageControlsSource).toContain('searchTrailing?: JSX.Element;');
    expect(pageControlsSource).toContain(
      'const mobileControlsEnabled = () => local.mobileFilters?.enabled === true;',
    );
    expect(pageControlsSource).toContain(
      'const activeMobileTrailing = () => (mobileControlsEnabled() ? local.mobileTrailing : undefined);',
    );
    expect(pageControlsSource).toContain(
      'const activeUtilityActions = () => (mobileControlsEnabled() ? undefined : local.utilityActions);',
    );
    expect(pageControlsSource).toContain(
      'const activeSearchTrailing = () => (mobileControlsEnabled() ? undefined : local.searchTrailing);',
    );
    expect(pageControlsSource).toContain(
      'searchAccessory={activeSearchTrailing() ?? mobileSearchAccessory()}',
    );

    expect(workloadsFilterSource).toContain('PageControls');
    expect(workloadsFilterSource).not.toContain('<FilterHeader');
    expect(workloadsFilterSource).not.toContain('<ColumnPicker');

    expect(storageFilterSource).toContain('PageControls');
    expect(storageFilterSource).not.toContain('<FilterHeader');
    expect(storageFilterSource).not.toContain('<ColumnPicker');

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

    expect(filterHeaderUsers).toEqual(['./PageControls.tsx']);

    expect(sharedToolbarImportUsers).toEqual(['./PageControls.tsx']);

    expect(columnPickerUsers).toEqual(['./PageControls.tsx']);

    expect(columnPickerImportUsers).toEqual(['./PageControls.tsx']);

    expect(mobileToggleUsers).toEqual(['./PageControls.tsx']);
  });

  it('keeps search-row leading content routed through PageControls rather than local FilterHeader forks', () => {
    expect(pageControlsSource).toContain('searchLeading?: JSX.Element');
    expect(pageControlsSource).toContain('searchLeading={local.searchLeading}');
    expect(pageControlsSource).toContain('{...divProps}');
    expect(recoveryProtectedInventorySectionSource).not.toContain('<FilterHeader');
    expect(recoveryHistorySectionSource).not.toContain('<FilterHeader');
    expect(recoveryProtectedInventorySectionSource).not.toContain('searchRowClass=');
    expect(recoveryHistorySectionSource).not.toContain('searchRowClass=');
    expect(recoveryProtectedInventorySectionSource).not.toContain('!w-auto');
    expect(recoveryHistorySectionSource).not.toContain('!w-auto');
  });

  it('keeps display controls and utility actions on the shared toolbar rail', () => {
    expect(workloadsFilterSource).toContain('toolbarTrailing={');
    expect(storageFilterSource).toContain('toolbarTrailing={');
    expect(workloadsFilterSource).toContain('ChartVisibilityToggleButton');
    expect(storageFilterSource).toContain('ChartVisibilityToggleButton');
    expect(infrastructurePageSurfaceSource).toContain('ChartVisibilityToggleButton');
    expect(workloadsFilterSource).not.toContain('aria-label="Charts"');
    expect(storageFilterSource).not.toContain('aria-label="Charts"');
    expect(infrastructurePageSurfaceSource).not.toContain('aria-label="Charts"');
    expect(pageControlsSource).toContain('page-controls-filter-controls');
    expect(pageControlsSource).toContain('page-controls-toolbar-actions ml-auto');
    expect(pageControlsSource).toContain('pageControlsControlDeckClass');
    expect(pageControlsSource).toContain('pageControlsFilterSectionClass');
    expect(pageControlsSource).toContain('pageControlsSectionedFilterControlsClass');
    expect(pageControlsSource).toContain('filterControlsVariant?:');
    expect(pageControlsSource).toContain('actionsLayout?:');
    expect(pageControlsSource).toContain('controlDeckClass?:');
    expect(pageControlsSource).toContain('const toolbarControls = () => (');
    expect(pageControlsSource).toContain("actionsLayout() === 'stacked'");
    expect(pageControlsSource).toContain("local.actionsLayout ?? 'stacked'");
    expect(pageControlsSource).toContain(
      'shrink-0 flex-wrap items-center justify-end gap-2 self-start',
    );
    expect(pageControlsSource).not.toContain('2xl:ml-auto');
    expect(recoveryHistorySectionSource).not.toContain('toolbarClass="lg:flex-nowrap"');
    expect(recoveryHistorySectionSource).not.toContain('ml-auto flex items-center gap-2');
  });

  it('keeps compact stable filters on the shared labeled toggle primitive', () => {
    expect(filterToolbarSource).toContain('export const LabeledFilterToggleGroup');
    expect(filterToolbarSource).toContain('COMPACT_FILTER_TOGGLE_MAX_OPTIONS = 5');
    expect(filterToolbarSource).toContain("class={local.toggleClass ?? 'hidden xl:inline-flex'}");
    expect(filterToolbarSource).toContain("groupClass={local.selectGroupClass ?? 'xl:hidden'}");
    expect(workloadsFilterSource).toContain('LabeledFilterToggleGroup');
    expect(workloadsFilterSource).toContain('WORKLOAD_TYPE_OPTIONS');
    expect(workloadsFilterSource).toContain('WORKLOAD_STATUS_FILTER_OPTIONS');
    expect(workloadsFilterSource).toContain('workloads-filter-primary-controls');
    expect(workloadsFilterSource).toContain('workloads-filter-secondary-controls');
    expect(workloadsFilterSource).toContain('pageControlsFilterSectionClass');
    expect(workloadsFilterSource).toContain('filterControlsVariant="sectioned-children"');
    expect(workloadsFilterSource).not.toContain('workloads-filter-control-deck');
    expect(workloadsFilterSource).not.toContain('controlDeckClass=');
    expect(pageControlsSource).toContain('border border-border bg-surface-alt');
    expect(pageControlsSource).toContain('border border-border-subtle bg-surface');
    expect(pageControlsSource).toContain('xl:grid-cols-[minmax(0,1fr)_auto]');
    expect(workloadsFilterSource).not.toContain('xl:grid-cols-[max-content_minmax(16rem,1fr)]');
    expect(workloadsFilterSource).not.toContain('actionsLayout="stacked"');
    expect(workloadsFilterSource).not.toContain('page-controls-toolbar-actions inline-flex');
    expect(workloadsFilterSource).toContain('xl:flex-col xl:items-start');
    expect(workloadsFilterSource).not.toContain(
      '<LabeledFilterSelect\n          id="workloads-type-filter"',
    );
    expect(workloadsFilterSource).not.toContain(
      '<LabeledFilterSelect\n          id="workloads-status-filter"',
    );
    expect(recoveryHistorySectionSource).toContain('LabeledFilterToggleGroup');
    expect(recoveryHistorySectionSource).not.toContain(
      '<LabeledFilterSelect\n                id="recovery-status-filter"',
    );
    expect(storageFilterSource).toContain(
      '<LabeledFilterSelect\n          id="storage-status-filter"',
    );
    expect(infrastructurePageSurfaceSource).toContain(
      '<LabeledFilterSelect\n                          id="infra-status-filter"',
    );
  });

  it('keeps embedded workspace tabs on the canonical shared subtabs class pattern', () => {
    expect(subtabsSource).not.toContain("variant?: 'default' | 'control'");
    expect(recoveryPageSource).not.toContain('variant="control"');
    expect(recoveryPageSource).not.toContain('listClass=');
    expect(recoveryPageSource).not.toContain('tabClass=');
    expect(recoveryPageSource).toContain('const workspaceControls = () => (');
    expect(recoveryPageSource).toContain('<Subtabs');
    expect(recoveryPageSource).not.toContain('Focused drill-in');
  });
});
