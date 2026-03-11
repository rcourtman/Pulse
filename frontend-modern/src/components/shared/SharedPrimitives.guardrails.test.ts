import { describe, expect, it } from 'vitest';
import filterButtonGroupSource from '@/components/shared/FilterButtonGroup.tsx?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';

const sharedSources = import.meta.glob(['./*.tsx', './cards/*.tsx', './responsive/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('shared primitive guardrails', () => {
  it('limits raw Table composition inside shared primitives to the canonical allowlist', () => {
    const sharedRuntimeEntries = Object.entries(sharedSources).filter(
      ([path]) => !path.endsWith('.test.tsx') && !path.endsWith('.guardrails.test.ts'),
    );
    const tableImportPattern = /from\s*['"]@\/components\/shared\/Table['"]/;

    const rawTableUsers = sharedRuntimeEntries
      .filter(([, source]) => tableImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(rawTableUsers).toEqual([
      './InfrastructureSummaryTable.tsx',
      './PulseDataGrid.tsx',
    ]);
  });

  it('routes canonical settings segmented selectors through FilterButtonGroup', () => {
    expect(filterButtonGroupSource).toContain("variant?: 'default' | 'settings'");
    expect(generalSettingsPanelSource).toContain('FilterButtonGroup');
    expect(generalSettingsPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(2);
    expect(generalSettingsPanelSource).not.toContain("props.themePreference() === 'light'");
    expect(generalSettingsPanelSource).not.toContain("temperatureStore.unit() === 'celsius'");
  });
});
