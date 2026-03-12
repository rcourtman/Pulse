import { describe, expect, it } from 'vitest';
import calloutCardSource from '@/components/shared/CalloutCard.tsx?raw';
import filterButtonGroupSource from '@/components/shared/FilterButtonGroup.tsx?raw';
import selectionCardGroupSource from '@/components/shared/SelectionCardGroup.tsx?raw';
import aiSettingsSource from '@/components/Settings/AISettings.tsx?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import proxmoxSettingsPanelSource from '@/components/Settings/ProxmoxSettingsPanel.tsx?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';

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
    expect(filterButtonGroupSource).toContain("variant?: FilterButtonGroupVariant");
    expect(filterButtonGroupSource).toContain("prominent: 'grid grid-cols-1 gap-2'");
    expect(filterButtonGroupSource).toContain('touch-scroll');
    expect(filterButtonGroupSource).not.toContain('-webkit-overflow-scrolling: touch;');
    expect(generalSettingsPanelSource).toContain('FilterButtonGroup');
    expect(generalSettingsPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(3);
    expect(generalSettingsPanelSource).toContain('variant="prominent"');
    expect(generalSettingsPanelSource).not.toContain("props.themePreference() === 'light'");
    expect(generalSettingsPanelSource).not.toContain("temperatureStore.unit() === 'celsius'");
    expect(generalSettingsPanelSource).not.toContain("props.pvePollingSelection() === option.value");
    expect(reportingPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(2);
    expect(reportingPanelSource).toContain('variant="prominent"');
    expect(reportingPanelSource).not.toContain('getReportingToggleButtonClass');
    expect(reportingPanelSource).not.toContain("<For each={REPORTING_RANGE_OPTIONS}>");
  });

  it('routes selectable settings cards through SelectionCardGroup', () => {
    expect(selectionCardGroupSource).toContain(
      "type SelectionCardGroupVariant = 'compact' | 'detail'",
    );
    expect(aiSettingsSource).toContain('SelectionCardGroup');
    expect(aiSettingsSource).toContain('variant="compact"');
    expect(aiSettingsSource).not.toContain(
      'class={`p-3 rounded-md border-2 transition-all text-center',
    );
    expect(updatesSettingsPanelSource).toContain('SelectionCardGroup');
    expect(updatesSettingsPanelSource).toContain('variant="detail"');
    expect(updatesSettingsPanelSource).not.toContain(
      'class={`p-4 rounded-md border-2 transition-all text-left',
    );
  });

  it('routes settings info callouts through CalloutCard', () => {
    expect(calloutCardSource).toContain(
      "type CalloutTone = 'danger' | 'info' | 'success' | 'warning'",
    );
    expect(proxmoxSettingsPanelSource).toContain('CalloutCard');
    expect(proxmoxSettingsPanelSource).not.toContain(
      'rounded-md border border-blue-200 bg-blue-50 px-4 py-3',
    );
    expect(reportingPanelSource).toContain('CalloutCard');
    expect(reportingPanelSource).not.toContain(
      'rounded-md border border-blue-200 bg-blue-50 p-6',
    );
  });
});
