import { createEffect, createMemo, createSignal } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import type { DisplayMetricType } from '@/utils/metricThresholds';
import {
  getCanonicalWorkloadId,
  getDiscoveryResourceTypeForWorkload,
  hasDiscoverySupportForWorkload,
  getWebInterfaceTargetLabelForWorkload,
} from '@/utils/workloads';

import {
  getDiscoveryHostIdForWorkload,
  getDiscoveryResourceIdForWorkload,
  getWorkloadAlertResourceIdCandidates,
  getWorkloadAlertThresholdScope,
} from './workloadTopology';
import {
  getGuestDrawerAgentLabel,
  getGuestDrawerAgentTitle,
  getGuestDrawerBackupPresentation,
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  getGuestDrawerHistoryTarget,
  getGuestDrawerMemoryExtraLines,
  getGuestDrawerNetworkInterfaces,
  hasGuestDrawerFilesystemDetails,
  hasGuestDrawerOsInfo,
  normalizeGuestDrawerTags,
  type GuestDrawerProps,
  type GuestDrawerTab,
} from './guestDrawerModel';

export function useGuestDrawerState(props: GuestDrawerProps) {
  const alertsActivation = useAlertsActivation();
  const [activeTab, setActiveTab] = createSignal<GuestDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );

  const guestId = createMemo(() => getCanonicalWorkloadId(props.guest));
  const alertThresholdScope = createMemo(() => getWorkloadAlertThresholdScope(props.guest));
  const alertResourceIdCandidates = createMemo(() =>
    getWorkloadAlertResourceIdCandidates(props.guest),
  );
  const metricThresholds = (metric: DisplayMetricType) =>
    createMemo(() =>
      alertsActivation.getMetricThresholds(
        alertThresholdScope(),
        metric,
        alertResourceIdCandidates(),
      ),
    );
  const diskThresholds = metricThresholds('disk');
  const osName = createMemo(() => props.guest.osName || '');
  const osVersion = createMemo(() => props.guest.osVersion || '');
  const guestOsSummary = createMemo(() => {
    const name = osName().trim();
    const version = osVersion().trim();
    if (name && version) return `${name} • ${version}`;
    return name || version;
  });
  const hasOsInfo = createMemo(() => hasGuestDrawerOsInfo(props.guest));
  const agentLabel = createMemo(() => getGuestDrawerAgentLabel(props.guest));
  const agentTitle = createMemo(() => getGuestDrawerAgentTitle(props.guest));
  const hasAgentInfo = createMemo(() => agentLabel().length > 0);
  const ipAddresses = createMemo(() => props.guest.ipAddresses || []);
  const memoryExtraLines = createMemo(() => getGuestDrawerMemoryExtraLines(props.guest));
  const hasFilesystemDetails = createMemo(() => hasGuestDrawerFilesystemDetails(props.guest));
  const networkInterfaces = createMemo(() => getGuestDrawerNetworkInterfaces(props.guest));
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const normalizedTags = createMemo(() => normalizeGuestDrawerTags(props.guest.tags));
  const backupPresentation = createMemo(() =>
    props.guest.lastBackup ? getGuestDrawerBackupPresentation(props.guest.lastBackup) : null,
  );
  const hasDiscoverySupport = createMemo(() => hasDiscoverySupportForWorkload(props.guest));
  const historyTarget = createMemo(() => getGuestDrawerHistoryTarget(props.guest));
  const hasHistorySupport = createMemo(() => historyTarget() !== null);
  const discoveryAgentId = createMemo(() => getDiscoveryHostIdForWorkload(props.guest));
  const discoveryResourceId = createMemo(() => getDiscoveryResourceIdForWorkload(props.guest));
  const discoveryResourceType = createMemo(() => getDiscoveryResourceTypeForWorkload(props.guest));
  const webInterfaceTargetLabel = createMemo(() =>
    getWebInterfaceTargetLabelForWorkload(props.guest),
  );

  const switchTab = (tab: GuestDrawerTab) => {
    setActiveTab(tab);
  };

  createEffect(() => {
    if (activeTab() === 'discovery' && !hasDiscoverySupport()) {
      setActiveTab('overview');
    }
    if (activeTab() === 'history' && !hasHistorySupport()) {
      setActiveTab('overview');
    }
  });

  return {
    activeTab,
    agentLabel,
    agentTitle,
    backupPresentation,
    discoveryAgentId,
    discoveryLoadingState: getDiscoveryLoadingState(),
    discoveryResourceId,
    discoveryResourceType,
    diskThresholds,
    guestId,
    guestOsSummary,
    hasAgentInfo,
    hasDiscoverySupport,
    hasFilesystemDetails,
    hasHistorySupport,
    hasNetworkInterfaces,
    hasOsInfo,
    historyTarget,
    historyRange,
    ipAddresses,
    memoryExtraLines,
    networkInterfaces,
    normalizedTags,
    osName,
    osVersion,
    switchTab,
    setHistoryRange,
    webInterfaceTargetLabel,
  };
}
