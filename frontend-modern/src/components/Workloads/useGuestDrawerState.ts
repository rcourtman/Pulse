import { createEffect, createMemo, createSignal } from 'solid-js';

import { useAlertsActivation } from '@/stores/alertsActivation';
import type { HistoryTimeRange } from '@/api/charts';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { DisplayMetricType } from '@/utils/metricThresholds';
import {
  getCanonicalWorkloadId,
  getWebInterfaceTargetLabelForWorkload,
  getWorkloadMetadataId,
} from '@/utils/workloads';

import {
  getWorkloadAlertPolicyTags,
  getWorkloadAlertResourceIdCandidates,
  getWorkloadAlertThresholdScope,
} from './workloadTopology';
import {
  getGuestDrawerAgentLabel,
  getGuestDrawerAgentHeading,
  getGuestDrawerAgentTitle,
  getGuestDrawerBackupPresentation,
  getGuestDrawerHistoryTarget,
  getGuestDrawerNetworkInterfaces,
  hasGuestDrawerFilesystemDetails,
  hasGuestDrawerOsInfo,
  normalizeGuestDrawerTags,
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  type GuestDrawerViewState,
  type GuestDrawerProps,
  type GuestDrawerTab,
} from './guestDrawerModel';
import {
  getWorkloadActionAgentTitle,
  hasExplicitWorkloadActionAgent,
  shouldShowInGuestAgentInstallCue,
} from './workloadAgentReadiness';

export function createGuestDrawerViewState(): GuestDrawerViewState {
  const [activeTab, setActiveTab] = createSignal<GuestDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  return {
    activeTab,
    setActiveTab,
    historyRange,
    setHistoryRange,
    reset: () => {
      setActiveTab('overview');
      setHistoryRange(GUEST_DRAWER_HISTORY_DEFAULT_RANGE);
    },
  };
}

export function useGuestDrawerState(props: GuestDrawerProps) {
  const alertsActivation = useAlertsActivation();
  const now = useRelativeTimeNow();
  const { activeTab, setActiveTab, historyRange, setHistoryRange } =
    props.viewState ?? createGuestDrawerViewState();

  const guestId = createMemo(() => getCanonicalWorkloadId(props.guest));
  const alertThresholdScope = createMemo(() => getWorkloadAlertThresholdScope(props.guest));
  const alertResourceIdCandidates = createMemo(() =>
    getWorkloadAlertResourceIdCandidates(props.guest),
  );
  const alertPolicyTags = createMemo(() => getWorkloadAlertPolicyTags(props.guest));
  const metricThresholds = (metric: DisplayMetricType) =>
    createMemo(() =>
      alertsActivation.getMetricThresholds(
        alertThresholdScope(),
        metric,
        alertResourceIdCandidates(),
        alertPolicyTags(),
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
  const agentHeading = createMemo(() => getGuestDrawerAgentHeading(props.guest));
  const agentTitle = createMemo(() => getGuestDrawerAgentTitle(props.guest));
  const hasAgentInfo = createMemo(() => agentLabel().length > 0);
  const hasWorkloadActionAgent = createMemo(() => hasExplicitWorkloadActionAgent(props.guest));
  const workloadActionAgentTitle = createMemo(() => getWorkloadActionAgentTitle(props.guest));
  const showInGuestAgentInstallCue = createMemo(() =>
    shouldShowInGuestAgentInstallCue(props.guest, props.parentNodeOnline !== false),
  );
  const ipAddresses = createMemo(() => props.guest.ipAddresses || []);
  const hasFilesystemDetails = createMemo(() => hasGuestDrawerFilesystemDetails(props.guest));
  const networkInterfaces = createMemo(() => getGuestDrawerNetworkInterfaces(props.guest));
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const normalizedTags = createMemo(() => normalizeGuestDrawerTags(props.guest.tags));
  const backupPresentation = createMemo(() =>
    getGuestDrawerBackupPresentation(
      props.guest.lastBackup,
      alertsActivation.getBackupThresholds(),
      now(),
    ),
  );
  const historyTarget = createMemo(() => getGuestDrawerHistoryTarget(props.guest));
  const hasHistorySupport = createMemo(() => historyTarget() !== null);
  const webInterfaceTargetLabel = createMemo(() =>
    getWebInterfaceTargetLabelForWorkload(props.guest),
  );
  const webInterfaceMetadataId = createMemo(
    () => props.metadataId?.trim() || getWorkloadMetadataId(props.guest),
  );

  const switchTab = (tab: GuestDrawerTab) => {
    setActiveTab(tab);
  };

  createEffect(() => {
    if (activeTab() === 'history' && !hasHistorySupport()) {
      setActiveTab('overview');
    }
  });

  return {
    activeTab,
    agentHeading,
    agentLabel,
    agentTitle,
    backupPresentation,
    diskThresholds,
    guestId,
    guestOsSummary,
    hasAgentInfo,
    hasFilesystemDetails,
    hasHistorySupport,
    hasNetworkInterfaces,
    hasOsInfo,
    hasWorkloadActionAgent,
    historyTarget,
    historyRange,
    ipAddresses,
    networkInterfaces,
    normalizedTags,
    osName,
    osVersion,
    showInGuestAgentInstallCue,
    workloadActionAgentTitle,
    switchTab,
    setHistoryRange,
    webInterfaceMetadataId,
    webInterfaceTargetLabel,
  };
}
