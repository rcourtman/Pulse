import { describe, expect, it } from 'vitest';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import alertsConfigurationSurfaceSource from '@/features/alerts/AlertsConfigurationSurface.tsx?raw';
import alertsConfigurationStateSource from '@/features/alerts/useAlertsConfigurationState.ts?raw';
import alertsConfigurationSnapshotStateSource from '@/features/alerts/useAlertsConfigurationSnapshotState.ts?raw';
import alertsConfigurationModelSource from '@/features/alerts/alertsConfigurationModel.ts?raw';
import alertOverridesModelSource from '@/features/alerts/alertOverridesModel.ts?raw';
import alertOverridesStateSource from '@/features/alerts/useAlertOverridesState.ts?raw';
import alertDestinationsModelSource from '@/features/alerts/alertDestinationsModel.ts?raw';
import alertDestinationsStateSource from '@/features/alerts/useAlertDestinationsState.ts?raw';
import alertDestinationsTabStateSource from '@/features/alerts/useAlertDestinationsTabState.ts?raw';
import alertWebhookDestinationsStateSource from '@/features/alerts/useAlertWebhookDestinationsState.ts?raw';
import alertAcknowledgementStateSource from '@/features/alerts/useAlertAcknowledgementState.ts?raw';
import alertHistoryAdministrationCardSource from '@/features/alerts/AlertHistoryAdministrationCard.tsx?raw';
import alertHistoryFiltersCardSource from '@/features/alerts/AlertHistoryFiltersCard.tsx?raw';
import alertHistoryFrequencyCardSource from '@/features/alerts/AlertHistoryFrequencyCard.tsx?raw';
import alertHistoryTableAlertRowSource from '@/features/alerts/AlertHistoryTableAlertRow.tsx?raw';
import alertHistoryTableGroupRowSource from '@/features/alerts/AlertHistoryTableGroupRow.tsx?raw';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import alertResourceIncidentsPanelSource from '@/features/alerts/AlertResourceIncidentsPanel.tsx?raw';
import alertHistoryStateSource from '@/features/alerts/useAlertHistoryState.ts?raw';
import alertResourceIncidentsStateSource from '@/features/alerts/useAlertResourceIncidentsState.ts?raw';
import alertHistoryModelSource from '@/features/alerts/alertHistoryModel.ts?raw';
import alertIncidentTimelineStateSource from '@/features/alerts/useAlertIncidentTimelineState.ts?raw';
import alertOverviewActiveAlertsSectionSource from '@/features/alerts/AlertOverviewActiveAlertsSection.tsx?raw';
import alertOverviewAlertCardSource from '@/features/alerts/AlertOverviewAlertCard.tsx?raw';
import alertOverviewStatsCardsSource from '@/features/alerts/AlertOverviewStatsCards.tsx?raw';
import alertOverviewStateSource from '@/features/alerts/useAlertOverviewState.ts?raw';
import alertScheduleStateSource from '@/features/alerts/useAlertScheduleState.ts?raw';
import alertDestinationsTabSource from '@/features/alerts/tabs/DestinationsTab.tsx?raw';
import alertHistoryTabSource from '@/features/alerts/tabs/HistoryTab.tsx?raw';
import alertOverviewTabSource from '@/features/alerts/OverviewTab.tsx?raw';
import alertScheduleTabSource from '@/features/alerts/tabs/ScheduleTab.tsx?raw';
import alertThresholdsTabSource from '@/features/alerts/tabs/ThresholdsTab.tsx?raw';
import thresholdsTabModelSource from '@/features/alerts/thresholds/thresholdsTabModel.ts?raw';
import recentAlertsPanelSource from '@/components/Alerts/RecentAlertsPanel.tsx?raw';
import thresholdsTableSource from '@/components/Alerts/ThresholdsTable.tsx?raw';
import thresholdsTableAgentDisksSectionSource from '@/components/Alerts/ThresholdsTableAgentDisksSection.tsx?raw';
import thresholdsTableAgentsTabSource from '@/components/Alerts/ThresholdsTableAgentsTab.tsx?raw';
import thresholdsTableAgentsResourcesSectionSource from '@/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx?raw';
import thresholdsTableDockerContainersSectionSource from '@/components/Alerts/ThresholdsTableDockerContainersSection.tsx?raw';
import thresholdsTableDockerHostsSectionSource from '@/components/Alerts/ThresholdsTableDockerHostsSection.tsx?raw';
import thresholdsTableDockerIgnoredPrefixesSectionSource from '@/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx?raw';
import thresholdsTableDockerServiceGapSectionSource from '@/components/Alerts/ThresholdsTableDockerServiceGapSection.tsx?raw';
import thresholdsTableDockerTabSource from '@/components/Alerts/ThresholdsTableDockerTab.tsx?raw';
import thresholdsTablePMGTabSource from '@/components/Alerts/ThresholdsTablePMGTab.tsx?raw';
import thresholdsTableProxmoxBackupsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx?raw';
import thresholdsTableProxmoxGuestFilteringSectionSource from '@/components/Alerts/ThresholdsTableProxmoxGuestFilteringSection.tsx?raw';
import thresholdsTableProxmoxGuestsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxGuestsSection.tsx?raw';
import thresholdsTableProxmoxNodesSectionSource from '@/components/Alerts/ThresholdsTableProxmoxNodesSection.tsx?raw';
import thresholdsTableProxmoxPBSSectionSource from '@/components/Alerts/ThresholdsTableProxmoxPBSSection.tsx?raw';
import thresholdsTableProxmoxSnapshotsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx?raw';
import thresholdsTableProxmoxStorageSectionSource from '@/components/Alerts/ThresholdsTableProxmoxStorageSection.tsx?raw';
import thresholdsTableProxmoxTabSource from '@/components/Alerts/ThresholdsTableProxmoxTab.tsx?raw';
import thresholdsDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsData.ts?raw';
import thresholdsHostDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsHostData.ts?raw';
import thresholdsDockerDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsDockerData.ts?raw';
import thresholdsGuestDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsGuestData.ts?raw';
import thresholdsInfrastructureDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsInfrastructureData.ts?raw';
import thresholdsRecoveryDefaultsStateHookSource from '@/features/alerts/thresholds/hooks/useThresholdsRecoveryDefaultsState.ts?raw';
import thresholdsTableStateHookSource from '@/features/alerts/thresholds/hooks/useThresholdsTableState.ts?raw';
import thresholdsAvailabilityMutationsHookSource from '@/features/alerts/thresholds/hooks/useThresholdsAvailabilityMutations.ts?raw';
import thresholdsOverrideMutationsHookSource from '@/features/alerts/thresholds/hooks/useThresholdsOverrideMutations.ts?raw';
import thresholdsOverrideMutationModelSource from '@/features/alerts/thresholds/thresholdsOverrideMutationModel.ts?raw';
import thresholdsResourceModelSource from '@/features/alerts/thresholds/thresholdsResourceModel.ts?raw';
import thresholdsTableSectionPropsSource from '@/features/alerts/thresholds/thresholdsTableSectionProps.ts?raw';

import {
  ALERT_TAB_SEGMENTS,
  filterIncidentEvents,
  pathForTab,
  summarizeIncidentEvents,
  tabFromPath,
  clampCooldownMinutes,
  fallbackCooldownMinutes,
} from '@/features/alerts/types';
import {
  clampMaxAlertsPerHour,
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  fallbackMaxAlertsPerHour,
  extractTriggerValues,
  getAlertResourceDisplayLabel,
  getTriggerValue,
  normalizeEmailConfigFromAPI,
  normalizeMetricDelayMap,
  unifiedTypeToAlertDisplayType,
} from '@/features/alerts/helpers';
import {
  getAlertIncidentAcknowledgedBadgeClass,
  getAlertIncidentEventFilterActionButtonClass,
  getAlertIncidentEventFilterChipClass,
  getAlertIncidentEventFilterContainerClass,
  getAlertIncidentNoteSaveButtonClass,
  getAlertIncidentNoteTextareaClass,
  getAlertIncidentTimelineCommandClass,
  getAlertIncidentTimelineDetailClass,
  getAlertIncidentTimelineEventCardClass,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
  getAlertResourceIncidentActivityChipClass,
  getAlertResourceIncidentActivitySummaryClass,
  getAlertResourceIncidentCardClass,
  getAlertResourceIncidentSummaryRowClass,
  getAlertResourceIncidentToggleButtonClass,
  getAlertResourceIncidentTruncatedEventsLabel,
} from '@/utils/alertIncidentPresentation';
import {
  getAlertQuietSuppressCardClass,
  getAlertQuietSuppressCheckboxClass,
} from '@/utils/alertSchedulePresentation';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';

describe('normalizeMetricDelayMap', () => {
  it('returns empty object when input is nullish', () => {
    expect(normalizeMetricDelayMap(undefined)).toEqual({});
    expect(normalizeMetricDelayMap(null)).toEqual({});
  });

  it('normalizes resource and metric keys while discarding invalid values', () => {
    const input = {
      Guest: {
        CPU: 10,
        ' ': 5,
        memory: -1,
        disk: Number.NaN,
      },
      node: {
        Temperature: 30,
        disk: 15.6,
      },
      ' ': {
        metric: 5,
      },
    };

    const result = normalizeMetricDelayMap(input);

    expect(result).toEqual({
      guest: {
        cpu: 10,
      },
      node: {
        temperature: 30,
        disk: 16,
      },
    });
  });

  it('drops metric groups that normalize to empty', () => {
    const result = normalizeMetricDelayMap({
      guest: {
        cpu: -1,
        mem: Number.NaN,
      },
    });

    expect(result).toEqual({});
  });
});

describe('alert resource display labels', () => {
  it('uses the governed aiSafeSummary when policy requires redaction', () => {
    const resource = {
      id: 'resource-1',
      name: 'secret-host',
      displayName: 'Secret Host',
      type: 'agent',
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname'],
        },
      },
      aiSafeSummary: 'redacted by policy',
    } as unknown as Resource;

    expect(getAlertResourceDisplayLabel(resource)).toBe('redacted by policy');
  });

  it('falls back to the provided alert-specific fallback when needed', () => {
    const resource = {
      id: 'docker:agent-1/container-abc123',
      name: '',
      type: 'app-container',
    } as unknown as Resource;

    expect(getAlertResourceDisplayLabel(resource, 'abc123')).toBe('abc123');
  });
});

describe('tab path helpers', () => {
  it('maps tab to path', () => {
    expect(pathForTab('overview')).toBe('/alerts/overview');
    expect(pathForTab('schedule')).toBe('/alerts/schedule');
  });

  it('resolves tab from path', () => {
    expect(tabFromPath('/alerts')).toBe('overview');
    expect(tabFromPath('/alerts/thresholds')).toBe('thresholds');
    expect(tabFromPath('/alerts/thresholds/infrastructure')).toBe('thresholds');
    expect(tabFromPath('/alerts/thresholds/systems')).toBe('thresholds');
    expect(tabFromPath('/alerts/thresholds/proxmox')).toBe('thresholds');
    expect(tabFromPath('/alerts/custom-rules')).toBe('thresholds');
    expect(tabFromPath('/foo/bar')).toBe('overview');
  });

  it('allows custom segments map', () => {
    const custom = { ...ALERT_TAB_SEGMENTS, overview: 'summary' as const };
    expect(pathForTab('overview', custom)).toBe('/alerts/summary');
    expect(tabFromPath('/alerts/summary', custom)).toBe('overview');
  });

  it('keeps alerts configuration owned by a feature surface instead of the page shell', () => {
    expect(alertsPageSource).toContain(
      "import { AlertsConfigurationSurface } from '@/features/alerts/AlertsConfigurationSurface';",
    );
    expect(alertsPageSource).toContain(
      "import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';",
    );
    expect(alertsPageSource).toContain(
      "import { HistoryTab } from '@/features/alerts/tabs/HistoryTab';",
    );
    expect(alertsPageSource).toContain('getAlertsTabGroups({ readOnly: readOnlySession() })');
    expect(alertsPageSource).toContain('touch-scroll');
    expect(alertsPageSource).toContain('scrollbar-hide');
    expect(alertsPageSource).not.toContain('style="-webkit-overflow-scrolling: touch;"');
    expect(alertsPageSource).not.toContain('const loadAlertConfiguration = async');
    expect(alertsPageSource).not.toContain('const FACTORY_GUEST_DEFAULTS =');
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { DestinationsTab } from './tabs/DestinationsTab';",
    );
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { ScheduleTab } from './tabs/ScheduleTab';",
    );
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { ThresholdsTab } from './tabs/ThresholdsTab';",
    );
    expect(alertsConfigurationSurfaceSource).toContain('useAlertsConfigurationState');
    expect(alertsConfigurationSurfaceSource).not.toContain('AlertsAPI.getConfig');
    expect(alertsConfigurationSurfaceSource).not.toContain('NotificationsAPI.getEmailConfig');
    expect(alertsConfigurationSurfaceSource).not.toContain('NotificationsAPI.updateEmailConfig');
    expect(alertsConfigurationSurfaceSource).not.toContain("eventBus.on('org_switched'");
    expect(alertsConfigurationStateSource).toContain('export function useAlertsConfigurationState');
    expect(alertsConfigurationStateSource).toContain('AlertsAPI.getConfig');
    expect(alertsConfigurationStateSource).toContain('createDefaultAlertsConfigurationSnapshot');
    expect(alertsConfigurationStateSource).toContain('readAlertsConfigurationSnapshot');
    expect(alertsConfigurationStateSource).toContain('buildAlertsConfigurationPayload');
    expect(alertsConfigurationStateSource).toContain('useAlertDestinationsState');
    expect(alertsConfigurationStateSource).toContain('useAlertsConfigurationSnapshotState');
    expect(alertsConfigurationStateSource).toContain('useAlertOverridesState');
    expect(alertsConfigurationStateSource).not.toContain('NotificationsAPI.getEmailConfig');
    expect(alertsConfigurationStateSource).not.toContain('NotificationsAPI.updateEmailConfig');
    expect(alertsConfigurationStateSource).toContain("eventBus.on('org_switched'");
    expect(alertsConfigurationStateSource).not.toContain('const createHysteresisThreshold =');
    expect(alertsConfigurationStateSource).not.toContain('const normalizeGap =');
    expect(alertsConfigurationStateSource).not.toContain(
      'const [rawOverridesConfig, setRawOverridesConfig] = createSignal',
    );
    expect(alertsConfigurationStateSource).not.toContain(
      'const [scheduleQuietHours, setScheduleQuietHours] = createSignal',
    );
    expect(alertsConfigurationStateSource).not.toContain('const hostOverrideIdCandidates =');
    expect(alertsConfigurationStateSource).not.toContain('const allGuests = createMemo');
    expect(alertsConfigurationSnapshotStateSource).toContain(
      'export function useAlertsConfigurationSnapshotState',
    );
    expect(alertsConfigurationSnapshotStateSource).toContain(
      'const [scheduleQuietHours, setScheduleQuietHours] = createSignal',
    );
    expect(alertsConfigurationSnapshotStateSource).toContain('applyConfigurationSnapshot');
    expect(alertsConfigurationSnapshotStateSource).toContain('captureConfigurationSnapshot');
    expect(alertsConfigurationSnapshotStateSource).toContain('resetGuestDefaults');
    expect(alertsConfigurationSnapshotStateSource).toContain('FACTORY_GUEST_DEFAULTS');
    expect(alertsConfigurationModelSource).toContain(
      'export function createDefaultAlertsConfigurationSnapshot',
    );
    expect(alertsConfigurationModelSource).toContain(
      'export function readAlertsConfigurationSnapshot',
    );
    expect(alertsConfigurationModelSource).toContain(
      'export function buildAlertsConfigurationPayload',
    );
    expect(alertsConfigurationModelSource).toContain('const createHysteresisThreshold =');
    expect(alertsConfigurationModelSource).toContain('const normalizeGap =');
    expect(alertOverridesStateSource).toContain('export function useAlertOverridesState');
    expect(alertOverridesStateSource).toContain('pbsInstanceFromResource');
    expect(alertOverridesStateSource).toContain('buildProjectedOverrides');
    expect(alertOverridesStateSource).not.toContain('getActionableAgentIdFromResource');
    expect(alertOverridesStateSource).not.toContain('const hostOverrideIdCandidates =');
    expect(alertOverridesStateSource).toContain('props.setOverviewOverrides(overrides())');
    expect(alertOverridesModelSource).toContain('export const normalizeRawOverridesConfig =');
    expect(alertOverridesModelSource).toContain('export const hostOverrideIdCandidates =');
    expect(alertOverridesModelSource).toContain('export const dockerHostOverrideIdCandidates =');
    expect(alertOverridesModelSource).toContain('export const buildProjectedOverrides =');
    expect(alertOverridesModelSource).toContain('getActionableAgentIdFromResource');
    expect(alertDestinationsStateSource).toContain('export function useAlertDestinationsState');
    expect(alertDestinationsStateSource).toContain('NotificationsAPI.getEmailConfig');
    expect(alertDestinationsStateSource).toContain('NotificationsAPI.updateEmailConfig');
    expect(alertDestinationsStateSource).toContain('buildEmailConfigPayload');
    expect(alertDestinationsStateSource).toContain('buildAppriseConfigPayload');
    expect(alertDestinationsStateSource).toContain('normalizeAppriseConfig');
    expect(alertDestinationsStateSource).not.toContain('formatAppriseTargets');
    expect(alertDestinationsStateSource).not.toContain('parseAppriseTargets');
    expect(alertDestinationsModelSource).toContain('export function normalizeAppriseConfig');
    expect(alertDestinationsModelSource).toContain('export function buildEmailConfigPayload');
    expect(alertDestinationsModelSource).toContain('export function buildAppriseConfigPayload');
    expect(alertDestinationsModelSource).toContain('formatAppriseTargets');
    expect(alertDestinationsModelSource).toContain('parseAppriseTargets');
    expect(alertDestinationsTabStateSource).toContain('export function useAlertDestinationsTabState');
    expect(alertDestinationsTabStateSource).toContain('NotificationsAPI.testNotification');
    expect(alertDestinationsTabStateSource).toContain('useAlertWebhookDestinationsState');
    expect(alertDestinationsTabStateSource).not.toContain('NotificationsAPI.getWebhooks');
    expect(alertDestinationsTabStateSource).not.toContain('NotificationsAPI.createWebhook');
    expect(alertWebhookDestinationsStateSource).toContain(
      'export function useAlertWebhookDestinationsState',
    );
    expect(alertWebhookDestinationsStateSource).toContain('NotificationsAPI.getWebhooks');
    expect(alertWebhookDestinationsStateSource).toContain('NotificationsAPI.createWebhook');
    expect(alertWebhookDestinationsStateSource).toContain('NotificationsAPI.testNotification');
    expect(alertsPageSource).toContain(
      "import { HistoryTab } from '@/features/alerts/tabs/HistoryTab';",
    );
    expect(alertsPageSource).not.toContain('function HistoryTab(');
    expect(alertHistoryTabSource).toContain('useAlertHistoryState');
    expect(alertHistoryTabSource).toContain('AlertHistoryFrequencyCard');
    expect(alertHistoryTabSource).toContain('AlertHistoryFiltersCard');
    expect(alertHistoryTabSource).toContain('AlertResourceIncidentsPanel');
    expect(alertHistoryTabSource).toContain('AlertHistoryTableSection');
    expect(alertHistoryTabSource).toContain('AlertHistoryAdministrationCard');
    expect(alertHistoryTabSource).toContain('historyState.alertData().length');
    expect(alertHistoryTabSource).not.toContain('historyState.filteredAlerts().length');
    expect(alertHistoryTabSource).toContain(
      '<AlertResourceIncidentsPanel state={historyState} getResource={props.getResource} />',
    );
    expect(alertDestinationsTabSource).toContain('useAlertDestinationsTabState');
    expect(alertDestinationsTabSource).toContain('AlertDestinationsLoadingState');
    expect(alertDestinationsTabSource).toContain('AlertDestinationsLoadErrorCard');
    expect(alertDestinationsTabSource).toContain('AlertEmailDestinationsSection');
    expect(alertDestinationsTabSource).toContain('AlertAppriseDestinationsSection');
    expect(alertDestinationsTabSource).toContain('AlertWebhookDestinationsSection');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.getWebhooks');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.testNotification');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.createWebhook');
    expect(alertDestinationsTabSource).not.toContain('ALERT_DESTINATIONS_EMAIL_PANEL_TITLE');
    expect(alertDestinationsTabSource).not.toContain('ALERT_DESTINATIONS_APPRISE_PANEL_TITLE');
    expect(alertDestinationsTabSource).not.toContain('getAlertWebhooksSectionTitle');
    expect(alertHistoryTabSource).not.toContain('useAlertIncidentTimelineState');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getHistory');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getIncidentsForResource');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.clearHistory');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getIncidentTimeline');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.addIncidentNote');
    expect(alertHistoryTabSource).not.toContain('const loadIncidentTimeline = async');
    expect(alertHistoryTabSource).not.toContain('const saveIncidentNote = async');
    expect(alertHistoryTabSource).not.toContain('usePersistentSignal(');
    expect(alertHistoryTabSource).not.toContain("const [searchTerm, setSearchTerm] = createSignal");
    expect(alertHistoryFrequencyCardSource).toContain('export function AlertHistoryFrequencyCard');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertFrequencySelectionPresentation');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertBucketCountLabel');
    expect(alertHistoryFiltersCardSource).toContain('export function AlertHistoryFiltersCard');
    expect(alertHistoryFiltersCardSource).toContain('getAlertHistorySearchPlaceholder');
    expect(alertResourceIncidentsPanelSource).toContain('export function AlertResourceIncidentsPanel');
    expect(alertResourceIncidentsPanelSource).toContain('IncidentEventFilters');
    expect(alertResourceIncidentsPanelSource).toContain('IncidentTimelineEventCard');
    expect(alertResourceIncidentsPanelSource).toContain('buildResolvedResourceSurfaceLinks');
    expect(alertResourceIncidentsPanelSource).toContain('allowInfrastructureFallback: true');
    expect(alertResourceIncidentsPanelSource).not.toContain('buildInfrastructureResourceLink');
    expect(alertResourceIncidentsPanelSource).not.toContain(
      'buildResourceSurfaceLinksForResource',
    );
    expect(alertResourceIncidentsPanelSource).toContain('{link.compactLabel}');
    expect(alertHistoryTableSectionSource).toContain('export function AlertHistoryTableSection');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableGroupRow');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableAlertRow');
    expect(alertHistoryTableSectionSource).not.toContain('IncidentTimelinePanel');
    expect(alertHistoryTableSectionSource).not.toContain('InvestigateAlertButton');
    expect(alertHistoryTableGroupRowSource).toContain('export function AlertHistoryTableGroupRow');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupSummaryLabel');
    expect(alertHistoryTableAlertRowSource).toContain('export function AlertHistoryTableAlertRow');
    expect(alertHistoryTableAlertRowSource).toContain('IncidentTimelinePanel');
    expect(alertHistoryTableAlertRowSource).toContain('InvestigateAlertButton');
    expect(alertHistoryAdministrationCardSource).toContain(
      'export function AlertHistoryAdministrationCard',
    );
    expect(alertHistoryAdministrationCardSource).toContain(
      'getAlertAdministrationClearHistoryLabel',
    );
    expect(alertHistoryStateSource).toContain('export function useAlertHistoryState');
    expect(alertHistoryStateSource).toContain('export type AlertHistoryState');
    expect(alertHistoryStateSource).toContain('AlertsAPI.getHistory');
    expect(alertHistoryStateSource).toContain('AlertsAPI.clearHistory');
    expect(alertHistoryStateSource).toContain('useAlertResourceIncidentsState');
    expect(alertHistoryStateSource).toContain('useAlertIncidentTimelineState');
    expect(alertHistoryStateSource).not.toContain('AlertsAPI.getIncidentsForResource');
    expect(alertHistoryStateSource).toContain('buildAlertHistoryItems');
    expect(alertHistoryStateSource).toContain('buildAlertTrends');
    expect(alertHistoryStateSource).toContain('groupAlertHistoryItems');
    expect(alertHistoryStateSource).not.toContain('const formatDuration =');
    expect(alertHistoryStateSource).not.toContain('const formatBucketRange =');
    expect(alertHistoryStateSource).not.toContain('const formatAxisTickLabel =');
    expect(alertHistoryStateSource).not.toContain('const monthNames = [');
    expect(alertHistoryModelSource).toContain('export function buildAlertHistoryItems');
    expect(alertHistoryModelSource).toContain('export function buildAlertTrends');
    expect(alertHistoryModelSource).toContain('export function groupAlertHistoryItems');
    expect(alertHistoryModelSource).toContain('export const MS_PER_HOUR');
    expect(alertResourceIncidentsStateSource).toContain(
      'export function useAlertResourceIncidentsState',
    );
    expect(alertResourceIncidentsStateSource).toContain('AlertsAPI.getIncidentsForResource');
    expect(alertIncidentTimelineStateSource).toContain(
      'export function useAlertIncidentTimelineState',
    );
    expect(alertIncidentTimelineStateSource).toContain('AlertsAPI.getIncidentTimeline');
    expect(alertIncidentTimelineStateSource).toContain('AlertsAPI.addIncidentNote');
    expect(alertOverviewTabSource).toContain('AlertOverviewStatsCards');
    expect(alertOverviewTabSource).toContain('AlertOverviewActiveAlertsSection');
    expect(alertOverviewTabSource).toContain('useAlertOverviewState');
    expect(alertOverviewTabSource).toContain('useAlertIncidentTimelineState');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.unacknowledge');
    expect(alertOverviewStateSource).toContain('export function useAlertOverviewState');
    expect(alertOverviewStateSource).toContain('export type AlertOverviewState');
    expect(alertOverviewStateSource).toContain('useAlertAcknowledgementState');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.unacknowledge');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().acknowledged');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().total24h');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().overrides');
    expect(alertOverviewActiveAlertsSectionSource).toContain('AlertOverviewAlertCard');
    expect(alertOverviewActiveAlertsSectionSource).toContain('getAlertListEmptyState');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewCardPresentation');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewAcknowledgedBadgeClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewStartedAtClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewPrimaryActionClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewSecondaryActionClass');
    expect(alertAcknowledgementStateSource).toContain('export function useAlertAcknowledgementState');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.bulkAcknowledge');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.acknowledge');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.unacknowledge');
    expect(recentAlertsPanelSource).toContain('useAlertAcknowledgementState');
    expect(recentAlertsPanelSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(recentAlertsPanelSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertScheduleTabSource).toContain('useAlertScheduleState');
    expect(alertScheduleTabSource).not.toContain('createDefaultQuietHours');
    expect(alertScheduleTabSource).not.toContain('createDefaultCooldown');
    expect(alertScheduleTabSource).not.toContain('createDefaultGrouping');
    expect(alertScheduleTabSource).not.toContain('createDefaultEscalation');
    expect(alertScheduleTabSource).not.toContain('const timezones = [');
    expect(alertScheduleTabSource).not.toContain('const days = [');
    expect(alertScheduleStateSource).toContain('export function useAlertScheduleState');
    expect(alertScheduleStateSource).toContain('createDefaultQuietHours');
    expect(alertScheduleStateSource).toContain('createDefaultCooldown');
    expect(alertScheduleStateSource).toContain('createDefaultGrouping');
    expect(alertScheduleStateSource).toContain('createDefaultEscalation');
    expect(alertScheduleTabSource).toContain('getAlertConfigQuietHourSuppressOptions');
    expect(alertScheduleTabSource).toContain('AlertQuietHoursSection');
    expect(alertScheduleTabSource).toContain('AlertCooldownSection');
    expect(alertScheduleTabSource).toContain('AlertGroupingSection');
    expect(alertScheduleTabSource).toContain('AlertRecoverySection');
    expect(alertScheduleTabSource).toContain('AlertEscalationSection');
    expect(alertScheduleTabSource).toContain('AlertScheduleSummarySection');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_COOLDOWN_TITLE');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_QUIET_HOURS_TITLE');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_ESCALATION_TITLE');
    expect(alertThresholdsTabSource).toContain('ThresholdsTable');
    expect(alertThresholdsTabSource).toContain('pmgThresholds={props.pmgThresholds}');
    expect(alertThresholdsTabSource).toContain('guestDefaults={props.guestDefaults()}');
    expect(alertThresholdsTabSource).not.toContain('buildThresholdsTableProps');
    expect(thresholdsTabModelSource).toContain('export interface ThresholdsTabProps');
    expect(thresholdsTabModelSource).toContain('extends Omit<');
    expect(thresholdsTabModelSource).toContain(
      "guestDefaults: Accessor<ThresholdsTableProps['guestDefaults']>;",
    );
    expect(thresholdsTabModelSource).not.toContain('hasUnsavedChanges');
    expect(thresholdsTableSource).toContain(
      "import { useThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';",
    );
    expect(thresholdsTableSource).toContain("import { ThresholdsTableProxmoxTab } from './ThresholdsTableProxmoxTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTablePMGTab } from './ThresholdsTablePMGTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTableAgentsTab } from './ThresholdsTableAgentsTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTableDockerTab } from './ThresholdsTableDockerTab';");
    expect(thresholdsTableSource).not.toContain('const [searchTerm, setSearchTerm] = createSignal');
    expect(thresholdsTableSource).not.toContain('const handleTabClick =');
    expect(thresholdsTableSource).not.toContain("groupedResources={state.guestsGroupedByNode()}");
    expect(thresholdsTableSource).not.toContain('dockerIgnoredPrefixesPresentation.title');
    expect(thresholdsTableProxmoxTabSource).toContain('export function ThresholdsTableProxmoxTab');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxNodesSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxPBSSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxGuestsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxGuestFilteringSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxBackupsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxSnapshotsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxStorageSection');
    expect(thresholdsTableProxmoxTabSource).not.toContain('backupOrphanedPresentation');
    expect(thresholdsTableProxmoxTabSource).not.toContain("sectionTitles.guestFiltering");
    expect(thresholdsTableSectionPropsSource).toContain('export interface ThresholdsTableSectionProps');
    expect(thresholdsTableProxmoxNodesSectionSource).toContain('export function ThresholdsTableProxmoxNodesSection');
    expect(thresholdsTableProxmoxPBSSectionSource).toContain('export function ThresholdsTableProxmoxPBSSection');
    expect(thresholdsTableProxmoxGuestsSectionSource).toContain('export function ThresholdsTableProxmoxGuestsSection');
    expect(thresholdsTableProxmoxGuestFilteringSectionSource).toContain(
      'export function ThresholdsTableProxmoxGuestFilteringSection',
    );
    expect(thresholdsTableProxmoxBackupsSectionSource).toContain(
      'export function ThresholdsTableProxmoxBackupsSection',
    );
    expect(thresholdsTableProxmoxBackupsSectionSource).toContain('backupOrphanedPresentation');
    expect(thresholdsTableProxmoxSnapshotsSectionSource).toContain(
      'export function ThresholdsTableProxmoxSnapshotsSection',
    );
    expect(thresholdsTableProxmoxStorageSectionSource).toContain(
      'export function ThresholdsTableProxmoxStorageSection',
    );
    expect(thresholdsTablePMGTabSource).toContain('export function ThresholdsTablePMGTab');
    expect(thresholdsTablePMGTabSource).toContain('pmgGlobalDefaults()');
    expect(thresholdsTableAgentsTabSource).toContain('export function ThresholdsTableAgentsTab');
    expect(thresholdsTableAgentsTabSource).toContain('ThresholdsTableAgentsResourcesSection');
    expect(thresholdsTableAgentsTabSource).toContain('ThresholdsTableAgentDisksSection');
    expect(thresholdsTableAgentsTabSource).not.toContain('agentDisksGroupedByAgent()');
    expect(thresholdsTableAgentsResourcesSectionSource).toContain(
      'export function ThresholdsTableAgentsResourcesSection',
    );
    expect(thresholdsTableAgentDisksSectionSource).toContain(
      'export function ThresholdsTableAgentDisksSection',
    );
    expect(thresholdsTableAgentDisksSectionSource).toContain('agentDisksGroupedByAgent()');
    expect(thresholdsTableDockerTabSource).toContain('export function ThresholdsTableDockerTab');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerIgnoredPrefixesSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerServiceGapSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerHostsSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerContainersSection');
    expect(thresholdsTableDockerTabSource).not.toContain('dockerIgnoredPrefixesPresentation.title');
    expect(thresholdsTableDockerTabSource).not.toContain('serviceGapValidationMessage()');
    expect(thresholdsTableDockerIgnoredPrefixesSectionSource).toContain(
      'export function ThresholdsTableDockerIgnoredPrefixesSection',
    );
    expect(thresholdsTableDockerIgnoredPrefixesSectionSource).toContain(
      'dockerIgnoredPrefixesPresentation.title',
    );
    expect(thresholdsTableDockerServiceGapSectionSource).toContain(
      'export function ThresholdsTableDockerServiceGapSection',
    );
    expect(thresholdsTableDockerServiceGapSectionSource).toContain('serviceGapValidationMessage()');
    expect(thresholdsTableDockerHostsSectionSource).toContain(
      'export function ThresholdsTableDockerHostsSection',
    );
    expect(thresholdsTableDockerContainersSectionSource).toContain(
      'export function ThresholdsTableDockerContainersSection',
    );
    expect(thresholdsDataHookSource).toContain('export function useThresholdsData');
    expect(thresholdsDataHookSource).toContain('useThresholdsHostData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsDockerData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsGuestData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsInfrastructureData(inputs)');
    expect(thresholdsDataHookSource).not.toContain('const hostOverrideIdCandidates =');
    expect(thresholdsDataHookSource).not.toContain('const dockerContainersGroupedByHost = createMemo');
    expect(thresholdsHostDataHookSource).toContain('export function useThresholdsHostData');
    expect(thresholdsHostDataHookSource).toContain('hostOverrideIdCandidates(agentResource)');
    expect(thresholdsDockerDataHookSource).toContain('export function useThresholdsDockerData');
    expect(thresholdsDockerDataHookSource).toContain('dockerContainerOverrideIdCandidates');
    expect(thresholdsGuestDataHookSource).toContain('export function useThresholdsGuestData');
    expect(thresholdsInfrastructureDataHookSource).toContain(
      'export function useThresholdsInfrastructureData',
    );
    expect(thresholdsResourceModelSource).toContain('export function hostOverrideIdCandidates');
    expect(thresholdsResourceModelSource).toContain('export function buildNodeHeaderMeta');
    expect(thresholdsResourceModelSource).toContain('export const normalizeStorageStatus');
    expect(thresholdsTableStateHookSource).toContain('export function useThresholdsTableState');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsData(props, editingId, searchTerm)');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsRecoveryDefaultsState(props)');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsOverrideMutations');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsAvailabilityMutations');
    expect(thresholdsTableStateHookSource).not.toContain('const saveEdit = (resourceId: string) => {');
    expect(thresholdsTableStateHookSource).not.toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsDataHookSource).not.toContain('const sanitizeSnapshotConfig =');
    expect(thresholdsDataHookSource).not.toContain('const sanitizeBackupConfig =');
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain(
      'export function useThresholdsRecoveryDefaultsState',
    );
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain('const sanitizeSnapshotConfig =');
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain('const sanitizeBackupConfig =');
    expect(thresholdsOverrideMutationsHookSource).toContain(
      'export function useThresholdsOverrideMutations',
    );
    expect(thresholdsOverrideMutationsHookSource).toContain('const saveEdit = (resourceId: string) => {');
    expect(thresholdsOverrideMutationsHookSource).toContain(
      'const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {',
    );
    expect(thresholdsOverrideMutationsHookSource).not.toContain('matchesAlertIdentifier');
    expect(thresholdsOverrideMutationsHookSource).not.toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'export function useThresholdsAvailabilityMutations',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain('matchesAlertIdentifier');
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'const setOfflineState = (resourceId: string, state: OfflineState) => {',
    );
    expect(thresholdsOverrideMutationModelSource).toContain('export const upsertOverride =');
    expect(thresholdsOverrideMutationModelSource).toContain(
      'export const withThresholdEntries =',
    );
    expect(thresholdsOverrideMutationModelSource).toContain('export const stripStateKeys =');
  });

  it('keeps alerts websocket access on the shared app runtime context', () => {
    expect(alertsPageSource).toContain("from '@/contexts/appRuntime'");
    expect(alertsPageSource).not.toContain("from '@/App'");
    expect(alertHistoryTabSource).toContain("from '@/contexts/appRuntime'");
    expect(alertHistoryTabSource).not.toContain("from '@/App'");
  });
});

describe('default schedule helpers', () => {
  it('creates quiet hours defaults', () => {
    const quiet = createDefaultQuietHours();
    const expectedTz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

    expect(quiet).toMatchObject({
      enabled: false,
      start: '22:00',
      end: '08:00',
      suppress: {
        performance: false,
        storage: false,
        offline: false,
      },
    });
    expect(quiet.timezone).toBe(expectedTz);
    expect(quiet.days).toEqual({
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false,
    });
  });

  it('creates cooldown defaults', () => {
    expect(createDefaultCooldown()).toEqual({
      enabled: true,
      minutes: 30,
      maxAlerts: 3,
    });
  });

  it('creates grouping defaults', () => {
    expect(createDefaultGrouping()).toEqual({
      enabled: true,
      window: 1,
      byNode: true,
      byGuest: false,
    });
  });

  it('creates escalation defaults', () => {
    expect(createDefaultEscalation()).toEqual({
      enabled: false,
      levels: [],
    });
  });
});

describe('quiet suppress presentation helpers', () => {
  it('returns the selected quiet suppress card presentation', () => {
    expect(getAlertQuietSuppressCardClass(true)).toBe(
      'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-500',
    );
  });

  it('returns the selected quiet suppress checkbox presentation', () => {
    expect(getAlertQuietSuppressCheckboxClass(true)).toBe(
      'mt-1 flex h-4 w-4 items-center justify-center rounded border-2 border-blue-500 bg-blue-500',
    );
  });
});

describe('incident event filter presentation helpers', () => {
  it('returns the compact filter container presentation', () => {
    expect(getAlertIncidentEventFilterContainerClass('compact')).toBe(
      'flex flex-wrap items-center gap-2 text-[10px] text-muted',
    );
  });

  it('returns the shared action button presentation', () => {
    expect(getAlertIncidentEventFilterActionButtonClass()).toBe(
      'px-2 py-0.5 rounded border border-border text-muted hover:bg-surface-hover',
    );
  });

  it('returns the selected compact chip presentation', () => {
    expect(getAlertIncidentEventFilterChipClass(true, 'compact')).toBe(
      'px-2 py-0.5 rounded border text-[10px] transition-colors border-blue-300 bg-blue-100 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
    );
  });
});

describe('incident timeline presentation helpers', () => {
  it('returns the acknowledged badge presentation', () => {
    expect(getAlertIncidentAcknowledgedBadgeClass()).toBe(
      'px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    );
  });

  it('returns the surface event-card presentation', () => {
    expect(getAlertIncidentTimelineEventCardClass('surface')).toBe(
      'rounded border border-border bg-surface p-2',
    );
  });

  it('returns the note editor presentation', () => {
    expect(getAlertIncidentNoteTextareaClass()).toBe(
      'w-full rounded border border-border bg-surface p-2 text-xs text-base-content',
    );
    expect(getAlertIncidentNoteSaveButtonClass()).toBe(
      'px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed',
    );
  });

  it('returns the shared event detail presentation', () => {
    expect(getAlertIncidentTimelineMetaRowClass()).toBe(
      'flex flex-wrap items-center gap-2 text-xs text-muted',
    );
    expect(getAlertIncidentTimelineHeadingClass()).toBe('font-medium text-base-content');
    expect(getAlertIncidentTimelineDetailClass()).toBe('mt-1 text-xs text-base-content');
    expect(getAlertIncidentTimelineCommandClass()).toBe(
      'mt-1 font-mono text-xs text-base-content',
    );
    expect(getAlertIncidentTimelineOutputClass()).toBe('mt-1 text-xs text-muted');
  });

  it('returns the resource incident panel presentation', () => {
    expect(getAlertResourceIncidentCardClass()).toBe(
      'rounded border border-border bg-surface p-3',
    );
    expect(getAlertResourceIncidentSummaryRowClass()).toBe(
      'mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted',
    );
    expect(getAlertResourceIncidentActivitySummaryClass()).toBe(
      'flex flex-wrap items-center gap-1.5',
    );
    expect(getAlertResourceIncidentActivityChipClass()).toBe(
      'rounded bg-surface-alt px-2 py-0.5 text-[10px] font-medium text-base-content',
    );
    expect(getAlertResourceIncidentToggleButtonClass()).toBe(
      'px-2 py-1 text-[10px] border rounded-md border-border text-muted hover:bg-surface-hover',
    );
    expect(getAlertResourceIncidentTruncatedEventsLabel(6)).toBe('Showing last 6 events');
    expect(getAlertResourceIncidentTruncatedEventsLabel(6, 6)).toBe('Showing 6 events');
    expect(getAlertResourceIncidentTruncatedEventsLabel(6, 9)).toBe('Showing last 6 of 9 events');
  });
});

describe('incident event summaries', () => {
  it('treats a fully selected filter set as all events and an empty set as no events', () => {
    const events = [
      { id: '1', type: 'alert_fired', timestamp: '2026-03-20T10:00:00Z', summary: 'Fired' },
      { id: '2', type: 'note', timestamp: '2026-03-20T10:01:00Z', summary: 'Noted' },
    ];

    expect(
      filterIncidentEvents(
        events,
        new Set([
          'alert_fired',
          'alert_acknowledged',
          'alert_unacknowledged',
          'alert_resolved',
          'ai_analysis',
          'command',
          'runbook',
          'note',
        ]),
      ),
    ).toEqual(events);
    expect(filterIncidentEvents(events, new Set())).toEqual([]);
  });

  it('summarizes incident events in canonical order and retains unknown event types', () => {
    expect(
      summarizeIncidentEvents([
        { id: '1', type: 'note', timestamp: '2026-03-20T10:00:00Z', summary: 'Added note' },
        {
          id: '2',
          type: 'alert_fired',
          timestamp: '2026-03-20T10:01:00Z',
          summary: 'Alert fired',
        },
        { id: '3', type: 'note', timestamp: '2026-03-20T10:02:00Z', summary: 'Added note' },
        {
          id: '4',
          type: 'command',
          timestamp: '2026-03-20T10:03:00Z',
          summary: 'Command executed',
        },
        {
          id: '5',
          type: 'operator_followup',
          timestamp: '2026-03-20T10:04:00Z',
          summary: 'Operator follow-up',
        },
      ]),
    ).toEqual([
      { type: 'alert_fired', label: 'Fired', count: 1 },
      { type: 'command', label: 'Cmd', count: 1 },
      { type: 'note', label: 'Note', count: 2 },
      { type: 'operator_followup', label: 'operator_followup', count: 1 },
    ]);
  });
});

describe('cooldown sanitizers', () => {
  it('clamps cooldown minutes into valid range', () => {
    expect(clampCooldownMinutes(2)).toBe(5);
    expect(clampCooldownMinutes(60)).toBe(60);
    expect(clampCooldownMinutes(999)).toBe(120);
    expect(clampCooldownMinutes(undefined)).toBe(5);
  });

  it('provides sensible fallback when enabling cooldown', () => {
    expect(fallbackCooldownMinutes(0)).toBe(30);
    expect(fallbackCooldownMinutes(undefined)).toBe(30);
    expect(fallbackCooldownMinutes(2)).toBe(5);
  });

  it('clamps max alerts per hour', () => {
    expect(clampMaxAlertsPerHour(0)).toBe(1);
    expect(clampMaxAlertsPerHour(7)).toBe(7);
    expect(clampMaxAlertsPerHour(40)).toBe(10);
    expect(clampMaxAlertsPerHour(undefined)).toBe(1);
  });

  it('falls back to defaults for invalid max alerts values', () => {
    expect(fallbackMaxAlertsPerHour(undefined)).toBe(3);
    expect(fallbackMaxAlertsPerHour(0)).toBe(3);
    expect(fallbackMaxAlertsPerHour(50)).toBe(10);
  });
});

describe('threshold helper utilities', () => {
  it('extracts trigger values and ignores non-threshold keys', () => {
    const result = extractTriggerValues({
      cpu: { trigger: 80, clear: 70 },
      memory: { trigger: 85, clear: 75 },
      disabled: true,
      poweredOffSeverity: 'warning',
      customFlag: true,
      customLegacy: 42,
      label: 'ignored',
    } as RawOverrideConfig);

    expect(result).toEqual({
      cpu: 80,
      memory: 85,
      customFlag: 0,
      customLegacy: 42,
    });
  });

  it('getTriggerValue handles multiple input shapes', () => {
    expect(getTriggerValue(75)).toBe(75);
    expect(getTriggerValue({ trigger: 90, clear: 80 })).toBe(90);
    expect(getTriggerValue(true)).toBe(0);
    expect(getTriggerValue(undefined)).toBe(0);
  });
});

describe('normalizeEmailConfigFromAPI', () => {
  it('preserves explicit zero values and false booleans', () => {
    const result = normalizeEmailConfigFromAPI({
      enabled: true,
      provider: 'custom',
      server: 'smtp.example.com',
      port: 0,
      username: 'user',
      password: 'pass',
      from: 'alerts@example.com',
      to: ['ops@example.com'],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    });

    expect(result).toEqual({
      enabled: true,
      provider: 'custom',
      server: 'smtp.example.com',
      port: 0,
      username: 'user',
      password: 'pass',
      from: 'alerts@example.com',
      to: ['ops@example.com'],
      tls: false,
      startTLS: false,
      replyTo: '',
      maxRetries: 3,
      retryDelay: 5,
      rateLimit: 0,
    });
  });

  it('falls back to defaults for malformed payload types', () => {
    const malformed = {
      enabled: 'yes',
      provider: 123,
      server: ['smtp'],
      port: '587',
      username: null,
      password: {},
      from: true,
      to: ['ops@example.com', 42, null],
      tls: 'true',
      startTLS: {},
      rateLimit: '60',
    } as unknown as Partial<import('@/api/notifications').EmailConfig>;

    const result = normalizeEmailConfigFromAPI(malformed);

    expect(result).toEqual({
      enabled: false,
      provider: '',
      server: '',
      port: 587,
      username: '',
      password: '',
      from: '',
      to: ['ops@example.com'],
      tls: true,
      startTLS: false,
      replyTo: '',
      maxRetries: 3,
      retryDelay: 5,
      rateLimit: 60,
    });
  });
});

describe('unifiedTypeToAlertDisplayType', () => {
  it('maps vm to VM', () => {
    expect(unifiedTypeToAlertDisplayType('vm')).toBe('VM');
  });

  it('maps system-container and oci-container to Container', () => {
    expect(unifiedTypeToAlertDisplayType('system-container')).toBe('Container');
    expect(unifiedTypeToAlertDisplayType('oci-container')).toBe('Container');
  });

  it('maps app-container to Container', () => {
    expect(unifiedTypeToAlertDisplayType('app-container')).toBe('App Container');
  });

  it('maps agent to Agent', () => {
    expect(unifiedTypeToAlertDisplayType('agent')).toBe('Agent');
  });

  it('maps docker-host to Container Runtime', () => {
    expect(unifiedTypeToAlertDisplayType('docker-host')).toBe('Container Runtime');
  });

  it('maps storage and datastore to canonical labels', () => {
    expect(unifiedTypeToAlertDisplayType('storage')).toBe('Storage');
    expect(unifiedTypeToAlertDisplayType('datastore')).toBe('Datastore');
  });

  it('maps pbs to PBS', () => {
    expect(unifiedTypeToAlertDisplayType('pbs')).toBe('PBS');
  });

  it('maps pmg to PMG', () => {
    expect(unifiedTypeToAlertDisplayType('pmg')).toBe('PMG');
  });

  it('maps k8s-cluster to K8s Cluster', () => {
    expect(unifiedTypeToAlertDisplayType('k8s-cluster')).toBe('K8s Cluster');
  });

  it('passes through unknown types', () => {
    expect(unifiedTypeToAlertDisplayType('other-type' as any)).toBe('other-type');
  });
});

describe('Unified selector parity', () => {
  it('maps all unified resource types to display types', () => {
    const cases: Array<[ResourceType, string]> = [
      ['agent', 'Agent'],
      ['docker-host', 'Container Runtime'],
      ['k8s-cluster', 'K8s Cluster'],
      ['k8s-node', 'K8s Node'],
      ['vm', 'VM'],
      ['system-container', 'Container'],
      ['oci-container', 'Container'],
      ['app-container', 'App Container'],
      ['pod', 'Pod'],
      ['jail', 'Jail'],
      ['docker-service', 'Docker Service'],
      ['k8s-deployment', 'K8s Deployment'],
      ['k8s-service', 'K8s Service'],
      ['storage', 'Storage'],
      ['datastore', 'Datastore'],
      ['pool', 'Pool'],
      ['dataset', 'Dataset'],
      ['pbs', 'PBS'],
      ['pmg', 'PMG'],
    ];

    for (const [input, expected] of cases) {
      expect(unifiedTypeToAlertDisplayType(input)).toBe(expected);
    }
  });

  it('maps the legacy truenas alias to Agent', () => {
    expect(unifiedTypeToAlertDisplayType('truenas' as any)).toBe('Agent');
  });

  it('keeps guest override extraction shape aligned with legacy mapping', () => {
    const thresholds: RawOverrideConfig = {
      cpu: { trigger: 88, clear: 78 },
      memory: { trigger: 82, clear: 72 },
      disabled: true,
      disableConnectivity: true,
      poweredOffSeverity: 'critical',
    };

    const buildLegacyGuestOverride = (
      guestType: 'qemu' | 'lxc',
      id: string,
      name: string,
      vmid: number,
      node: string,
      instance: string,
    ) => ({
      id,
      name,
      type: 'guest' as const,
      resourceType: guestType === 'qemu' ? 'VM' : 'Container',
      vmid,
      node,
      instance,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });

    const buildUnifiedGuestOverride = (
      resourceType: 'vm' | 'system-container' | 'oci-container',
      id: string,
      name: string,
      vmid: number,
      node: string,
      instance: string,
    ) => ({
      id,
      name,
      type: 'guest' as const,
      resourceType: unifiedTypeToAlertDisplayType(resourceType),
      vmid,
      node,
      instance,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });

    expect(
      buildUnifiedGuestOverride('vm', 'vm-pve1-100', 'app-100', 100, 'pve1', 'pve1/qemu/100'),
    ).toEqual(
      buildLegacyGuestOverride('qemu', 'vm-pve1-100', 'app-100', 100, 'pve1', 'pve1/qemu/100'),
    );

    expect(
      buildUnifiedGuestOverride(
        'system-container',
        'ct-pve1-200',
        'ct-200',
        200,
        'pve1',
        'pve1/lxc/200',
      ),
    ).toEqual(
      buildLegacyGuestOverride('lxc', 'ct-pve1-200', 'ct-200', 200, 'pve1', 'pve1/lxc/200'),
    );
  });
});
