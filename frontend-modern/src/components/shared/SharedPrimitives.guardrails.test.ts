import { describe, expect, it } from 'vitest';
import calloutCardSource from '@/components/shared/CalloutCard.tsx?raw';
import filterButtonGroupSource from '@/components/shared/FilterButtonGroup.tsx?raw';
import interactiveSparklineSource from '@/components/shared/InteractiveSparkline.tsx?raw';
import interactiveSparklineModelSource from '@/components/shared/interactiveSparklineModel.ts?raw';
import infrastructureSummaryTableSource from '@/components/shared/InfrastructureSummaryTable.tsx?raw';
import infrastructureSummaryTableRowSource from '@/components/shared/InfrastructureSummaryTableRow.tsx?raw';
import infrastructureSummaryTableModelSource from '@/components/shared/infrastructureSummaryTableModel.ts?raw';
import infrastructureSummaryTableStateSource from '@/components/shared/useInfrastructureSummaryTableState.ts?raw';
import monitoredSystemLimitWarningBannerSource from '@/components/shared/MonitoredSystemLimitWarningBanner.tsx?raw';
import selectionCardGroupSource from '@/components/shared/SelectionCardGroup.tsx?raw';
import tagBadgesSource from '@/components/shared/TagBadges.tsx?raw';
import interactiveSparklineStateSource from '@/components/shared/useInteractiveSparklineState.ts?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import aiSettingsDialogsSource from '@/components/Settings/AISettingsDialogs.tsx?raw';
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
      './InfrastructureSummaryTableRow.tsx',
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
    expect(aiSettingsDialogsSource).toContain('SelectionCardGroup');
    expect(aiSettingsDialogsSource).toContain('variant="compact"');
    expect(aiSettingsDialogsSource).not.toContain(
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

  it('keeps shared fleet limit banner copy on the monitored-system commercial term', () => {
    expect(monitoredSystemLimitWarningBannerSource).toContain('Monitored systems:');
    expect(monitoredSystemLimitWarningBannerSource).toContain('monitored-system cap');
    expect(monitoredSystemLimitWarningBannerSource).toContain('Install v6 collectors');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('v6 Unified Agents:');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain(
      'do not count toward Unified Agents.',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain(
      'Install v6 Unified Agents',
    );
  });

  it('keeps shared tag badges in the shared primitive boundary', () => {
    expect(tagBadgesSource).toContain("from '@/components/shared/Tooltip'");
    expect(guestRowSource).toContain("from '@/components/shared/TagBadges'");
    expect(guestRowSource).not.toContain("from './TagBadges'");
    expect(resourceDetailDrawerOverviewSource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      "from '@/components/Dashboard/TagBadges'",
    );
  });

  it('keeps infrastructure summary table on shell, runtime, and model owners', () => {
    expect(infrastructureSummaryTableSource).toContain('useInfrastructureSummaryTableState');
    expect(infrastructureSummaryTableSource).toContain('InfrastructureSummaryTableRow');
    expect(infrastructureSummaryTableSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableSource).not.toContain('useAlertsActivation');
    expect(infrastructureSummaryTableSource).not.toContain('createSignal');
    expect(infrastructureSummaryTableSource).not.toContain('getNormalizedIdentityLookupVariants');
    expect(infrastructureSummaryTableSource).not.toContain('getAgentLikeIdentityAliases');

    expect(infrastructureSummaryTableStateSource).toContain('useWebSocket');
    expect(infrastructureSummaryTableStateSource).toContain('useAlertsActivation');
    expect(infrastructureSummaryTableStateSource).toContain('export function useInfrastructureSummaryTableState');
    expect(infrastructureSummaryTableStateSource).toContain('createSignal');

    expect(infrastructureSummaryTableModelSource).toContain('getNormalizedIdentityLookupVariants');
    expect(infrastructureSummaryTableModelSource).toContain('getAgentLikeIdentityAliases');
    expect(infrastructureSummaryTableModelSource).toContain(
      'export const resolveInfrastructureSummaryLinkedAgent',
    );
    expect(infrastructureSummaryTableModelSource).toContain(
      'export const sortInfrastructureSummaryItems',
    );

    expect(infrastructureSummaryTableRowSource).toContain('InfrastructureDetailsDrawer');
    expect(infrastructureSummaryTableRowSource).toContain('getAlertStyles');
    expect(infrastructureSummaryTableRowSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableRowSource).not.toContain('useAlertsActivation');
  });

  it('keeps interactive sparkline on shell, runtime, and model owners', () => {
    expect(interactiveSparklineSource).toContain('useInteractiveSparklineState');
    expect(interactiveSparklineSource).not.toContain('createEffect');
    expect(interactiveSparklineSource).not.toContain('createSignal');
    expect(interactiveSparklineSource).not.toContain('scheduleSparkline');
    expect(interactiveSparklineSource).not.toContain('downsampleLTTB');

    expect(interactiveSparklineStateSource).toContain('export function useInteractiveSparklineState');
    expect(interactiveSparklineStateSource).toContain('createSignal');
    expect(interactiveSparklineStateSource).toContain('scheduleSparkline');
    expect(interactiveSparklineStateSource).toContain('computeInteractiveSparklineHoverState');

    expect(interactiveSparklineModelSource).toContain('buildInteractiveSparklineChartData');
    expect(interactiveSparklineModelSource).toContain('computeInteractiveSparklineHoverState');
    expect(interactiveSparklineModelSource).toContain('downsampleLTTB');
    expect(interactiveSparklineModelSource).toContain('findNearestMetricPoint');
  });
});
