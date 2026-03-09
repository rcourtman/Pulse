import { describe, expect, it } from 'vitest';
import pageControlsSource from '@/components/shared/PageControls.tsx?raw';
import dashboardFilterSource from '@/components/Dashboard/DashboardFilter.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import infrastructureSource from '@/pages/Infrastructure.tsx?raw';

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

    expect(dashboardFilterSource).toContain('PageControls');
    expect(dashboardFilterSource).not.toContain('<FilterHeader');
    expect(dashboardFilterSource).not.toContain('<ColumnPicker');

    expect(recoverySource).toContain('PageControls');
    expect(recoverySource).not.toContain('<FilterHeader');
    expect(recoverySource).not.toContain('<ColumnPicker');

    expect(infrastructureSource).toContain('PageControls');
    expect(infrastructureSource).not.toContain('<FilterHeader');
  });

  it('limits raw FilterHeader and ColumnPicker usage to the known allowlist', () => {
    const runtimeEntries = Object.entries(tsxSources).filter(
      ([path]) => !path.endsWith('.test.tsx'),
    );
    const filterHeaderTagPattern = /<FilterHeader(?:\s|>)/;
    const columnPickerTagPattern = /<ColumnPicker(?:\s|>)/;

    const filterHeaderUsers = runtimeEntries
      .filter(([, source]) => filterHeaderTagPattern.test(source))
      .map(([path]) => path)
      .sort();
    const columnPickerUsers = runtimeEntries
      .filter(([, source]) => columnPickerTagPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(filterHeaderUsers).toEqual([
      '../../pages/Alerts.tsx',
      '../Storage/StorageFilter.tsx',
      './PageControls.tsx',
    ]);

    expect(columnPickerUsers).toEqual([
      '../Storage/StorageFilter.tsx',
      './PageControls.tsx',
    ]);
  });
});
